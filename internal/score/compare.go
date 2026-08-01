package score

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CompareOptions say how strictly two runs must agree.
type CompareOptions struct {
	// AllowStderr drops the stderr comparison, for entries whose stderr
	// output is part of the program rather than a mistake.
	AllowStderr bool
	// SkipStdout drops the stdout comparison, for an entry whose stdout is
	// not reproducible run to run and therefore cannot be diffed at all.
	SkipStdout bool
	// WantLabel and GotLabel name the two sides in the reason text.
	WantLabel string
	GotLabel  string
	// WantFiles and GotFiles are the files each side wrote, as hashes keyed
	// by relative path. Both nil skips the comparison.
	WantFiles map[string]string
	GotFiles  map[string]string
}

// Diff is the verdict on one comparison.
type Diff struct {
	Equal bool
	// Reason states every way the two runs differed, in one line.
	Reason string
}

// Compare decides whether two runs of the same program are the same run.
//
// It compares stdout byte for byte and the exit status exactly. stderr is
// compared only when the entry does not sanction stderr output and one of the
// two runs produced some, because that is the case where stderr carries a real
// difference rather than a deliberate message.
func Compare(want, got Output, opts CompareOptions) Diff {
	wantLabel, gotLabel := opts.WantLabel, opts.GotLabel
	if wantLabel == "" {
		wantLabel = "the Perl"
	}
	if gotLabel == "" {
		gotLabel = "the Go"
	}

	var reasons []string
	switch {
	case want.TimedOut:
		reasons = append(reasons, wantLabel+" ran out of time")
	case want.Err != nil:
		reasons = append(reasons, wantLabel+" would not run: "+want.Err.Error())
	}
	switch {
	case got.TimedOut:
		reasons = append(reasons, gotLabel+" ran out of time")
	case got.Err != nil:
		reasons = append(reasons, gotLabel+" would not run: "+got.Err.Error())
	}
	if len(reasons) > 0 {
		return Diff{Reason: strings.Join(reasons, "; ")}
	}

	if want.Exit != got.Exit {
		reasons = append(reasons, fmt.Sprintf("exit status %d, wanted %d", got.Exit, want.Exit))
	}
	if !opts.SkipStdout && !bytes.Equal(want.Stdout, got.Stdout) {
		reasons = append(reasons, "stdout differs: "+describeBytes(want.Stdout, got.Stdout))
	}
	if !opts.AllowStderr && (len(want.Stderr) > 0 || len(got.Stderr) > 0) && !bytes.Equal(want.Stderr, got.Stderr) {
		reasons = append(reasons, "stderr differs: "+describeStderr(want.Stderr, got.Stderr))
	}
	if opts.WantFiles != nil || opts.GotFiles != nil {
		if d := describeFileDiff(opts.WantFiles, opts.GotFiles); d != "" {
			reasons = append(reasons, "files written differ: "+d)
		}
	}
	if len(reasons) == 0 {
		return Diff{Equal: true}
	}
	return Diff{Reason: strings.Join(reasons, "; ")}
}

// describeBytes says where two byte streams first parted company and shows the
// line it happened on, which is nearly always enough to see what went wrong.
func describeBytes(want, got []byte) string {
	at := 0
	for at < len(want) && at < len(got) && want[at] == got[at] {
		at++
	}
	line := 1 + bytes.Count(want[:at], []byte("\n"))
	switch {
	case at == len(want):
		return fmt.Sprintf("%d extra byte(s) after line %d, starting %s",
			len(got)-at, line, quoteAround(got, at))
	case at == len(got):
		return fmt.Sprintf("%d byte(s) missing after line %d, expected %s",
			len(want)-at, line, quoteAround(want, at))
	default:
		return fmt.Sprintf("line %d, byte %d: got %s, wanted %s",
			line, at, quoteAround(got, at), quoteAround(want, at))
	}
}

// volatile matches the parts of a captured output that say where a run
// happened rather than what it did: the temporary directory this harness makes
// for an entry, and a pointer address a program printed.
var volatile = regexp.MustCompile(`perl2go-score-[A-Za-z0-9_-]*|0x[0-9a-fA-F]{4,}`)

// steady rewrites those parts to a fixed marker.
//
// The scorecard is kept so one run can be compared with the next, which only
// works while a line that reports the same fact reads the same way. A random
// directory name or a heap address quoted in a reason changes every run on its
// own, and a record that changes on its own is a record nobody can read a
// change out of.
func steady(reason string) string {
	return volatile.ReplaceAllStringFunc(reason, func(m string) string {
		if strings.HasPrefix(m, "0x") {
			return "0xADDR"
		}
		return "perl2go-score-TMP"
	})
}

// steadyResult is steady applied to a stage result.
func steadyResult(r StageResult) StageResult {
	r.Reason = steady(r.Reason)
	return r
}

// describeStderr says how two error streams differed, by what was said rather
// than by how much of it there was.
//
// A crash prints a stack trace naming the directory the program was built in,
// and that directory has a different name every run, so a length taken from it
// would change while the fault behind it stayed exactly the same. The first
// line is what identifies the fault, and it is the same line every time.
func describeStderr(want, got []byte) string {
	switch {
	case len(want) == 0:
		return "unexpected " + quoteFirstLine(got)
	case len(got) == 0:
		return "nothing, expected " + quoteFirstLine(want)
	}
	return "got " + quoteFirstLine(got) + ", wanted " + quoteFirstLine(want)
}

// quoteFirstLine quotes the first line of a stream, shortened to fit a table.
func quoteFirstLine(b []byte) string {
	return strconv.Quote(truncate(firstLine(string(b)), 72))
}

// quoteAround quotes a short window of a byte stream starting at an offset.
//
// The window is steadied before it is cut, not after: a pointer address is a
// different length every run, so cutting first would move where the window ends
// and change the quoted text even where the program's behaviour did not.
func quoteAround(b []byte, at int) string {
	const window = 32
	if at > len(b) {
		at = len(b)
	}
	end := at + 4*window
	if end > len(b) {
		end = len(b)
	}
	text := steady(string(b[at:end]))
	more := end < len(b)
	if len(text) > window {
		text, more = text[:window], true
	}
	s := strconv.Quote(text)
	if more {
		s = s[:len(s)-1] + `..."`
	}
	return s
}
