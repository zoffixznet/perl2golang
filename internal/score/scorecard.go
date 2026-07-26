package score

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FormatVersion is the shape of the scorecard file. It changes when a reader
// would otherwise misread an older file.
const FormatVersion = 1

// Scorecard is one run of the corpus: what every entry did, summed up per tier.
type Scorecard struct {
	FormatVersion int `json:"format_version"`
	// Tool is the version of the converter that produced these numbers.
	Tool string `json:"tool_version"`
	// Recorded is when the run finished. It is never part of a comparison.
	Recorded time.Time `json:"recorded"`
	// Filter records the restriction the run was made under, so a partial
	// run is never mistaken for a full one.
	Filter Filter `json:"filter"`
	// Environment says which external tools were available, because a
	// missing one turns stages into skips.
	Environment Environment `json:"environment"`

	Tiers []TierScore `json:"tiers"`
	Total TierScore   `json:"total"`

	// Quality holds the numbers that are not stages: TODOs left in the
	// output, and how often type inference gave up.
	Quality Quality `json:"quality"`

	// Entries is the per-entry detail, in tier and name order.
	Entries []EntryResult `json:"entries"`

	// Notes are problems with the corpus itself, such as an entry whose
	// recorded output no longer matches what perl prints.
	Notes []string `json:"notes,omitempty"`

	// Elapsed is how long the run took.
	Elapsed time.Duration `json:"elapsed_ns"`
}

// Filter records how the run was restricted.
type Filter struct {
	Tier string `json:"tier,omitempty"`
	Only string `json:"only,omitempty"`
	// Short is true when the equivalence stage was skipped wholesale.
	Short bool `json:"short,omitempty"`
}

// Partial reports whether the run covered less than the whole corpus, which
// makes its totals incomparable with a full run.
func (f Filter) Partial() bool { return f.Tier != "" || f.Only != "" || f.Short }

// Environment is what the run had to work with.
type Environment struct {
	Perl      bool   `json:"perl"`
	Toolchain bool   `json:"go_toolchain"`
	PerlWhy   string `json:"perl_note,omitempty"`
	GoWhy     string `json:"go_note,omitempty"`
}

// Quality is the pair of numbers the scorecard tracks beside the stages.
type Quality struct {
	Todos        int `json:"todos"`
	Symbols      int `json:"symbols"`
	SymbolsTyped int `json:"symbols_typed"`
	// Refusals and Approximations count reported constructs across the run.
	Refusals       int `json:"refusals"`
	Approximations int `json:"approximations"`
}

// DynamicRate is the share of variables that stayed dynamic, in the range 0 to
// 1. It is 0 when nothing was tracked, because no fallback happened.
func (q Quality) DynamicRate() float64 {
	if q.Symbols == 0 {
		return 0
	}
	return float64(q.Symbols-q.SymbolsTyped) / float64(q.Symbols)
}

// StageCount is one stage's tally for one tier.
type StageCount struct {
	Pass int `json:"pass"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`
}

// Total is how many entries the stage applied to at all.
func (c StageCount) Total() int { return c.Pass + c.Fail + c.Skip }

// Percent is the share of applicable entries that passed. Skipped entries stay
// in the denominator: a check that could not be run is not a check that passed,
// and hiding it would make a bare machine look like a perfect one.
func (c StageCount) Percent() float64 {
	if c.Total() == 0 {
		return 0
	}
	return 100 * float64(c.Pass) / float64(c.Total())
}

// TierScore is one row of the table.
type TierScore struct {
	Tier    string `json:"tier"`
	Entries int    `json:"entries"`
	// Stages is keyed by stage name.
	Stages map[string]StageCount `json:"stages"`
	// EquivalentAnnotated is the equivalence tally for the annotated
	// program, kept apart from the clean one so the claim that the
	// annotations are cosmetic is a measurement rather than an assertion.
	EquivalentAnnotated StageCount `json:"equivalent_annotated"`
	Quality             Quality    `json:"quality"`
}

// Stage returns a stage's tally, zero when the tier never reached it.
func (t TierScore) Stage(s Stage) StageCount {
	if t.Stages == nil {
		return StageCount{}
	}
	return t.Stages[s.String()]
}

// EntryResult is everything the run learned about one corpus entry.
type EntryResult struct {
	Tier string `json:"tier"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Categories are the tier 4 expectation categories the entry accepts.
	Categories []string `json:"categories,omitempty"`

	Stages map[string]StageResult `json:"stages"`
	// EquivalentAnnotated is the annotated program's equivalence result.
	EquivalentAnnotated StageResult `json:"equivalent_annotated"`

	Quality Quality `json:"quality"`
	// Notes carry facts that do not change the score: a disagreement between
	// the manifest and the entry's directory, or perl no longer printing
	// what the entry recorded.
	Notes []string `json:"notes,omitempty"`

	Elapsed time.Duration `json:"elapsed_ns"`
}

// ID is "tier2/14-regex-captures".
func (r EntryResult) ID() string { return r.Tier + "/" + r.Name }

// Stage returns one stage's result.
func (r EntryResult) Stage(s Stage) StageResult {
	if r.Stages == nil {
		return StageResult{Outcome: NotApplicable}
	}
	return r.Stages[s.String()]
}

// FirstFailure returns the earliest stage the entry failed, and true, or false
// when it failed nothing. Skips are not failures, so an entry that could not be
// checked is not reported as broken.
func (r EntryResult) FirstFailure() (Stage, StageResult, bool) {
	for _, s := range Stages() {
		if got := r.Stage(s); got.Outcome == Fail {
			return s, got, true
		}
	}
	return 0, StageResult{}, false
}

// Summarize turns per-entry results into the per-tier and total rows.
func Summarize(results []EntryResult) ([]TierScore, TierScore) {
	byTier := map[string]*TierScore{}
	var order []string
	total := &TierScore{Tier: "TOTAL", Stages: map[string]StageCount{}}

	add := func(t *TierScore, r EntryResult) {
		t.Entries++
		for _, s := range Stages() {
			got := r.Stage(s)
			c := t.Stages[s.String()]
			switch got.Outcome {
			case Pass:
				c.Pass++
			case Fail:
				c.Fail++
			case Skip:
				c.Skip++
			default:
				continue
			}
			t.Stages[s.String()] = c
		}
		switch r.EquivalentAnnotated.Outcome {
		case Pass:
			t.EquivalentAnnotated.Pass++
		case Fail:
			t.EquivalentAnnotated.Fail++
		case Skip:
			t.EquivalentAnnotated.Skip++
		}
		t.Quality.Todos += r.Quality.Todos
		t.Quality.Symbols += r.Quality.Symbols
		t.Quality.SymbolsTyped += r.Quality.SymbolsTyped
		t.Quality.Refusals += r.Quality.Refusals
		t.Quality.Approximations += r.Quality.Approximations
	}

	for _, r := range results {
		t, ok := byTier[r.Tier]
		if !ok {
			t = &TierScore{Tier: r.Tier, Stages: map[string]StageCount{}}
			byTier[r.Tier] = t
			order = append(order, r.Tier)
		}
		add(t, r)
		add(total, r)
	}

	sort.Slice(order, func(i, j int) bool { return tierLess(order[i], order[j]) })
	tiers := make([]TierScore, 0, len(order))
	for _, name := range order {
		tiers = append(tiers, *byTier[name])
	}
	return tiers, *total
}

// tierLess orders the numbered tiers first and everything else after them,
// which is the order the corpus is meant to be read in.
func tierLess(a, b string) bool {
	na, oka := tierNumber(a)
	nb, okb := tierNumber(b)
	if oka != okb {
		return oka
	}
	if oka && na != nb {
		return na < nb
	}
	return a < b
}

func tierNumber(s string) (int, bool) {
	var n int
	if _, err := fmt.Sscanf(s, "tier%d", &n); err == nil {
		return n, true
	}
	return 0, false
}

// LoadScorecard reads a scorecard file. A missing file is not an error: it just
// means there is nothing to compare against yet.
func LoadScorecard(path string) (*Scorecard, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sc Scorecard
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &sc, nil
}

// Save writes a scorecard, creating the directory if it is missing.
func (s *Scorecard) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// TierScoreFor returns a tier's row by name.
func (s *Scorecard) TierScoreFor(name string) (TierScore, bool) {
	if name == s.Total.Tier {
		return s.Total, true
	}
	for _, t := range s.Tiers {
		if t.Tier == name {
			return t, true
		}
	}
	return TierScore{}, false
}

// Change is one difference between two runs of the scorecard.
type Change struct {
	Tier  string `json:"tier"`
	Stage string `json:"stage"`
	// Was and Now are the passing counts.
	Was int `json:"was"`
	Now int `json:"now"`
	// Applicable is how many entries the stage applied to now, so a delta
	// can be read against its denominator.
	Applicable int `json:"applicable"`
}

// Delta is the difference between the current run and the previous one.
type Delta struct {
	// Comparable is false when the two runs cannot be honestly compared,
	// with Note saying why.
	Comparable bool     `json:"comparable"`
	Note       string   `json:"note,omitempty"`
	Changes    []Change `json:"changes,omitempty"`
	// EntriesGained and EntriesLost name entries that started or stopped
	// passing their tier's headline stage.
	Gained []string `json:"gained,omitempty"`
	Lost   []string `json:"lost,omitempty"`
}

// Improved reports whether every change was upward.
func (d Delta) Improved() bool {
	if !d.Comparable || len(d.Changes) == 0 {
		return false
	}
	for _, c := range d.Changes {
		if c.Now < c.Was {
			return false
		}
	}
	return true
}

// Compare works out what changed between a previous scorecard and this one.
//
// Two runs are only comparable when they covered the same ground: comparing a
// full run against a run of one tier would report a collapse that never
// happened, so that comparison is refused rather than reported.
func (s *Scorecard) Compare(prev *Scorecard) Delta {
	if prev == nil {
		return Delta{Note: "no previous scorecard to compare against"}
	}
	if prev.Filter != s.Filter {
		return Delta{Note: fmt.Sprintf("the previous run covered %s and this one covered %s, so the two are not comparable",
			describeFilter(prev.Filter), describeFilter(s.Filter))}
	}
	d := Delta{Comparable: true}

	names := map[string]bool{}
	var order []string
	for _, t := range append(append([]TierScore{}, prev.Tiers...), s.Tiers...) {
		if !names[t.Tier] {
			names[t.Tier] = true
			order = append(order, t.Tier)
		}
	}
	sort.Slice(order, func(i, j int) bool { return tierLess(order[i], order[j]) })
	order = append(order, s.Total.Tier)

	for _, tier := range order {
		was, _ := prev.TierScoreFor(tier)
		now, _ := s.TierScoreFor(tier)
		for _, st := range Stages() {
			a, b := was.Stage(st), now.Stage(st)
			if a.Pass == b.Pass {
				continue
			}
			d.Changes = append(d.Changes, Change{
				Tier: tier, Stage: st.String(),
				Was: a.Pass, Now: b.Pass, Applicable: b.Total(),
			})
		}
	}

	prevEntries := map[string]EntryResult{}
	for _, e := range prev.Entries {
		prevEntries[e.ID()] = e
	}
	for _, e := range s.Entries {
		old, ok := prevEntries[e.ID()]
		if !ok {
			continue
		}
		st := headlineStage(e.Kind)
		switch {
		case e.Stage(st).passed() && !old.Stage(st).passed():
			d.Gained = append(d.Gained, e.ID())
		case !e.Stage(st).passed() && old.Stage(st).passed():
			d.Lost = append(d.Lost, e.ID())
		}
	}
	sort.Strings(d.Gained)
	sort.Strings(d.Lost)
	return d
}

// headlineStage is the stage an entry of this kind is judged by.
func headlineStage(kind string) Stage {
	if kind == KindHonestFailure {
		return StageHonest
	}
	return StageEquivalent
}

// describeFilter puts a run's coverage into words, for the table header and for
// the refusal to compare two runs that did not cover the same ground.
func describeFilter(f Filter) string {
	var parts []string
	if f.Tier != "" {
		parts = append(parts, "tier "+f.Tier)
	}
	if f.Only != "" {
		parts = append(parts, "names matching "+f.Only)
	}
	out := "the whole corpus"
	if len(parts) > 0 {
		out = strings.Join(parts, ", ")
	}
	if f.Short {
		out += " with no equivalence checks"
	}
	return out
}
