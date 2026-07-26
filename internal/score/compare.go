package score

import (
	"bytes"
	"fmt"
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
		reasons = append(reasons, "stderr differs: "+describeBytes(want.Stderr, got.Stderr))
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

// quoteAround quotes a short window of a byte stream starting at an offset.
func quoteAround(b []byte, at int) string {
	const window = 32
	if at > len(b) {
		at = len(b)
	}
	end := at + window
	if end > len(b) {
		end = len(b)
	}
	s := strconv.Quote(string(b[at:end]))
	if end < len(b) {
		s = s[:len(s)-1] + `..."`
	}
	return s
}
