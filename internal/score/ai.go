package score

// This file is the with-and-without-AI comparison. The corpus knows what
// every program should print, which makes it the one place the strongest
// possible check on an AI-modified program is available: run it and compare
// against real perl, byte for byte. An entry the model changed is measured
// here exactly as the deterministic one is, and a change that no longer
// matches perl's output is a rejection, whatever else it improved.

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"perl2golang/internal/convert"
	"perl2golang/internal/idioms"
)

// DefaultAIConvertTimeout bounds one conversion with a model in the loop. A
// deterministic conversion takes well under a second; one that waits on a
// local model is dominated by generation and by queueing behind the other
// workers, so the ceiling is minutes, not seconds.
const DefaultAIConvertTimeout = 15 * time.Minute

// AISession is one entry's improvement pass, built fresh per entry so the
// numbers it reports belong to that entry alone.
type AISession struct {
	// Improver is passed to the converter as its optional post-pass.
	Improver convert.Improver
	// Activity, when set, is read after the entry finishes: what the model
	// proposed, what was kept, and which gate turned down each rejection.
	Activity func() AIActivity
}

// AIActivity is what one entry's session did, in the session's own numbers.
type AIActivity struct {
	Proposed, Accepted, Rejected int
	// Gates aggregates the rejections by the job that proposed the change and
	// the gate that turned it down.
	Gates []GateCount
}

// GateCount is one gate's rejection count for one job.
type GateCount struct {
	Job   string `json:"job"`
	Gate  string `json:"gate"`
	Count int    `json:"count"`
}

// AIEntry is what one entry's second, model-assisted conversion did, next to
// the deterministic one.
type AIEntry struct {
	// Outcome is the comparison verdict:
	//
	//	improved   a stage the deterministic run failed now passes
	//	rejected   a stage the deterministic run passed now fails, so the
	//	           deterministic output is the one that counts
	//	changed    the output differs but every stage came out the same
	//	unchanged  the model changed nothing
	//	error      the conversion with the model in the loop did not finish
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
	// Changed reports whether any generated file differs from the
	// deterministic conversion.
	Changed bool `json:"changed"`
	// Accepted and Rejected count the model's changes: applied, and turned
	// down by the improver's own checks.
	Accepted int `json:"accepted,omitempty"`
	Rejected int `json:"rejected,omitempty"`
	// Gates says which check turned down each rejected change.
	Gates []GateCount `json:"gates,omitempty"`

	// HitsBefore and HitsAfter count antipattern hits on the clean rendering,
	// deterministic and model-assisted. This is the measurement that can see
	// an idiom improvement in a program that was already passing every stage:
	// the outcome above only moves when a stage flips, and on a mostly green
	// corpus a stage almost never can.
	HitsBefore int `json:"hits_before"`
	HitsAfter  int `json:"hits_after"`

	Compiled            StageResult `json:"compiled"`
	Equivalent          StageResult `json:"equivalent"`
	EquivalentAnnotated StageResult `json:"equivalent_annotated"`

	Elapsed time.Duration `json:"elapsed_ns,omitempty"`
}

// runAIEntry converts the entry a second time with the model in the loop and
// compares the result with the deterministic one, stage by stage.
func (r *runner) runAIEntry(ctx context.Context, e Entry, f *Fixture, det *convert.Result, detRes *EntryResult) *AIEntry {
	start := time.Now()
	out := &AIEntry{}
	finish := func() *AIEntry {
		out.Elapsed = time.Since(start)
		return out
	}

	session := r.opts.AISession()
	timeout := r.opts.AIConvertTimeout
	if timeout <= 0 {
		timeout = DefaultAIConvertTimeout
	}
	conv, cerr := withDeadline(ctx, timeout, func() (*convert.Result, error) {
		return convert.Convert(f.Source, convert.Options{
			Path:    "input.pl",
			Modules: convert.FilesBeside(f.Dir),
			Improve: session.Improver,
		})
	})
	if session.Activity != nil {
		a := session.Activity()
		out.Accepted, out.Rejected, out.Gates = a.Accepted, a.Rejected, a.Gates
	}
	if cerr != nil {
		out.Outcome, out.Reason = "error", firstLine(cerr.Error())
		return finish()
	}

	out.Changed = filesDiffer(det, conv)
	out.HitsBefore, _ = idioms.Count(det.Clean)
	out.HitsAfter, _ = idioms.Count(conv.Clean)
	if r.opts.AIDumpDir != "" && out.Changed {
		r.dumpAIEntry(e, det, conv)
	}
	class := Classify(conv.Report)
	compiled, bins := r.compile(ctx, e, conv, class)
	if bins != nil {
		defer bins.cleanup()
	}
	scratch := EntryResult{Tier: e.Tier, Name: e.Name, Kind: e.Kind, Stages: map[string]StageResult{}}
	clean, annotated := r.equivalence(ctx, e, f, conv, compiled, bins, &scratch)
	out.Compiled, out.Equivalent, out.EquivalentAnnotated = compiled, clean, annotated

	out.Outcome, out.Reason = aiOutcome(out.Changed,
		[]stagePair{
			{"compiled", detRes.Stages[StageCompiled.String()], compiled},
			{"equivalent", detRes.Stages[StageEquivalent.String()], clean},
			{"equivalent (annotated)", detRes.EquivalentAnnotated, annotated},
		})
	return finish()
}

// stagePair is one stage's deterministic and model-assisted result.
type stagePair struct {
	name     string
	det, but StageResult
}

// aiOutcome is the comparison verdict. A stage that got worse outweighs
// everything, because the deterministic output is then the one to ship; only
// with nothing worse does a stage that got better count as an improvement.
func aiOutcome(changed bool, stages []stagePair) (string, string) {
	if !changed {
		return "unchanged", ""
	}
	for _, s := range stages {
		if s.det.passed() && !s.but.passed() {
			return "rejected", fmt.Sprintf("%s pass -> %s: %s", s.name, s.but.Outcome, s.but.Reason)
		}
	}
	for _, s := range stages {
		if !s.det.passed() && s.but.passed() && s.det.Outcome != NotApplicable && s.det.Outcome != Skip {
			return "improved", fmt.Sprintf("%s %s -> pass", s.name, s.det.Outcome)
		}
	}
	return "changed", "the rewritten output passes and fails the same stages"
}

// dumpAIEntry writes the two conversions' clean renderings side by side under
// the dump directory, so a person can read the diff the model actually made.
// A failure to dump costs the inspection aid, never the measurement, so
// errors are deliberately dropped.
func (r *runner) dumpAIEntry(e Entry, det, conv *convert.Result) {
	for sub, files := range map[string]map[string][]byte{"det": det.Clean, "ai": conv.Clean} {
		dir := filepath.Join(r.opts.AIDumpDir, e.Tier+"-"+e.Name, sub)
		for name, content := range files {
			path := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				continue
			}
			_ = os.WriteFile(path, content, 0o644)
		}
	}
}

// filesDiffer reports whether the two conversions generated different code.
func filesDiffer(a, b *convert.Result) bool {
	return renderingDiffers(a.Clean, b.Clean) || renderingDiffers(a.Annotated, b.Annotated)
}

func renderingDiffers(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return true
	}
	for _, name := range slices.Sorted(maps.Keys(a)) {
		other, ok := b[name]
		if !ok || string(a[name]) != string(other) {
			return true
		}
	}
	return false
}

// AITotals sums the per-entry comparisons for the rendering.
type AITotals struct {
	Entries, Improved, Changed, Unchanged, Rejected, Errors, Skipped int
	Accepted, RejectedChanges                                        int
	// HitsBefore and HitsAfter total the antipattern hits over every measured
	// entry, deterministic and model-assisted; Tidier and Untidier count the
	// entries whose hits went down and up.
	HitsBefore, HitsAfter, Tidier, Untidier int
	// Gates aggregates every rejection across the run by job and gate, most
	// frequent first.
	Gates   []GateCount
	Elapsed time.Duration
}

// SummarizeAI adds up what the comparison found. Entries without an AI result
// (tier 4's honest-failure entries, and any entry whose deterministic
// conversion already failed) count as skipped.
func SummarizeAI(entries []EntryResult) AITotals {
	var t AITotals
	gates := map[GateCount]int{}
	for _, e := range entries {
		t.Entries++
		if e.AI == nil {
			t.Skipped++
			continue
		}
		t.Accepted += e.AI.Accepted
		t.RejectedChanges += e.AI.Rejected
		t.Elapsed += e.AI.Elapsed
		t.HitsBefore += e.AI.HitsBefore
		t.HitsAfter += e.AI.HitsAfter
		switch {
		case e.AI.HitsAfter < e.AI.HitsBefore:
			t.Tidier++
		case e.AI.HitsAfter > e.AI.HitsBefore:
			t.Untidier++
		}
		for _, g := range e.AI.Gates {
			gates[GateCount{Job: g.Job, Gate: g.Gate}] += g.Count
		}
		switch e.AI.Outcome {
		case "improved":
			t.Improved++
		case "changed":
			t.Changed++
		case "rejected":
			t.Rejected++
		case "error":
			t.Errors++
		default:
			t.Unchanged++
		}
	}
	for g, n := range gates {
		g.Count = n
		t.Gates = append(t.Gates, g)
	}
	sort.Slice(t.Gates, func(i, j int) bool {
		if t.Gates[i].Count != t.Gates[j].Count {
			return t.Gates[i].Count > t.Gates[j].Count
		}
		if t.Gates[i].Job != t.Gates[j].Job {
			return t.Gates[i].Job < t.Gates[j].Job
		}
		return t.Gates[i].Gate < t.Gates[j].Gate
	})
	return t
}

// RenderAI prints the with-and-without comparison: the totals, then every
// entry the model improved, and every entry the gates turned back.
func RenderAI(w io.Writer, sc *Scorecard) {
	t := SummarizeAI(sc.Entries)
	fmt.Fprintf(w, "\nwith and without --ai (%s)\n", sc.AIModel)
	fmt.Fprintf(w, "  entries measured %d   improved %d   rewritten, same result %d   unchanged %d   rejected by the checks %d",
		t.Entries-t.Skipped, t.Improved, t.Changed, t.Unchanged, t.Rejected)
	if t.Errors > 0 {
		fmt.Fprintf(w, "   errors %d", t.Errors)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  changes the model proposed and the tool applied %d, turned down %d\n", t.Accepted, t.RejectedChanges)
	fmt.Fprintf(w, "  antipattern hits on the clean renderings %d -> %d (%d entries tidier, %d less tidy)\n",
		t.HitsBefore, t.HitsAfter, t.Tidier, t.Untidier)
	if t.Skipped > 0 {
		fmt.Fprintf(w, "  not measured %d (tier 4 is judged on the report, not on behaviour)\n", t.Skipped)
	}
	fmt.Fprintf(w, "  model time %s in total, %s per measured entry\n",
		t.Elapsed.Round(time.Second), meanDuration(t.Elapsed, t.Entries-t.Skipped))

	section := func(outcome, heading string, describe func(*AIEntry) string) {
		var lines []string
		for _, e := range sc.Entries {
			if e.AI == nil || e.AI.Outcome != outcome {
				continue
			}
			lines = append(lines, fmt.Sprintf("    %-40s %s", e.ID(), describe(e.AI)))
		}
		if len(lines) == 0 {
			return
		}
		fmt.Fprintf(w, "  %s:\n", heading)
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
	}
	reason := func(a *AIEntry) string { return a.Reason }
	hits := func(a *AIEntry) string { return fmt.Sprintf("antipattern hits %d -> %d", a.HitsBefore, a.HitsAfter) }
	section("improved", "improved", reason)
	section("changed", "rewritten, same behaviour", hits)
	section("rejected", "rejected, deterministic output kept", reason)
	section("error", "did not finish", reason)

	if len(t.Gates) > 0 {
		fmt.Fprintln(w, "  proposals turned down, by job and gate:")
		for _, g := range t.Gates {
			fmt.Fprintf(w, "    %-30s %d\n", g.Job+"/"+g.Gate, g.Count)
		}
	}
}

func meanDuration(total time.Duration, n int) time.Duration {
	if n <= 0 {
		return 0
	}
	return (total / time.Duration(n)).Round(100 * time.Millisecond)
}
