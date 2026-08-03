package repl

import (
	"regexp"
	"slices"
	"strings"

	"perl2golang/internal/convert"
	"perl2golang/internal/perl/parser"
	"perl2golang/internal/report"
	"perl2golang/internal/teach"
)

// session is the accumulated program the REPL is building.
//
// The REPL holds a program, not a list of independent snippets. Every snippet
// is appended to the Perl typed so far and the whole thing is converted again,
// because what snippet nine does with a variable can change the Go type
// snippet two has to be given. Showing anything else would mean showing Go
// that a file conversion would not produce, and the REPL would be lying in
// exactly the situation where somebody is relying on it. A forty-line session
// re-converts in well under a millisecond, so the honest answer is also the
// cheap one.
type session struct {
	// snippets are the accepted fragments, in the order they were typed.
	snippets []string
	// view is the last presented rendering of the whole session's Go: the
	// lines a reader has already been shown, with the package clause, the
	// imports and the main function's own braces taken out.
	view []string
	// annotated is the same rendering of the commented program, which is what
	// :why reads from.
	annotated []string
	// named records every concept already mentioned, so the third mention of
	// slice aliasing in one session stays quiet.
	named map[string]bool
	// helpers records the support functions the session's Go already pulled in.
	helpers map[string]bool
	// last is the most recent accepted turn, which the meta commands work on.
	last *turn
}

// turn is one accepted snippet and everything the REPL learned from it.
type turn struct {
	// perl is the snippet as typed.
	perl string
	// startLine is the 1-based line the snippet starts on in the session
	// program, which is how a diagnostic is attributed to a snippet.
	startLine int
	// added holds the Go lines this snippet contributed.
	added []string
	// removed counts lines shown earlier that this snippet displaced.
	removed int
	// annotated holds the same lines from the commented program.
	annotated []string
	// concepts are the concept ids this snippet triggered and no earlier
	// snippet had.
	concepts []string
	// helpers names support functions this snippet pulled in.
	helpers []string
	// entries are the diagnostics raised inside this snippet.
	entries []report.Entry
	// result is the whole session's conversion, for :vars and :go full.
	result *convert.Result
}

func newSession() *session {
	return &session{named: map[string]bool{}, helpers: map[string]bool{}}
}

// program returns the session's Perl as one source file.
func (s *session) program() string {
	if len(s.snippets) == 0 {
		return ""
	}
	return strings.Join(s.snippets, "\n") + "\n"
}

// lineCount is how many lines of Perl the session holds.
func (s *session) lineCount() int {
	p := s.program()
	if p == "" {
		return 0
	}
	return strings.Count(p, "\n")
}

// reset forgets everything, which is the only way out of a session that has
// gone somewhere confusing.
func (s *session) reset() {
	s.snippets = nil
	s.view = nil
	s.annotated = nil
	s.named = map[string]bool{}
	s.helpers = map[string]bool{}
	s.last = nil
}

// rejection is a snippet that never entered the session program.
type rejection struct {
	// diags are the parse diagnostics that stopped it, with positions
	// relative to the snippet rather than to the session.
	diags []parser.Diag
	// internal is set when the converter itself failed, which is a defect in
	// this tool rather than a problem with what was typed.
	internal string
}

// submit converts the session with snippet appended and, if that worked,
// keeps it. A snippet that does not parse leaves the session program exactly
// as it was, so one mistyped line cannot poison the rest of the session.
func (s *session) submit(snippet string) (*turn, *rejection) {
	snippet = strings.TrimRight(snippet, "\n")
	startLine := s.lineCount() + 1

	candidate := append(slices.Clone(s.snippets), snippet)
	src := strings.Join(candidate, "\n") + "\n"

	res, err := convert.Convert([]byte(src), convert.Options{
		Path: "<repl>",
		Name: "session",
		// The REPL wants the code and the concepts, not a documentation
		// bundle, and it wants them between keystrokes rather than between
		// compiles.
		NoDocs:    true,
		SkipBuild: true,
	})
	if err != nil {
		return nil, &rejection{internal: err.Error()}
	}
	if bad := within(res.Diags, startLine); len(bad) > 0 {
		return nil, &rejection{diags: shift(bad, startLine)}
	}

	t := &turn{perl: snippet, startLine: startLine, result: res}

	view := presentable(string(res.Clean["main.go"]))
	t.added, t.removed = deltaOf(s.view, view)
	annotated := presentable(string(res.Annotated["main.go"]))
	t.annotated, _ = deltaOf(s.annotated, annotated)

	for _, id := range res.Report.Concepts {
		if !s.named[id] {
			s.named[id] = true
			t.concepts = append(t.concepts, id)
		}
	}
	for _, h := range res.Helpers {
		if !s.helpers[h] {
			s.helpers[h] = true
			t.helpers = append(t.helpers, h)
		}
	}
	for _, e := range res.Report.Entries {
		if e.Line >= startLine {
			t.entries = append(t.entries, e)
		}
	}

	s.snippets = candidate
	s.view = view
	s.annotated = annotated
	s.last = t
	return t, nil
}

// symbols returns the variables in scope with the Go type inference gave each.
func (s *session) symbols() []report.Symbol {
	if s.last == nil {
		return nil
	}
	return s.last.result.Report.Symbols
}

// segments returns the walkthrough regions covering the last snippet, which is
// the converter's own account of why it chose the Go it did.
func (s *session) segments() []teach.Segment {
	if s.last == nil {
		return nil
	}
	var out []teach.Segment
	for _, seg := range s.last.result.Walkthrough {
		if seg.PerlTo >= s.last.startLine {
			out = append(out, seg)
		}
	}
	return out
}

// full returns the session's whole Go program, imports and main included,
// ready to paste into a file.
func (s *session) full(annotated bool) string {
	if s.last == nil {
		return ""
	}
	files := s.last.result.Clean
	if annotated {
		files = s.last.result.Annotated
	}
	helpers, split := files["helpers.go"]
	var b strings.Builder
	if split {
		// Two files, so each one is named. A session that pulled in support
		// code has to be pasted into two files to build, and saying which is
		// which is cheaper than letting somebody find out from the compiler.
		b.WriteString("// main.go\n")
	}
	b.Write(files["main.go"])
	if split {
		b.WriteString("\n// helpers.go\n")
		b.Write(helpers)
	}
	return b.String()
}

// within returns the parse diagnostics that fall inside the snippet starting
// at startLine. Anything before that line was already there and was already
// reported, so it is not this snippet's fault.
func within(diags []parser.Diag, startLine int) []parser.Diag {
	var out []parser.Diag
	for _, d := range diags {
		if d.Pos.Line >= startLine {
			out = append(out, d)
		}
	}
	return out
}

// shift moves diagnostics from session coordinates back to snippet
// coordinates, so a caret lands under the line the user just typed.
func shift(diags []parser.Diag, startLine int) []parser.Diag {
	out := make([]parser.Diag, len(diags))
	for i, d := range diags {
		d.Pos.Line -= startLine - 1
		out[i] = d
	}
	return out
}

// presentable reduces a generated Go file to the lines worth showing at a
// prompt: no package clause, no import block, no main function wrapper, and
// no blank lines. What is left is the code the session has actually produced,
// at the indentation it will have in the finished file minus one level.
func presentable(src string) []string {
	if src == "" {
		return nil
	}
	var out []string
	inImports, inMain := false, false
	for _, line := range strings.Split(src, "\n") {
		switch {
		case inImports:
			if strings.HasPrefix(line, ")") {
				inImports = false
			}
			continue
		case strings.HasPrefix(line, "package "):
			continue
		case strings.HasPrefix(line, "import ("):
			inImports = true
			continue
		case strings.HasPrefix(line, "import "):
			continue
		}
		if !inMain && line == "func main() {" {
			// The comment block the emitter puts above main describes the
			// wrapper, not anything that was typed.
			for len(out) > 0 && strings.HasPrefix(out[len(out)-1], "//") {
				out = out[:len(out)-1]
			}
			inMain = true
			continue
		}
		if inMain {
			if line == "}" {
				inMain = false
				continue
			}
			line = strings.TrimPrefix(line, "\t")
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		if blankAssign.MatchString(line) {
			// In the annotated program the line carries a comment block
			// explaining it. That goes with it.
			for len(out) > 0 && strings.HasPrefix(strings.TrimSpace(out[len(out)-1]), "//") {
				out = out[:len(out)-1]
			}
			// `_ = name` is the emitter satisfying Go's rule against a
			// variable that is never read. In a session it appears the moment
			// a variable is declared and vanishes the moment it is used, so
			// showing it per snippet would be a stream of corrections about
			// something that was never wrong. It stays in `:go full`, where
			// the note beside it is the lesson.
			continue
		}
		out = append(out, line)
	}
	return out
}

// blankAssign matches the emitter's unused-variable line.
var blankAssign = regexp.MustCompile(`^\s*_ = [A-Za-z_][A-Za-z0-9_]*$`)

// deltaOf returns the lines present in next and not in prev, in order, and how
// many of prev's lines next dropped.
//
// A longest-common-subsequence diff rather than a suffix comparison, because
// re-inference can rewrite a line printed several snippets ago and the reader
// is owed both the new line and the fact that an old one changed.
func deltaOf(prev, next []string) (added []string, removed int) {
	table := lcs(prev, next)
	i, j := 0, 0
	for i < len(prev) && j < len(next) {
		switch {
		case prev[i] == next[j]:
			i, j = i+1, j+1
		case table[i+1][j] >= table[i][j+1]:
			removed++
			i++
		default:
			added = append(added, next[j])
			j++
		}
	}
	removed += len(prev) - i
	added = append(added, next[j:]...)
	return added, removed
}

// lcs builds the longest-common-subsequence length table for two line slices.
func lcs(a, b []string) [][]int {
	table := make([][]int, len(a)+1)
	for i := range table {
		table[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	return table
}
