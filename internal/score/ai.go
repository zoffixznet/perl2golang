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
	"slices"
	"time"

	"perl2golang/internal/convert"
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
	// Stats, when set, is read after the entry finishes: how many changes the
	// pass applied and how many its checks turned down.
	Stats func() (accepted, rejected int)
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
	if session.Stats != nil {
		out.Accepted, out.Rejected = session.Stats()
	}
	if cerr != nil {
		out.Outcome, out.Reason = "error", firstLine(cerr.Error())
		return finish()
	}

	out.Changed = filesDiffer(det, conv)
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
	Elapsed                                                          time.Duration
}

// SummarizeAI adds up what the comparison found. Entries without an AI result
// (tier 4's honest-failure entries, and any entry whose deterministic
// conversion already failed) count as skipped.
func SummarizeAI(entries []EntryResult) AITotals {
	var t AITotals
	for _, e := range entries {
		t.Entries++
		if e.AI == nil {
			t.Skipped++
			continue
		}
		t.Accepted += e.AI.Accepted
		t.RejectedChanges += e.AI.Rejected
		t.Elapsed += e.AI.Elapsed
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
	if t.Skipped > 0 {
		fmt.Fprintf(w, "  not measured %d (tier 4 is judged on the report, not on behaviour)\n", t.Skipped)
	}
	fmt.Fprintf(w, "  model time %s in total, %s per measured entry\n",
		t.Elapsed.Round(time.Second), meanDuration(t.Elapsed, t.Entries-t.Skipped))

	section := func(outcome, heading string) {
		var lines []string
		for _, e := range sc.Entries {
			if e.AI == nil || e.AI.Outcome != outcome {
				continue
			}
			lines = append(lines, fmt.Sprintf("    %-40s %s", e.ID(), e.AI.Reason))
		}
		if len(lines) == 0 {
			return
		}
		fmt.Fprintf(w, "  %s:\n", heading)
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
	}
	section("improved", "improved")
	section("rejected", "rejected, deterministic output kept")
	section("error", "did not finish")
}

func meanDuration(total time.Duration, n int) time.Duration {
	if n <= 0 {
		return 0
	}
	return (total / time.Duration(n)).Round(100 * time.Millisecond)
}
