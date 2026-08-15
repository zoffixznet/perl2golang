package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxFindings caps how much one review may propose. A model that returns
// dozens of findings for one file is failing, not reviewing.
const maxFindings = 25

// maxWhyChars caps the free-text field. Long free text inside a grammar is
// where repetition loops live, so it is bounded in the prompt and again here.
const maxWhyChars = 200

// CodeRequest is one file to review: the deterministic Go, and as much
// grounding as the converter can supply.
type CodeRequest struct {
	// Path is the file's path in the module. It is used in messages, and for
	// the compile gate when [Options.ModuleDir] is set.
	Path string
	// Source is the deterministic Go, already formatted.
	Source string
	// PerlSource is the Perl this file was converted from. Optional.
	PerlSource string
	// Skeleton lists the rest of the package as signatures only. It is what
	// stops the model re-implementing a helper that already exists.
	Skeleton string
	// ReviewKinds are the antipattern classes the deterministic checks
	// flagged here, so the prompt asks about what the tool already suspects.
	ReviewKinds []string
	// MaxFindings caps the number of findings accepted. Zero means the
	// package default.
	MaxFindings int
}

// Finding is one proposed change. OldCode is quoted from the file and must
// appear in it verbatim; NewCode replaces it. Imports lists the standard
// library import paths the new code needs, because the fix this job exists
// for, a hand-rolled loop replaced by the standard library call that does the
// same thing, necessarily brings the call's package with it.
type Finding struct {
	Line    int      `json:"line"`
	Kind    string   `json:"kind"`
	OldCode string   `json:"old_code"`
	NewCode string   `json:"new_code"`
	Imports []string `json:"imports"`
	Why     string   `json:"why"`
}

// RejectedFinding is a finding that did not survive a check, with the check
// that turned it down. These belong in the report as suggestions the tool
// declined to make, so the reader can judge for themselves.
type RejectedFinding struct {
	Finding Finding
	Gate    string
	Reason  string
}

// CodeResult is the outcome of reviewing one file.
type CodeResult struct {
	// Source is the file to keep. It is the caller's input unchanged unless
	// at least one finding was applied and every check passed.
	Source string
	// Changed reports whether Source differs from the input.
	Changed bool
	// Applied lists the findings that were accepted, and Rejected those that
	// were not, with the reason.
	Applied  []Finding
	Rejected []RejectedFinding
}

type findingsPayload struct {
	Findings []Finding `json:"findings"`
}

// ImproveCode asks the model to improve one Go file and returns the improved
// source only if it survives every check. Otherwise it returns the input
// unchanged together with a non-nil error, which is a [RejectedError] when the
// model answered but its answer was not usable.
func (c *Client) ImproveCode(ctx context.Context, req CodeRequest) (string, error) {
	res, err := c.ReviewCode(ctx, req)
	if err != nil {
		return req.Source, err
	}
	if !res.Changed && len(res.Rejected) > 0 {
		r := res.Rejected[0]
		return req.Source, &RejectedError{Gate: r.Gate, Reason: r.Reason}
	}
	return res.Source, nil
}

// ReviewCode is [Client.ImproveCode] with the detail kept: which findings were
// applied, which were rejected, and why. Acceptance is per finding, so one bad
// suggestion never costs the good ones.
func (c *Client) ReviewCode(ctx context.Context, req CodeRequest) (CodeResult, error) {
	result := CodeResult{Source: req.Source}
	if !c.opts.Jobs.Has(JobIdiomReview) {
		return result, nil
	}
	if strings.TrimSpace(req.Source) == "" {
		return result, &RejectedError{Gate: "input", Reason: "there is no source to review"}
	}
	if err := VerifyGo(req.Source); err != nil {
		return result, fmt.Errorf("the deterministic file %s does not parse, so there is nothing to improve: %w", req.Path, err)
	}

	c.progressf("ai: reviewing %s", displayPath(req.Path))
	text, _, err := c.generate(ctx, "code review", idiomSystemPrompt, idiomUserPrompt(req), callParams{
		Temperature:   0.1,
		TopP:          0.9,
		TopK:          40,
		RepeatPenalty: 1.05,
		RepeatLastN:   256,
		NumPredict:    2048,
		Format:        rawSchema(idiomSchema),
		Stream:        false,
	})
	if err != nil {
		return result, err
	}

	var payload findingsPayload
	if err := decodeJSON(text, &payload); err != nil {
		return result, err
	}

	limit := req.MaxFindings
	if limit <= 0 {
		limit = maxFindings
	}
	if len(payload.Findings) > limit {
		return result, &RejectedError{
			Gate:   "shape",
			Reason: fmt.Sprintf("%d findings for one file is a failed review, not a review", len(payload.Findings)),
		}
	}

	current := req.Source
	var imported []string
	for _, f := range payload.Findings {
		candidate, used, err := applyFinding(current, req.Source, f, imported)
		if err != nil {
			gate, reason := "unknown", err.Error()
			var re *RejectedError
			if errors.As(err, &re) {
				gate, reason = re.Gate, re.Reason
			}
			result.Rejected = append(result.Rejected, RejectedFinding{Finding: f, Gate: gate, Reason: reason})
			c.recordRejected(Rejection{Job: JobIdiomReview, Target: req.Path, Gate: gate, Reason: reason})
			continue
		}
		current = candidate
		imported = append(imported, used...)
		result.Applied = append(result.Applied, f)
		c.recordAccepted(Change{Job: JobIdiomReview, Target: req.Path, Detail: f.Kind})
	}

	if current == req.Source {
		return result, nil
	}

	// One compile gate for the whole file, when there is a module to check
	// it against. It is the only check that can see a call that parses but
	// does not exist.
	if c.opts.ModuleDir != "" && req.Path != "" {
		if err := c.compileGate(ctx, req.Path, current); err != nil {
			gate, reason := "compile", err.Error()
			var re *RejectedError
			if errors.As(err, &re) {
				gate, reason = re.Gate, re.Reason
			}
			c.rejectRolledBack(req.Path, gate, reason, len(result.Applied))
			result.Rejected = append(result.Rejected, RejectedFinding{Gate: gate, Reason: reason})
			result.Applied = nil
			return CodeResult{Source: req.Source, Rejected: result.Rejected}, nil
		}
	}

	result.Source = current
	result.Changed = true
	return result, nil
}

// compileGate type-checks the candidate against the real module through an
// overlay, leaving the working tree alone.
func (c *Client) compileGate(ctx context.Context, path, candidate string) error {
	dir, err := os.MkdirTemp("", "perl2golang-candidate-")
	if err != nil {
		return nil // not the model's fault; the cheaper checks still stand
	}
	defer os.RemoveAll(dir)

	tmp := filepath.Join(dir, filepath.Base(path))
	if err := os.WriteFile(tmp, []byte(candidate), 0o600); err != nil {
		return nil
	}
	target := path
	if !filepath.IsAbs(target) {
		target = filepath.Join(c.opts.ModuleDir, path)
	}
	return VerifyBuild(ctx, c.opts.ModuleDir, map[string]string{target: tmp})
}

// rollBack turns previously accepted changes for a file back into rejections,
// which is what happens when a whole-file gate fails after per-finding gates
// passed.
func (c *Client) rollBack(target string, n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.summary.Accepted -= n
	c.summary.Rejected += n
	kept := c.summary.Changes[:0]
	for _, ch := range c.summary.Changes {
		if ch.Target != target || ch.Job != JobIdiomReview {
			kept = append(kept, ch)
		}
	}
	c.summary.Changes = kept
}

// rejectRolledBack rolls back a file's accepted findings and records each of
// them as a rejection under the gate that failed the whole file, so the
// summary's counts and its rejection detail stay in step. The findings were
// already counted as proposed when they were accepted, so nothing here touches
// that number.
func (c *Client) rejectRolledBack(target, gate, reason string, n int) {
	c.rollBack(target, n)
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := 0; i < n; i++ {
		c.summary.Rejections = append(c.summary.Rejections,
			Rejection{Job: JobIdiomReview, Target: target, Gate: gate, Reason: reason})
	}
}

// applyFinding checks one finding and splices it in, returning the whole
// candidate file and the imports the finding brought with it. prior lists the
// imports earlier accepted findings already added, since the structural check
// compares against the deterministic baseline, which has none of them. Every
// failure is a [RejectedError] naming the check.
func applyFinding(current, baseline string, f Finding, prior []string) (string, []string, error) {
	switch {
	case !findingKinds[f.Kind]:
		return "", nil, &RejectedError{Gate: "shape", Reason: fmt.Sprintf("%q is not one of the classes the review may report", f.Kind)}
	case strings.TrimSpace(f.OldCode) == "":
		return "", nil, &RejectedError{Gate: "shape", Reason: "the finding quotes no code"}
	case strings.TrimSpace(f.NewCode) == "":
		return "", nil, &RejectedError{Gate: "shape", Reason: "the finding proposes no replacement"}
	case f.OldCode == f.NewCode:
		return "", nil, &RejectedError{Gate: "shape", Reason: "the finding changes nothing"}
	case len(f.Why) > maxWhyChars:
		return "", nil, &RejectedError{Gate: "shape", Reason: "the explanation ran past its one-sentence budget"}
	}

	imports, err := usableImports(f.NewCode, f.Imports)
	if err != nil {
		return "", nil, err
	}

	switch strings.Count(current, f.OldCode) {
	case 1:
	case 0:
		return "", nil, &RejectedError{Gate: "grounding", Reason: "the quoted code is not in the file"}
	default:
		return "", nil, &RejectedError{Gate: "grounding", Reason: "the quoted code appears more than once, so the change is ambiguous"}
	}

	candidate := strings.Replace(current, f.OldCode, f.NewCode, 1)
	candidate, err = addImports(candidate, imports)
	if err != nil {
		return "", nil, err
	}
	formatted, err := gofmt(candidate)
	if err != nil {
		// An unformattable splice is almost always a syntax error, so say
		// which line rather than "gofmt failed".
		if perr := VerifyGo(candidate); perr != nil {
			return "", nil, &RejectedError{Gate: "parse", Reason: perr.Error(), Err: perr}
		}
		return "", nil, err
	}
	if err := VerifyGo(formatted); err != nil {
		return "", nil, &RejectedError{Gate: "parse", Reason: err.Error(), Err: err}
	}
	allowed := append(append([]string{}, prior...), imports...)
	if err := checkIdiomRewrite(baseline, formatted, allowed); err != nil {
		return "", nil, err
	}
	return formatted, imports, nil
}

// decodeJSON turns a model's answer into a payload, tolerating exactly the two
// ways small models mangle JSON that do not invent content: a markdown fence
// around it, and chatter either side of it. Anything beyond that is a rejection
// rather than a repair, because repairing structure means guessing at meaning.
func decodeJSON(text string, out any) error {
	body := stripFences(text)
	body = trimToEnvelope(body)
	if body == "" {
		return &RejectedError{Gate: "shape", Reason: "the answer contained no JSON object"}
	}

	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err == nil {
		return nil
	}

	// One lenient pass, fixing only trailing commas and raw control
	// characters inside strings. Both are formatting noise; neither changes
	// what the model said.
	repaired := repairJSON(body)
	dec = json.NewDecoder(strings.NewReader(repaired))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return &RejectedError{Gate: "shape", Reason: "the answer was not usable JSON", Err: err}
	}
	return nil
}

// stripFences removes a markdown code fence wrapped around the whole answer.
func stripFences(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return t
	}
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[i+1:]
	} else {
		return ""
	}
	if i := strings.LastIndex(t, "```"); i >= 0 {
		t = t[:i]
	}
	return strings.TrimSpace(t)
}

// trimToEnvelope keeps the text from the first brace to the last, which
// removes a preamble or a sign-off without touching the content.
func trimToEnvelope(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}

// repairJSON fixes trailing commas and raw control characters in strings.
func repairJSON(s string) string {
	var out bytes.Buffer
	inString, escaped := false, false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case escaped:
			escaped = false
			out.WriteByte(ch)
		case inString && ch == '\\':
			escaped = true
			out.WriteByte(ch)
		case ch == '"':
			inString = !inString
			out.WriteByte(ch)
		case inString && ch < 0x20:
			switch ch {
			case '\n':
				out.WriteString(`\n`)
			case '\t':
				out.WriteString(`\t`)
			case '\r':
				out.WriteString(`\r`)
			default:
				fmt.Fprintf(&out, `\u%04x`, ch)
			}
		case !inString && ch == ',':
			if next := nextNonSpace(s[i+1:]); next == '}' || next == ']' {
				continue
			}
			out.WriteByte(ch)
		default:
			out.WriteByte(ch)
		}
	}
	return out.String()
}

func nextNonSpace(s string) byte {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return s[i]
		}
	}
	return 0
}

func displayPath(p string) string {
	if p == "" {
		return "the generated file"
	}
	return p
}
