package score

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// entryResult builds a per-entry result from a compact stage spelling, so a
// table-driven test can describe a whole corpus in a few lines.
func entryResult(tier, name, kind string, stages map[Stage]Outcome) EntryResult {
	r := EntryResult{Tier: tier, Name: name, Kind: kind, Stages: map[string]StageResult{}}
	for _, s := range Stages() {
		out, ok := stages[s]
		if !ok {
			out = NotApplicable
		}
		reason := ""
		if out == Fail {
			reason = "because of " + s.String()
		}
		r.Stages[s.String()] = StageResult{Outcome: out, Reason: reason}
	}
	r.EquivalentAnnotated = r.Stages[StageEquivalent.String()]
	return r
}

func passing() map[Stage]Outcome {
	return map[Stage]Outcome{
		StageParsed: Pass, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Pass,
		StageEquivalent: Pass,
	}
}

func TestSummarize(t *testing.T) {
	results := []EntryResult{
		entryResult("tier1", "01", KindConvert, passing()),
		entryResult("tier1", "02", KindConvert, map[Stage]Outcome{
			StageParsed: Pass, StageTyped: Fail, StageEmitted: Pass, StageCompiled: Pass, StageEquivalent: Fail,
		}),
		entryResult("tier2", "01", KindConvert, map[Stage]Outcome{
			StageParsed: Pass, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Pass, StageEquivalent: Skip,
		}),
		entryResult("tier4", "01", KindHonestFailure, map[Stage]Outcome{
			StageParsed: Fail, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Pass, StageHonest: Pass,
		}),
	}
	results[0].Quality = Quality{Todos: 1, Symbols: 4, SymbolsTyped: 4}
	results[1].Quality = Quality{Todos: 2, Symbols: 6, SymbolsTyped: 3, Refusals: 1}
	results[3].Quality = Quality{Todos: 3, Symbols: 2, SymbolsTyped: 1, Approximations: 2}

	tiers, total := Summarize(results)

	if len(tiers) != 3 {
		t.Fatalf("tiers = %d, want 3", len(tiers))
	}
	names := []string{tiers[0].Tier, tiers[1].Tier, tiers[2].Tier}
	if !reflect.DeepEqual(names, []string{"tier1", "tier2", "tier4"}) {
		t.Fatalf("tier order = %v", names)
	}
	if got := tiers[0].Stage(StageTyped); got.Pass != 1 || got.Fail != 1 {
		t.Errorf("tier1 typed = %+v, want 1 pass and 1 fail", got)
	}
	if got := tiers[1].Stage(StageEquivalent); got.Skip != 1 || got.Pass != 0 {
		t.Errorf("tier2 equivalent = %+v, want the skip counted as a skip", got)
	}
	if got := tiers[1].Stage(StageEquivalent).Percent(); got != 0 {
		t.Errorf("a skipped check is not a pass: percent = %v, want 0", got)
	}
	if got := tiers[2].Stage(StageEquivalent); got.Total() != 0 {
		t.Errorf("tier4 has no equivalence standard, got %+v", got)
	}
	if got := tiers[2].Stage(StageHonest); got.Pass != 1 {
		t.Errorf("tier4 honest = %+v, want 1 pass", got)
	}
	if total.Entries != 4 {
		t.Errorf("total entries = %d, want 4", total.Entries)
	}
	if got := total.Stage(StageEquivalent); got.Pass != 1 || got.Fail != 1 || got.Skip != 1 {
		t.Errorf("total equivalent = %+v, want 1/1/1 and tier4 left out", got)
	}
	if total.Quality.Todos != 6 {
		t.Errorf("total todos = %d, want 6", total.Quality.Todos)
	}
	if got := total.Quality.DynamicRate(); got < 0.33 || got > 0.34 {
		t.Errorf("dynamic rate = %v, want 4 of 12", got)
	}
}

func TestStageCountPercent(t *testing.T) {
	tests := []struct {
		name string
		c    StageCount
		want float64
	}{
		{"nothing applicable", StageCount{}, 0},
		{"everything passed", StageCount{Pass: 4}, 100},
		{"skips stay in the denominator", StageCount{Pass: 1, Skip: 1}, 50},
		{"failures stay in the denominator", StageCount{Pass: 3, Fail: 1}, 75},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.Percent(); got != tt.want {
				t.Fatalf("Percent = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFirstFailure(t *testing.T) {
	tests := []struct {
		name   string
		stages map[Stage]Outcome
		want   Stage
		found  bool
	}{
		{
			name:   "the earliest failure wins",
			stages: map[Stage]Outcome{StageParsed: Fail, StageTyped: Fail, StageEmitted: Pass},
			want:   StageParsed, found: true,
		},
		{
			name:   "a later failure is found when the early stages pass",
			stages: map[Stage]Outcome{StageParsed: Pass, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Pass, StageEquivalent: Fail},
			want:   StageEquivalent, found: true,
		},
		{
			name:   "a skip is not a failure",
			stages: map[Stage]Outcome{StageParsed: Pass, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Skip, StageEquivalent: Skip},
			found:  false,
		},
		{
			name:   "nothing failed",
			stages: passing(),
			found:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, ok := entryResult("tier1", "x", KindConvert, tt.stages).FirstFailure()
			if ok != tt.found {
				t.Fatalf("found = %v, want %v", ok, tt.found)
			}
			if ok && got != tt.want {
				t.Fatalf("first failure = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCompareDelta(t *testing.T) {
	prevCard := func() *Scorecard {
		results := []EntryResult{
			entryResult("tier1", "01", KindConvert, passing()),
			entryResult("tier1", "02", KindConvert, map[Stage]Outcome{
				StageParsed: Pass, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Pass, StageEquivalent: Fail,
			}),
		}
		tiers, total := Summarize(results)
		return &Scorecard{Tiers: tiers, Total: total, Entries: results}
	}

	tests := []struct {
		name       string
		prev       *Scorecard
		cur        *Scorecard
		comparable bool
		wantNote   string
		changes    []Change
		gained     []string
		lost       []string
	}{
		{
			name:     "no previous run means no delta",
			prev:     nil,
			cur:      prevCard(),
			wantNote: "no previous scorecard",
		},
		{
			name:       "an unchanged run reports no changes",
			prev:       prevCard(),
			cur:        prevCard(),
			comparable: true,
		},
		{
			name: "an entry that started passing is a gain",
			prev: prevCard(),
			cur: func() *Scorecard {
				results := []EntryResult{
					entryResult("tier1", "01", KindConvert, passing()),
					entryResult("tier1", "02", KindConvert, passing()),
				}
				tiers, total := Summarize(results)
				return &Scorecard{Tiers: tiers, Total: total, Entries: results}
			}(),
			comparable: true,
			changes: []Change{
				{Tier: "tier1", Stage: "equivalent", Was: 1, Now: 2, Applicable: 2},
				{Tier: "TOTAL", Stage: "equivalent", Was: 1, Now: 2, Applicable: 2},
			},
			gained: []string{"tier1/02"},
		},
		{
			name: "an entry that stopped passing is a loss",
			prev: prevCard(),
			cur: func() *Scorecard {
				results := []EntryResult{
					entryResult("tier1", "01", KindConvert, map[Stage]Outcome{
						StageParsed: Pass, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Pass, StageEquivalent: Fail,
					}),
					entryResult("tier1", "02", KindConvert, map[Stage]Outcome{
						StageParsed: Pass, StageTyped: Pass, StageEmitted: Pass, StageCompiled: Pass, StageEquivalent: Fail,
					}),
				}
				tiers, total := Summarize(results)
				return &Scorecard{Tiers: tiers, Total: total, Entries: results}
			}(),
			comparable: true,
			changes: []Change{
				{Tier: "tier1", Stage: "equivalent", Was: 1, Now: 0, Applicable: 2},
				{Tier: "TOTAL", Stage: "equivalent", Was: 1, Now: 0, Applicable: 2},
			},
			lost: []string{"tier1/01"},
		},
		{
			name: "runs that covered different ground are not compared",
			prev: prevCard(),
			cur: func() *Scorecard {
				sc := prevCard()
				sc.Filter = Filter{Tier: "tier1"}
				return sc
			}(),
			wantNote: "not comparable",
		},
		{
			name: "a short run is not compared against a full one",
			prev: prevCard(),
			cur: func() *Scorecard {
				sc := prevCard()
				sc.Filter = Filter{Short: true}
				return sc
			}(),
			wantNote: "no equivalence checks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cur.Compare(tt.prev)
			if got.Comparable != tt.comparable {
				t.Fatalf("Comparable = %v, want %v (%s)", got.Comparable, tt.comparable, got.Note)
			}
			if tt.wantNote != "" && !strings.Contains(got.Note, tt.wantNote) {
				t.Fatalf("note %q does not mention %q", got.Note, tt.wantNote)
			}
			if len(tt.changes) > 0 && !reflect.DeepEqual(got.Changes, tt.changes) {
				t.Errorf("changes = %+v, want %+v", got.Changes, tt.changes)
			}
			if tt.changes == nil && len(got.Changes) > 0 {
				t.Errorf("changes = %+v, want none", got.Changes)
			}
			if !reflect.DeepEqual(got.Gained, tt.gained) {
				t.Errorf("gained = %v, want %v", got.Gained, tt.gained)
			}
			if !reflect.DeepEqual(got.Lost, tt.lost) {
				t.Errorf("lost = %v, want %v", got.Lost, tt.lost)
			}
		})
	}
}

func TestDeltaUsesTheTierStandard(t *testing.T) {
	// A tier 4 entry is judged by the honest stage, so gaining one shows up
	// even though its equivalence stage never applies.
	before := []EntryResult{entryResult("tier4", "01", KindHonestFailure, map[Stage]Outcome{StageHonest: Fail})}
	after := []EntryResult{entryResult("tier4", "01", KindHonestFailure, map[Stage]Outcome{StageHonest: Pass})}
	prevTiers, prevTotal := Summarize(before)
	curTiers, curTotal := Summarize(after)
	prev := &Scorecard{Tiers: prevTiers, Total: prevTotal, Entries: before}
	cur := &Scorecard{Tiers: curTiers, Total: curTotal, Entries: after}

	d := cur.Compare(prev)
	if !d.Improved() {
		t.Fatalf("the run improved; delta = %+v", d)
	}
	if !reflect.DeepEqual(d.Gained, []string{"tier4/01"}) {
		t.Fatalf("gained = %v, want tier4/01", d.Gained)
	}
}

func TestLoadScorecardMissingFileIsNotAnError(t *testing.T) {
	got, err := LoadScorecard(filepath.Join(t.TempDir(), "not-there.json"))
	if err != nil {
		t.Fatalf("a missing scorecard should not be an error, got %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	results := []EntryResult{
		entryResult("tier1", "01", KindConvert, passing()),
		entryResult("tier4", "01", KindHonestFailure, map[Stage]Outcome{
			StageParsed: Fail, StageEmitted: Pass, StageHonest: Pass,
		}),
	}
	tiers, total := Summarize(results)
	sc := &Scorecard{
		FormatVersion: FormatVersion,
		Tool:          "0.1.0",
		Tiers:         tiers,
		Total:         total,
		Quality:       total.Quality,
		Entries:       results,
		Notes:         []string{"tier1/01: something to say"},
	}
	path := filepath.Join(t.TempDir(), "nested", "scorecard.json")
	if err := sc.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadScorecard(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Entries, sc.Entries) {
		t.Errorf("entries did not survive the round trip:\n got %+v\nwant %+v", got.Entries, sc.Entries)
	}
	if !reflect.DeepEqual(got.Tiers, sc.Tiers) {
		t.Errorf("tiers did not survive the round trip:\n got %+v\nwant %+v", got.Tiers, sc.Tiers)
	}
	if d := got.Compare(sc); !d.Comparable || len(d.Changes) != 0 {
		t.Errorf("a scorecard compared against itself should show no change, got %+v", d)
	}
}

func TestOutcomeJSON(t *testing.T) {
	for _, o := range []Outcome{Pass, Fail, Skip, NotApplicable} {
		data, err := o.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var back Outcome
		if err := back.UnmarshalJSON(data); err != nil {
			t.Fatalf("%s: %v", data, err)
		}
		if back != o {
			t.Errorf("%s round-tripped to %s", o, back)
		}
	}
	var bad Outcome
	if err := bad.UnmarshalJSON([]byte(`"nonsense"`)); err == nil {
		t.Error("an unknown outcome should be an error")
	}
}
