// Package convert runs one Perl file through the whole pipeline and returns
// everything that came out of it: two Go programs, the honest report, and the
// teaching bundle.
//
// It is the only place that knows the order of the phases, which keeps the CLI
// thin and lets the tests drive a conversion without touching the filesystem.
package convert

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"perl2go/internal/gogen"
	"perl2go/internal/ir"
	"perl2go/internal/lower"
	"perl2go/internal/perl/parser"
	"perl2go/internal/report"
	"perl2go/internal/runtime"
	"perl2go/internal/teach"
)

// Version is the tool's version, stamped into generated documents.
const Version = "0.1.0"

// Options configure one conversion.
type Options struct {
	// Path names the input for diagnostics. Use "-e" for a snippet and "-"
	// for standard input.
	Path string
	// Name overrides the generated program's name, which otherwise comes
	// from the input's base name.
	Name string
	// Module overrides the generated module path.
	Module string
	// Verify asks for a full compile of the generated code when a Go
	// toolchain is available. Parsing is always checked, whatever this says.
	Verify bool
	// SkipBuild suppresses the compile even when Verify is set, for callers
	// that are going to build the output themselves anyway.
	SkipBuild bool
	// NoDocs skips the teaching bundle, which the snippet mode uses when it
	// only wants the code and the notes.
	NoDocs bool
	// Improve is an optional pass over the generated artefacts. Its output is
	// checked before it is accepted, so it can improve the result or leave it
	// alone but never corrupt it. Nil means the deterministic output is the
	// output, which is the default and the only behaviour v0.1 ships.
	Improve Improver
}

// Result is one finished conversion.
type Result struct {
	// Path is the input path as given.
	Path string
	// Name is the generated program's name, for example "logwatch".
	Name string
	// Module is the generated module path.
	Module string
	// PerlSource is the original input.
	PerlSource []byte

	// Clean and Annotated hold the two programs, keyed by file name.
	Clean     map[string][]byte
	Annotated map[string][]byte

	// Docs holds the teaching bundle, keyed by path relative to the output
	// directory.
	Docs map[string]string

	// Report is the honest account of what happened.
	Report *report.Report
	// Walkthrough is the per-region tour used by the walkthrough document.
	Walkthrough []teach.Segment
	// Diags are the front end's parse diagnostics.
	Diags []parser.Diag
	// Helpers names the runtime support functions the output uses.
	Helpers []string
}

// annotatedDir is the subdirectory the annotated program lives in. It is a
// second main package inside the same module, so both build from one go.mod.
const annotatedDir = "annotated"

// Convert runs the pipeline over one source buffer.
func Convert(src []byte, opts Options) (result *Result, err error) {
	// A panic anywhere in the pipeline is a defect in this tool, not a
	// problem with the user's file. It is turned into an error the CLI can
	// report cleanly, and the test suite treats any occurrence as a bug.
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf("internal error while converting %s: %v\n"+
				"this is a bug in perl2go; please report it with the input that caused it", opts.Path, r)
		}
	}()

	name := opts.Name
	if name == "" {
		name = programName(opts.Path)
	}
	module := opts.Module
	if module == "" {
		module = name
	}

	parsed := parser.Parse(src)
	low := lower.Lower(parsed, src, lower.Options{
		File:    displayPath(opts.Path),
		Program: name,
		Module:  module,
	})

	res := &Result{
		Path:       opts.Path,
		Name:       name,
		Module:     module,
		PerlSource: src,
		Report:     low.Report,
		Diags:      parsed.Diags,
		Helpers:    low.Helpers,
		Clean:      map[string][]byte{},
		Annotated:  map[string][]byte{},
	}
	res.Report.Version = Version
	res.Report.Source = displayPath(opts.Path)
	res.Report.Module = module

	for _, f := range low.Program.Files {
		clean, cerr := render(gogen.Clean, f)
		if cerr != nil {
			return nil, cerr
		}
		res.Clean[f.Name] = clean

		annotated, aerr := render(gogen.Annotated, f)
		if aerr != nil {
			return nil, aerr
		}
		res.Annotated[f.Name] = annotated
	}

	if len(low.Helpers) > 0 {
		clean, herr := runtime.Emit(low.Helpers, "main")
		if herr != nil {
			return nil, fmt.Errorf("emitting support code: %w", herr)
		}
		annotated, herr := runtime.EmitAnnotated(low.Helpers, "main")
		if herr != nil {
			return nil, fmt.Errorf("emitting support code: %w", herr)
		}
		res.Clean["helpers.go"] = clean
		res.Annotated["helpers.go"] = annotated
	}

	// The optional improvement pass runs over the code before it is checked,
	// so verification covers whatever it produced as well. The documents are
	// written afterwards, because they report what the checks found.
	res.improveCode(context.Background(), opts.Improve)
	res.verify(!opts.SkipBuild)
	res.Walkthrough = walkthrough(low, src)

	if !opts.NoDocs {
		docs, derr := teach.Docs(teach.DocInput{
			ScriptName:  path.Base(displayPath(opts.Path)),
			ProgramName: name,
			Module:      module,
			PerlSource:  string(src),
			GoSource:    string(res.Clean["main.go"]),
			Report:      res.Report,
			Concepts:    res.Report.Concepts,
			Walkthrough: res.Walkthrough,
			Version:     Version,
		})
		if derr != nil {
			return nil, fmt.Errorf("generating documentation: %w", derr)
		}
		res.Docs = docs
	}

	res.improveDocs(context.Background(), opts.Improve)
	res.Report.SortEntries()
	return res, nil
}

// render emits one file in one mode.
func render(mode gogen.Mode, f *ir.File) ([]byte, error) {
	e := gogen.New(mode)
	out, err := e.File(f)
	if err != nil {
		return nil, fmt.Errorf("emitting %s: %w", f.Name, err)
	}
	return out, nil
}

// verify checks the tool's own output, first by parsing it and then, when a
// toolchain is available, by compiling it. Output that does not build is a
// defect in this tool and is reported as one rather than written and forgotten.
func (r *Result) verify(build bool) {
	v := &r.Report.Verified
	v.Toolchain = gogen.HaveToolchain()

	for name, src := range r.Clean {
		if err := gogen.Parse(name, src); err != nil {
			v.Error = err.Error()
			r.Report.Add(report.Entry{
				Code:     "P2G8501",
				Severity: report.Refuse,
				// This is about the tool, so it names the tool.
				Construct: "generated Go",
				Short:     "the generated Go did not parse",
				Message:   "The Go this tool produced is not valid Go: " + err.Error(),
				Advice: "This is a defect in perl2go rather than a problem with the " +
					"input. Please report it along with the Perl that produced it.",
			})
			return
		}
	}
	v.Parsed = true

	if !v.Toolchain || !build {
		return
	}
	files := map[string][]byte{"go.mod": []byte(GoMod(r.Module))}
	for name, src := range r.Clean {
		files[name] = src
	}
	for name, src := range r.Annotated {
		files[path.Join(annotatedDir, name)] = src
	}
	err := gogen.Build(files)
	switch {
	case err == nil:
		v.Built = true
	case errors.Is(err, gogen.ErrNoToolchain):
		v.Toolchain = false
	default:
		v.Error = err.Error()
		r.Report.Add(report.Entry{
			Code:      "P2G8505",
			Severity:  report.Refuse,
			Construct: "generated Go",
			Short:     "the generated program does not compile",
			Message:   "The generated Go was checked with the Go toolchain and did not build: " + firstLines(err.Error(), 8),
			Advice: "This is a defect in perl2go rather than a problem with the " +
				"input. The output is still written so the error can be seen, and the " +
				"failing lines are worth reporting.",
		})
	}
}

// firstLines trims a long compiler transcript down to something a terminal
// summary can show.
func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n], "\n") + "\n..."
}

// Bundle returns the complete output tree, keyed by path relative to the
// output directory.
func (r *Result) Bundle() map[string][]byte {
	out := map[string][]byte{
		"go.mod": []byte(GoMod(r.Module)),
	}
	for name, src := range r.Clean {
		out[name] = src
	}
	for name, src := range r.Annotated {
		out[path.Join(annotatedDir, name)] = src
	}
	for name, text := range r.Docs {
		out[name] = []byte(text)
	}
	return out
}

// GoMod renders the generated module file.
//
// The generated program has no dependencies at all, so the module file says
// nothing but its own name and the language version it needs. Go 1.23 is the
// floor: the emitted code uses generics, the per-iteration loop variable
// semantics of 1.22, and the iterator forms of maps.Keys and slices.Sorted
// that arrived in 1.23.
func GoMod(module string) string {
	return "module " + module + "\n\ngo 1.23\n"
}

// programName turns an input path into an identifier-safe program name.
func programName(p string) string {
	switch p {
	case "", "-":
		return "snippet"
	case "-e":
		return "snippet"
	}
	base := path.Base(p)
	base = strings.TrimSuffix(base, path.Ext(base))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		case r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "program"
	}
	return out
}

// displayPath is what diagnostics call the input.
func displayPath(p string) string {
	switch p {
	case "":
		return "-e"
	case "-":
		return "standard input"
	}
	return p
}
