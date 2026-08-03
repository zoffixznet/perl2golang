package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"perl2golang/internal/convert"
	"perl2golang/internal/report"
)

// The terminal summary. Six to twelve lines, for the person who runs this
// fifty times in an afternoon.
//
// Every line starts with a fixed-width label or a one-character severity sigil
// in column three, so the eye tracks a column rather than prose. The sigils
// are `!` for a refusal, `~` for a warning and `.` for a note: one character
// each, no colour dependency, and they survive a pipe into a log file. At most
// three diagnostic lines are shown, worst first then source order, followed by
// a count of the rest and the flag that shows them. The last line is always an
// address: the one file to open next.

// The two documents the summary points at.
const (
	docStartHere = "docs/start-here.md"
	docReport    = "docs/conversion-report.md"
)

// maxSummaryDiags is how many diagnostic lines the summary shows before it
// stops and counts the rest. Three is enough to decide whether to open the
// report, which is all the summary is for.
const maxSummaryDiags = 3

// maxSummaryFiles is how many per-file lines a multi-file run shows.
const maxSummaryFiles = 8

// writeSummary prints the block for a finished run.
func writeSummary(w io.Writer, runs []*run, stream streamMode, verbose bool) {
	switch {
	case stream != streamOff:
		for _, r := range runs {
			fmt.Fprintln(w, streamLine(r, verbose))
		}
	case len(runs) == 1:
		fmt.Fprint(w, fileBlock(runs[0], verbose))
	default:
		fmt.Fprint(w, multiBlock(runs, verbose))
	}
}

// streamLine is the whole summary when the artifacts went to standard output.
// The product is then on stdout and the user is watching that, so the run gets
// one line on stderr and no more.
func streamLine(r *run, verbose bool) string {
	if r.failed() {
		return fmt.Sprintf("%s: failed, nothing was written", r.in.display)
	}
	rep := r.res.Report
	line := fmt.Sprintf("%s: converted, %s; %s", r.in.display, verifiedClause(rep), reviewClause(rep))
	if codes := topCodes(rep); codes != "" {
		line += " (" + codes + ")"
		if !verbose {
			line += ", -v to see them in full"
		}
	}
	return line
}

// fileBlock is the summary for a single input converted to a directory.
func fileBlock(r *run, verbose bool) string {
	var b strings.Builder
	if r.failed() {
		// The diagnostic above has already said why. This line exists so that
		// a scrollback full of them still ends with the verdict.
		fmt.Fprintf(&b, "%s -> failed%s\n", r.in.display, timing(r))
		return b.String()
	}
	rep := r.res.Report

	lines, statements := countLines(r.in.src), rep.Stats.Statements
	fmt.Fprintf(&b, "%s -> %s  (%d line%s, %d statement%s, %s)\n",
		r.in.display, r.dir+"/", lines, plural(lines), statements, plural(statements), elapsed(r))
	row := func(label, name string, n int, noun string) {
		fmt.Fprintf(&b, "  %-10s %-20s %4d %s\n", label, name, n, noun+plural(n))
	}
	row("program", "main.go", countLines(r.res.Clean["main.go"]), "line")
	row("annotated", "annotated/main.go", countLines(r.res.Annotated["main.go"]), "line")
	if n := docCount(r.res); n > 0 {
		row("docs", "docs/", n, "file")
	}
	fmt.Fprintf(&b, "  %-10s %s\n", "checked", verifiedClause(rep))
	fmt.Fprintf(&b, "  %-10s %s\n", "review", reviewClause(rep))
	b.WriteString(diagLines(rep, verbose))
	fmt.Fprintf(&b, "  %-10s %s\n", "read", r.dir+"/"+readTarget(r.res))
	return b.String()
}

// multiBlock is the summary for several inputs. It stays the same nine lines
// whatever the file count, because a layout that reorganises itself under load
// has to be re-learned at exactly the wrong moment.
func multiBlock(runs []*run, verbose bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "converting %d files\n", len(runs))

	shown := runs
	if len(shown) > maxSummaryFiles {
		shown = shown[:maxSummaryFiles]
	}
	for _, r := range shown {
		if r.failed() {
			fmt.Fprintf(&b, "  %-7s %-24s %s\n", "failed", r.in.display, "nothing was written")
			continue
		}
		fmt.Fprintf(&b, "  %-7s %-24s %4d statements   %-20s %s\n", "ok", r.in.display,
			r.res.Report.Stats.Statements, r.dir+"/", reviewClause(r.res.Report))
	}
	if n := len(runs) - len(shown); n > 0 {
		fmt.Fprintf(&b, "  + %d more\n", n)
	}

	converted, failed, total := 0, 0, 0.0
	for _, r := range runs {
		total += r.elapsed.Seconds()
		if r.failed() {
			failed++
			continue
		}
		converted++
	}
	b.WriteString("  ---\n")
	fmt.Fprintf(&b, "  %d of %d converted, %d failed, %.2fs\n", converted, len(runs), failed, total)
	if !verbose && anyEntries(runs) {
		b.WriteString("  -v shows every diagnostic in full\n")
	}
	// The last line is always an address: the one file to open next.
	if r := worstRun(runs); r != nil {
		fmt.Fprintf(&b, "  %-7s %s\n", "read", r.dir+"/"+readTarget(r.res))
	}
	return b.String()
}

// diagLines renders the worst few entries, and says how to see the rest. Under
// -v every entry has already been printed in full on stderr, so the summary
// leaves them out rather than saying everything twice.
func diagLines(rep *report.Report, verbose bool) string {
	if verbose || len(rep.Entries) == 0 {
		return ""
	}
	entries := worstFirst(rep)
	var b strings.Builder
	for _, e := range entries[:min(len(entries), maxSummaryDiags)] {
		fmt.Fprintf(&b, "  %s %-8s %s  %s\n", sigil(e.Severity), e.Code, lineColumn(e.Line), e.Short)
	}
	if n := len(entries) - maxSummaryDiags; n > 0 {
		fmt.Fprintf(&b, "  + %d more (-v to show them all)\n", n)
	}
	return b.String()
}

// worstFirst orders entries by severity and then by the order the report is
// already in, which is source order. A refusal is what the reader has to act
// on, so it goes first however late in the file it is.
func worstFirst(rep *report.Report) []report.Entry {
	out := make([]report.Entry, len(rep.Entries))
	copy(out, rep.Entries)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Severity > out[j].Severity })
	return out
}

// topCodes names the first few codes of a run, for the one-line summary.
func topCodes(rep *report.Report) string {
	entries := worstFirst(rep)
	if len(entries) == 0 {
		return ""
	}
	codes := make([]string, 0, maxSummaryDiags)
	for _, e := range entries[:min(len(entries), maxSummaryDiags)] {
		codes = append(codes, e.Code)
	}
	return strings.Join(codes, ", ")
}

// sigil is the one-character severity marker.
func sigil(s report.Severity) string {
	switch s {
	case report.Refuse:
		return "!"
	case report.Warn:
		return "~"
	default:
		return "."
	}
}

// lineColumn renders the source position in the summary's fixed width. An
// entry about the file as a whole has no line, and gets blanks rather than a
// zero that would read as line zero.
func lineColumn(line int) string {
	if line <= 0 {
		return "        "
	}
	return fmt.Sprintf("line %3d", line)
}

// reviewClause counts what came out of the conversion, and says so positively
// when nothing did: the absence of warning lines is not left to be inferred.
func reviewClause(rep *report.Report) string {
	var parts []string
	if n := rep.Stats.Refused; n > 0 {
		parts = append(parts, fmt.Sprintf("%d refusal%s", n, plural(n)))
	}
	if n := rep.Stats.Approximated; n > 0 {
		parts = append(parts, fmt.Sprintf("%d warning%s", n, plural(n)))
	}
	if n := len(rep.Entries) - rep.Stats.Refused - rep.Stats.Approximated; n > 0 {
		parts = append(parts, fmt.Sprintf("%d note%s", n, plural(n)))
	}
	if len(parts) == 0 {
		return "nothing to review"
	}
	return strings.Join(parts, ", ")
}

// strictClause names what tripped --strict, which is refusals and
// approximations and never notes, so the message agrees with the counts the
// summary printed a line earlier.
func strictClause(rep *report.Report) string {
	var parts []string
	if n := rep.Stats.Refused; n > 0 {
		parts = append(parts, fmt.Sprintf("%d refusal%s", n, plural(n)))
	}
	if n := rep.Stats.Approximated; n > 0 {
		parts = append(parts, fmt.Sprintf("%d warning%s", n, plural(n)))
	}
	return strings.Join(parts, " and ")
}

// verifiedClause says how far the tool got in checking its own output. The
// difference between "this compiles" and "this parses" is the difference
// between two very different claims, so it is spelled out rather than implied.
func verifiedClause(rep *report.Report) string {
	v := rep.Verified
	switch {
	case v.Built:
		return "compiled with the Go toolchain"
	case !v.Parsed:
		return "the generated Go does not parse, which is a bug in perl2golang"
	case !v.Toolchain:
		return "parsed only: no Go toolchain was found to compile it"
	default:
		return "parsed, but the Go toolchain rejected it, which is a bug in perl2golang"
	}
}

// readTarget is the one file to open next. It is the report as soon as there
// is anything to review, because the pointer should lead to whatever this
// particular run made important.
func readTarget(res *convert.Result) string {
	if res.Report.Stats.Refused+res.Report.Stats.Approximated > 0 {
		if _, ok := res.Docs[docReport]; ok {
			return docReport
		}
	}
	if _, ok := res.Docs[docStartHere]; ok {
		return docStartHere
	}
	return "main.go"
}

// docCount counts the generated teaching documents.
func docCount(res *convert.Result) int {
	n := 0
	for name := range res.Docs {
		if strings.HasPrefix(name, "docs/") {
			n++
		}
	}
	return n
}

// worstRun picks the finished conversion whose report is most worth reading.
func worstRun(runs []*run) *run {
	var best *run
	bestScore := -1
	for _, r := range runs {
		if r.failed() {
			continue
		}
		score := r.res.Report.Stats.Refused*1000 + r.res.Report.Stats.Approximated
		if score > bestScore {
			best, bestScore = r, score
		}
	}
	return best
}

// anyEntries reports whether any run has something to say.
func anyEntries(runs []*run) bool {
	for _, r := range runs {
		if !r.failed() && len(r.res.Report.Entries) > 0 {
			return true
		}
	}
	return false
}

// elapsed formats one run's wall time. Someone converting a corpus wants to
// see it get faster, so the header carries it always.
func elapsed(r *run) string {
	return fmt.Sprintf("%.2fs", r.elapsed.Seconds())
}

// timing is the parenthesised wall time, or nothing at all for a run that
// never started and so has no time worth reporting.
func timing(r *run) string {
	if r.elapsed == 0 {
		return ""
	}
	return "  (" + elapsed(r) + ")"
}

// plural is the suffix a noun takes for a count of n.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
