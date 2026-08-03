package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"unicode/utf8"

	"perl2golang/internal/convert"
	"perl2golang/internal/report"
)

// The --json envelope.
//
// One object on standard output, newline terminated, and nothing else on
// standard output ever. It is built completely in memory and written once, so
// a failure part way through a run never produces a truncated object.
//
// Consumers check schema before anything else, and ignore fields they do not
// know: adding a field is a minor change, removing or retyping one bumps the
// schema string.

// resultSchema names the shape of the object writeJSON produces.
const resultSchema = "perl2golang.result/v1"

// jsonResult is the whole object.
type jsonResult struct {
	Schema  string   `json:"schema"`
	Tool    jsonTool `json:"tool"`
	Command []string `json:"command"`
	// Outcome is one of clean, converted-with-notes,
	// converted-with-warnings, converted-with-refusals, failed, usage.
	Outcome  string `json:"outcome"`
	ExitCode int    `json:"exit_code"`
	// Usable is true when something was produced that is worth looking at,
	// which is the question most scripts are really asking.
	Usable      bool             `json:"usable"`
	Conversions []jsonConversion `json:"conversions"`
	Errors      []string         `json:"errors,omitempty"`
}

// jsonTool identifies the binary that produced the object.
type jsonTool struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Go       string `json:"go"`
	Platform string `json:"platform"`
}

// jsonConversion is one input's result.
type jsonConversion struct {
	Source string `json:"source"`
	// OutputDir is empty when nothing was written to disk.
	OutputDir string         `json:"output_dir,omitempty"`
	Artifacts []jsonArtifact `json:"artifacts"`
	Report    *report.Report `json:"report,omitempty"`
	// Summary is the terminal summary this run would have printed, for a
	// consumer that wants to show it without reproducing the layout.
	Summary []string `json:"summary,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// jsonArtifact is one generated file, content included, so a consumer never
// has to go back to the filesystem.
type jsonArtifact struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Role string `json:"role,omitempty"`
	// Bytes and Lines describe Content before any encoding.
	Bytes  int    `json:"bytes"`
	Lines  int    `json:"lines"`
	SHA256 string `json:"sha256"`
	// Encoding is utf8, or base64 for content that is not valid UTF-8.
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// writeJSON builds the object for a finished run and writes it once.
func writeJSON(e *env, f *convertFlags, runs []*run) int {
	res := jsonResult{
		Schema:      resultSchema,
		Tool:        toolInfo(),
		Command:     append([]string{"perl2golang"}, e.argv...),
		Conversions: make([]jsonConversion, 0, len(runs)),
	}

	worst := ExitOK
	for _, r := range runs {
		worst = max(worst, r.exit)
		res.Conversions = append(res.Conversions, conversionJSON(r, f))
		if r.err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", r.in.display, r.err))
		}
	}
	res.ExitCode = worst
	res.Outcome = outcomeOf(runs, worst)
	res.Usable = anyOutput(runs)

	if err := encodeJSON(e, res); err != nil {
		fmt.Fprintf(e.stderr, "perl2golang: writing JSON: %v\n", err)
		return ExitFailed
	}
	return worst
}

// usageJSON answers a command line that could not be understood, so that a
// wrapper passing --json never has to parse two different formats.
func usageJSON(e *env, message string) int {
	res := jsonResult{
		Schema:      resultSchema,
		Tool:        toolInfo(),
		Command:     append([]string{"perl2golang"}, e.argv...),
		Outcome:     "usage",
		ExitCode:    ExitUsage,
		Conversions: []jsonConversion{},
		Errors:      []string{message},
	}
	_ = encodeJSON(e, res)
	return ExitUsage
}

// encodeJSON writes the object. HTML escaping is off, because Perl source and
// generated Go are full of angle brackets and ampersands that a reader should
// see as themselves.
func encodeJSON(e *env, res jsonResult) error {
	enc := json.NewEncoder(e.stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

// conversionJSON describes one input's result.
func conversionJSON(r *run, f *convertFlags) jsonConversion {
	out := jsonConversion{Source: r.in.display, Artifacts: []jsonArtifact{}}
	if r.failed() {
		out.Error = r.err.Error()
		return out
	}
	out.OutputDir = r.dir
	out.Report = r.res.Report
	out.Summary = summaryLines(r, f)

	files := r.res.Bundle()
	for _, name := range artifactOrder(files) {
		out.Artifacts = append(out.Artifacts, artifactJSON(name, files[name]))
	}
	return out
}

// artifactJSON describes one generated file.
func artifactJSON(name string, content []byte) jsonArtifact {
	sum := sha256.Sum256(content)
	a := jsonArtifact{
		Path:     name,
		Kind:     artifactKind(name),
		Role:     artifactRole(name),
		Bytes:    len(content),
		Lines:    countLines(content),
		SHA256:   hex.EncodeToString(sum[:]),
		Encoding: "utf8",
		Content:  string(content),
	}
	// No artifact is binary today, and the field exists so that one could be
	// without breaking every consumer on the day it appears.
	if !utf8.Valid(content) {
		a.Encoding = "base64"
		a.Content = base64.StdEncoding.EncodeToString(content)
	}
	return a
}

// summaryLines is the terminal summary this run would have printed, split into
// lines so a consumer can indent or filter it. A conversion that wrote no
// directory gets the one-line form, because the block form is a tour of files
// that are not there.
func summaryLines(r *run, f *convertFlags) []string {
	if r.dir == "" {
		return []string{streamLine(r, f.verbose)}
	}
	return strings.Split(strings.TrimSuffix(fileBlock(r, f.verbose), "\n"), "\n")
}

// outcomeOf names what happened, in one word a script can switch on.
func outcomeOf(runs []*run, exit int) string {
	if exit == ExitFailed || !anyOutput(runs) {
		return "failed"
	}
	worst := ""
	for _, r := range runs {
		if r.failed() {
			continue
		}
		switch {
		case r.res.Report.Stats.Refused > 0:
			return "converted-with-refusals"
		case r.res.Report.Stats.Approximated > 0:
			worst = "converted-with-warnings"
		case len(r.res.Report.Entries) > 0 && worst == "":
			worst = "converted-with-notes"
		}
	}
	if worst == "" {
		return "clean"
	}
	return worst
}

// anyOutput reports whether any input produced something.
func anyOutput(runs []*run) bool {
	for _, r := range runs {
		if !r.failed() {
			return true
		}
	}
	return false
}

// toolInfo identifies this binary.
func toolInfo() jsonTool {
	return jsonTool{
		Name:     "perl2golang",
		Version:  convert.Version,
		Go:       runtime.Version(),
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
}
