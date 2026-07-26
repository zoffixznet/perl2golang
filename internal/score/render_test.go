package score

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// renderFixture is a small scorecard covering every shape the table has to
// print: a clean tier, a tier with failures, a tier with skips, and tier 4 with
// its own standard.
func renderFixture() *Scorecard {
	results := []EntryResult{
		entryResult("tier1", "01-hello", KindConvert, passing()),
		entryResult("tier1", "02-sort", KindConvert, map[Stage]Outcome{
			StageParsed: Pass, StageTyped: Fail, StageEmitted: Pass, StageCompiled: Pass, StageEquivalent: Fail,
		}),
		entryResult("tier2", "01-getopt", KindConvert, map[Stage]Outcome{
			StageParsed: Fail, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Pass, StageEquivalent: Skip,
		}),
		entryResult("tier4", "01-eval", KindHonestFailure, map[Stage]Outcome{
			StageParsed: Fail, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Pass, StageHonest: Pass,
		}),
	}
	results[0].Quality = Quality{Symbols: 5, SymbolsTyped: 5}
	results[1].Quality = Quality{Todos: 2, Symbols: 5, SymbolsTyped: 2, Refusals: 2}
	results[2].Quality = Quality{Todos: 1, Symbols: 4, SymbolsTyped: 4, Approximations: 1}
	tiers, total := Summarize(results)
	return &Scorecard{
		FormatVersion: FormatVersion,
		Tool:          "0.1.0",
		Environment:   Environment{Perl: true, Toolchain: true},
		Tiers:         tiers,
		Total:         total,
		Quality:       total.Quality,
		Entries:       results,
		Elapsed:       2500 * time.Millisecond,
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name     string
		card     func() *Scorecard
		delta    Delta
		opts     RenderOptions
		contains []string
		absent   []string
	}{
		{
			name: "the table has a row per tier and a total",
			card: renderFixture,
			contains: []string{
				"Conversion scorecard",
				"tier", "entries", "parsed", "typed", "emitted", "compiled", "equivalent", "honest", "skipped",
				"tier1", "tier2", "tier4", "TOTAL",
			},
		},
		{
			name: "the quality numbers are printed",
			card: renderFixture,
			contains: []string{
				"TODOs emitted            3",
				"dynamic fallback         3 of 14 symbols (21.4%)",
				"constructs refused       2",
				"constructs approximated  1",
			},
		},
		{
			name: "entries are grouped under the stage they first failed",
			card: renderFixture,
			contains: []string{
				"Where entries fall over first",
				"parsed (2)",
				"tier2/01-getopt",
				"typed (1)",
				"tier1/02-sort",
			},
		},
		{
			name:     "no previous run is stated rather than shown as a collapse",
			card:     renderFixture,
			delta:    Delta{Note: "no previous scorecard to compare against"},
			contains: []string{"Since the last run", "no previous scorecard"},
		},
		{
			name:     "an unchanged run says so",
			card:     renderFixture,
			delta:    Delta{Comparable: true},
			contains: []string{"no change"},
		},
		{
			name: "a delta prints the direction of every change",
			card: renderFixture,
			delta: Delta{
				Comparable: true,
				Changes: []Change{
					{Tier: "tier1", Stage: "equivalent", Was: 1, Now: 2, Applicable: 2},
					{Tier: "tier2", Stage: "compiled", Was: 3, Now: 1, Applicable: 4},
				},
				Gained: []string{"tier1/02-sort"},
				Lost:   []string{"tier2/09-thing"},
			},
			contains: []string{
				"tier1 equivalent", "1 -> 2 of 2  (+1)",
				"tier2 compiled", "3 -> 1 of 4  (-2)",
				"now passing: tier1/02-sort",
				"no longer passing: tier2/09-thing",
			},
		},
		{
			name:     "the per-entry table only appears with -v",
			card:     renderFixture,
			opts:     RenderOptions{Verbose: true},
			contains: []string{"Every entry", "tier1/01-hello", "ok", "FAIL"},
		},
		{
			name:   "the per-entry table is left out without -v",
			card:   renderFixture,
			absent: []string{"Every entry"},
		},
		{
			name: "a missing perl is stated in the table, not hidden",
			card: func() *Scorecard {
				sc := renderFixture()
				sc.Environment = Environment{Toolchain: true, PerlWhy: "perl was not found on PATH, so nothing could be compared against it"}
				return sc
			},
			contains: []string{"perl was not found on PATH"},
		},
		{
			name: "a partial run says what it covered",
			card: func() *Scorecard {
				sc := renderFixture()
				sc.Filter = Filter{Tier: "tier2", Short: true}
				return sc
			},
			contains: []string{"covering tier tier2 with no equivalence checks"},
		},
		{
			name: "corpus notes are printed",
			card: func() *Scorecard {
				sc := renderFixture()
				sc.Notes = []string{"tier1/01-hello: perl exits 1, expected_exit records 0"}
				return sc
			},
			contains: []string{"Corpus notes (1)", "perl exits 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			Render(&b, tt.card(), tt.delta, tt.opts)
			got := b.String()
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("output does not contain %q\n---\n%s", want, got)
				}
			}
			for _, unwanted := range tt.absent {
				if strings.Contains(got, unwanted) {
					t.Errorf("output should not contain %q\n---\n%s", unwanted, got)
				}
			}
		})
	}
}

func TestRenderFailureListIsCapped(t *testing.T) {
	var results []EntryResult
	for i := 0; i < 40; i++ {
		results = append(results, entryResult("tier1", string(rune('a'+i%26))+"-entry", KindConvert,
			map[Stage]Outcome{StageParsed: Fail}))
	}
	tiers, total := Summarize(results)
	sc := &Scorecard{Tiers: tiers, Total: total, Entries: results}

	var short strings.Builder
	Render(&short, sc, Delta{}, RenderOptions{})
	if !strings.Contains(short.String(), "and 28 more") {
		t.Errorf("the failure list should be capped, got:\n%s", short.String())
	}

	var long strings.Builder
	Render(&long, sc, Delta{}, RenderOptions{Verbose: true})
	if strings.Contains(long.String(), "more") {
		t.Errorf("-v should print the whole list")
	}
}

func TestRenderAnnotatedDivergence(t *testing.T) {
	results := []EntryResult{entryResult("tier1", "01", KindConvert, passing())}
	results[0].EquivalentAnnotated = StageResult{Outcome: Fail, Reason: "stdout differs: line 3"}
	tiers, total := Summarize(results)
	sc := &Scorecard{Tiers: tiers, Total: total, Entries: results}

	var b strings.Builder
	Render(&b, sc, Delta{}, RenderOptions{})
	got := b.String()
	if !strings.Contains(got, "the annotated program differs") {
		t.Errorf("a divergent annotated program must be called out:\n%s", got)
	}
	if strings.Contains(got, "the annotations changed nothing") {
		t.Errorf("the output claims the annotations are cosmetic when they are not:\n%s", got)
	}
}

func TestRenderJSON(t *testing.T) {
	var b strings.Builder
	if err := RenderJSON(&b, renderFixture(), Delta{Comparable: true}); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{`"tier": "tier1"`, `"equivalent"`, `"delta"`, `"quality"`, `"outcome": "pass"`} {
		if !strings.Contains(got, want) {
			t.Errorf("JSON output does not contain %q\n%s", want, got)
		}
	}
}

// row returns the table row for a tier, as its whitespace-separated cells.
func row(out, tier string) []string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == tier {
			return fields
		}
	}
	return nil
}

func TestRenderTableCells(t *testing.T) {
	var b strings.Builder
	Render(&b, renderFixture(), Delta{}, RenderOptions{})
	out := b.String()

	tests := []struct {
		tier string
		want []string
	}{
		{"tier1", []string{"tier1", "2", "2/2", "(100%)", "1/2", "(50%)", "2/2", "(100%)", "2/2", "(100%)", "1/2", "(50%)", "-", "0"}},
		{"tier2", []string{"tier2", "1", "0/1", "(0%)", "1/1", "(100%)", "1/1", "(100%)", "1/1", "(100%)", "0/1", "(0%)", "-", "1"}},
		{"tier4", []string{"tier4", "1", "0/1", "(0%)", "1/1", "(100%)", "1/1", "(100%)", "1/1", "(100%)", "-", "1/1", "(100%)", "0"}},
		{"TOTAL", []string{"TOTAL", "4", "2/4", "(50%)", "3/4", "(75%)", "4/4", "(100%)", "4/4", "(100%)", "1/3", "(33%)", "1/1", "(100%)", "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			got := row(out, tt.tier)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("row = %v\nwant %v\n---\n%s", got, tt.want, out)
			}
		})
	}
}

func TestRenderAnnotatedWithNothingRun(t *testing.T) {
	results := []EntryResult{entryResult("tier1", "01", KindConvert, map[Stage]Outcome{
		StageParsed: Pass, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Skip, StageEquivalent: Skip,
	})}
	tiers, total := Summarize(results)
	sc := &Scorecard{Tiers: tiers, Total: total, Entries: results}

	var b strings.Builder
	Render(&b, sc, Delta{}, RenderOptions{})
	if got := b.String(); strings.Contains(got, "the annotations changed nothing") {
		t.Errorf("nothing was run, so nothing can be claimed about the annotations:\n%s", got)
	}
}
