package score

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// RenderOptions control how much of the detail the table shows.
type RenderOptions struct {
	// Verbose prints a line per entry and holds nothing back from the
	// failure list.
	Verbose bool
	// MaxFailures caps how many entries each failing stage lists. Zero uses
	// a sensible default; Verbose lifts the cap entirely.
	MaxFailures int
}

const defaultMaxFailures = 12

// Render writes the human-readable scorecard: the table, the quality numbers,
// the entries that failed earliest, and the delta against the previous run.
func Render(w io.Writer, sc *Scorecard, delta Delta, opts RenderOptions) {
	renderTable(w, sc)
	renderAnnotated(w, sc)
	renderQuality(w, sc)
	renderFailures(w, sc, opts)
	renderNotes(w, sc, opts)
	renderDelta(w, delta)
	renderEntries(w, sc, opts)
	fmt.Fprintf(w, "\n%s in %s\n", plural(len(sc.Entries), "entry"), sc.Elapsed.Round(time.Millisecond))
}

// tableStages are the columns of the main table, in ladder order.
var tableStages = []Stage{StageTranslated, StageTyped, StageEmitted, StageCompiled, StageEquivalent, StageHonest}

func renderTable(w io.Writer, sc *Scorecard) {
	header := []string{"tier", "entries"}
	for _, s := range tableStages {
		header = append(header, s.String())
	}
	header = append(header, "skipped")

	rows := [][]string{}
	for _, t := range append(append([]TierScore{}, sc.Tiers...), sc.Total) {
		row := []string{t.Tier, strconv.Itoa(t.Entries)}
		skipped := 0
		for _, s := range tableStages {
			c := t.Stage(s)
			skipped += c.Skip
			row = append(row, cell(c))
		}
		row = append(row, strconv.Itoa(skipped))
		rows = append(rows, row)
	}
	// The total is set apart, because it is the number that gets quoted.
	rule := len(rows) - 2

	fmt.Fprintln(w, "Conversion scorecard")
	if sc.Filter.Partial() {
		fmt.Fprintf(w, "  covering %s\n", describeFilter(sc.Filter))
	}
	fmt.Fprintln(w)
	writeTable(w, header, rows, rule, map[int]bool{0: true})
	fmt.Fprintln(w, "  entries reaching each stage: translated (every construct had a translation,")
	fmt.Fprintln(w, "  so one refusal fails the file), typed (nothing fell back to the dynamic")
	fmt.Fprintln(w, "  value type), emitted (valid Go came out), compiled (the Go toolchain built")
	fmt.Fprintln(w, "  it), equivalent (it printed what perl printed, byte for byte), honest (a")
	fmt.Fprintln(w, "  tier 4 entry said something true about what it could not translate).")
	fmt.Fprintln(w, "  Skipped checks never count as passes.")
	if !sc.Environment.Perl {
		fmt.Fprintln(w, "  "+sc.Environment.PerlWhy)
	}
	if !sc.Environment.Toolchain {
		fmt.Fprintln(w, "  "+sc.Environment.GoWhy)
	}
}

// cell renders one stage tally, or a dash when the stage does not apply.
func cell(c StageCount) string {
	if c.Total() == 0 {
		return "-"
	}
	return fmt.Sprintf("%d/%d (%.0f%%)", c.Pass, c.Total(), c.Percent())
}

func renderAnnotated(w io.Writer, sc *Scorecard) {
	if sc.Total.EquivalentAnnotated.Total() == 0 {
		return
	}
	fmt.Fprintln(w, "\nAnnotated programs, checked against the same Perl")
	same := true
	for _, t := range append(append([]TierScore{}, sc.Tiers...), sc.Total) {
		if t.Stage(StageEquivalent).Pass != t.EquivalentAnnotated.Pass {
			same = false
		}
	}
	fmt.Fprintf(w, "  %s equivalent, against %s for the clean programs\n",
		cell(sc.Total.EquivalentAnnotated), cell(sc.Total.Stage(StageEquivalent)))
	checked := sc.Total.EquivalentAnnotated
	if checked.Pass+checked.Fail == 0 {
		fmt.Fprintln(w, "  no program was run, so the annotations were not checked against anything")
		return
	}
	if same {
		fmt.Fprintln(w, "  the annotations changed nothing: both programs behave the same everywhere")
		return
	}
	for _, e := range sc.Entries {
		if e.Stage(StageEquivalent).passed() && !e.EquivalentAnnotated.passed() {
			fmt.Fprintf(w, "  %s: the annotated program differs: %s\n", e.ID(), truncate(e.EquivalentAnnotated.Reason, 120))
		}
		if !e.Stage(StageEquivalent).passed() && e.EquivalentAnnotated.passed() {
			fmt.Fprintf(w, "  %s: only the annotated program matched, which means the clean one is wrong\n", e.ID())
		}
	}
}

func renderQuality(w io.Writer, sc *Scorecard) {
	q := sc.Quality
	fmt.Fprintln(w, "\nQuality")
	fmt.Fprintf(w, "  TODOs emitted            %d\n", q.Todos)
	dynamic := q.Symbols - q.SymbolsTyped
	fmt.Fprintf(w, "  dynamic fallback         %d of %d symbols (%.1f%%)\n", dynamic, q.Symbols, 100*q.DynamicRate())
	fmt.Fprintf(w, "  constructs refused       %d\n", q.Refusals)
	fmt.Fprintf(w, "  constructs approximated  %d\n", q.Approximations)
	fmt.Fprintf(w, "  programs that panicked   %d\n", q.Crashes)
	var crashed []EntryResult
	for _, e := range sc.Entries {
		if e.Quality.Crashes > 0 {
			crashed = append(crashed, e)
		}
	}
	if len(crashed) == 0 {
		return
	}
	width := 0
	for _, e := range crashed {
		if n := len(e.ID()); n > width {
			width = n
		}
	}
	for _, e := range crashed {
		fmt.Fprintf(w, "    %-*s  %s\n", width, e.ID(), truncate(e.Quality.Panic, 100))
	}
}

func renderFailures(w io.Writer, sc *Scorecard, opts RenderOptions) {
	limit := opts.MaxFailures
	if limit == 0 {
		limit = defaultMaxFailures
	}
	if opts.Verbose {
		limit = len(sc.Entries)
	}

	byStage := map[Stage][]EntryResult{}
	for _, e := range sc.Entries {
		if s, _, ok := e.FirstFailure(); ok {
			byStage[s] = append(byStage[s], e)
		}
	}
	if len(byStage) == 0 {
		fmt.Fprintln(w, "\nNothing failed.")
		return
	}
	fmt.Fprintln(w, "\nWhere entries fall over first")
	for _, s := range Stages() {
		group := byStage[s]
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n  %s (%d)\n", s, len(group))
		shown := group
		if len(shown) > limit {
			shown = shown[:limit]
		}
		width := 0
		for _, e := range shown {
			if n := len(e.ID()); n > width {
				width = n
			}
		}
		for _, e := range shown {
			_, r, _ := e.FirstFailure()
			fmt.Fprintf(w, "    %-*s  %s\n", width, e.ID(), truncate(r.Reason, 140))
		}
		if len(group) > len(shown) {
			fmt.Fprintf(w, "    ... and %d more\n", len(group)-len(shown))
		}
	}
}

func renderNotes(w io.Writer, sc *Scorecard, opts RenderOptions) {
	if len(sc.Notes) == 0 {
		return
	}
	limit := defaultMaxFailures
	if opts.Verbose {
		limit = len(sc.Notes)
	}
	fmt.Fprintf(w, "\nCorpus notes (%d)\n", len(sc.Notes))
	shown := sc.Notes
	if len(shown) > limit {
		shown = shown[:limit]
	}
	for _, n := range shown {
		fmt.Fprintln(w, "  "+truncate(n, 160))
	}
	if len(sc.Notes) > len(shown) {
		fmt.Fprintf(w, "  ... and %d more\n", len(sc.Notes)-len(shown))
	}
}

func renderDelta(w io.Writer, d Delta) {
	fmt.Fprintln(w, "\nSince the last run")
	if !d.Comparable {
		note := d.Note
		if note == "" {
			note = "no previous scorecard to compare against"
		}
		fmt.Fprintln(w, "  "+note)
		return
	}
	if len(d.Changes) == 0 {
		fmt.Fprintln(w, "  no change")
		return
	}
	width := 0
	for _, c := range d.Changes {
		if n := len(c.Tier + " " + c.Stage); n > width {
			width = n
		}
	}
	for _, c := range d.Changes {
		sign := "+"
		if c.Now < c.Was {
			sign = ""
		}
		fmt.Fprintf(w, "  %-*s  %d -> %d of %d  (%s%d)\n",
			width, c.Tier+" "+c.Stage, c.Was, c.Now, c.Applicable, sign, c.Now-c.Was)
	}
	for _, id := range d.Gained {
		fmt.Fprintln(w, "  now passing: "+id)
	}
	for _, id := range d.Lost {
		fmt.Fprintln(w, "  no longer passing: "+id)
	}
}

func renderEntries(w io.Writer, sc *Scorecard, opts RenderOptions) {
	if !opts.Verbose {
		return
	}
	fmt.Fprintln(w, "\nEvery entry")
	header := []string{"entry"}
	for _, s := range tableStages {
		header = append(header, s.String())
	}
	header = append(header, "reason")
	rows := make([][]string, 0, len(sc.Entries))
	for _, e := range sc.Entries {
		row := []string{e.ID()}
		for _, s := range tableStages {
			row = append(row, mark(e.Stage(s)))
		}
		reason := ""
		if _, r, ok := e.FirstFailure(); ok {
			reason = truncate(r.Reason, 90)
		}
		row = append(row, reason)
		rows = append(rows, row)
	}
	writeTable(w, header, rows, -1, map[int]bool{0: true, len(header) - 1: true})
}

// mark renders one stage outcome as a single glyph, for the per-entry table.
func mark(r StageResult) string {
	switch r.Outcome {
	case Pass:
		return "ok"
	case Fail:
		return "FAIL"
	case Skip:
		return "skip"
	default:
		return "-"
	}
}

// writeTable prints a table with columns padded to their widest cell. Columns
// in leftAlign are left-aligned; the rest are right-aligned, which is what makes
// counts readable down a column. A rule is drawn after row index rule.
func writeTable(w io.Writer, header []string, rows [][]string, rule int, leftAlign map[int]bool) {
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, c := range row {
			if i < len(widths) && len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}
	line := func(cells []string) string {
		var b strings.Builder
		b.WriteString("  ")
		for i, c := range cells {
			if i > 0 {
				b.WriteString("  ")
			}
			if leftAlign[i] {
				fmt.Fprintf(&b, "%-*s", widths[i], c)
				continue
			}
			fmt.Fprintf(&b, "%*s", widths[i], c)
		}
		return strings.TrimRight(b.String(), " ")
	}
	fmt.Fprintln(w, line(header))
	for i, row := range rows {
		fmt.Fprintln(w, line(row))
		if i == rule {
			total := 0
			for j, wd := range widths {
				if j > 0 {
					total += 2
				}
				total += wd
			}
			fmt.Fprintln(w, "  "+strings.Repeat("-", total))
		}
	}
}

// jsonOutput is the machine-readable form: the whole scorecard plus the delta
// that was printed beside it.
type jsonOutput struct {
	*Scorecard
	Delta Delta `json:"delta"`
}

// RenderJSON writes the scorecard and its delta as JSON.
func RenderJSON(w io.Writer, sc *Scorecard, delta Delta) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonOutput{Scorecard: sc, Delta: delta})
}
