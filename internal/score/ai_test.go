package score

import (
	"strings"
	"testing"
)

func TestAIOutcome(t *testing.T) {
	pairs := func(detC, aiC, detE, aiE StageResult) []stagePair {
		return []stagePair{
			{"compiled", detC, aiC},
			{"equivalent", detE, aiE},
			{"equivalent (annotated)", detE, aiE},
		}
	}
	tests := []struct {
		name    string
		changed bool
		stages  []stagePair
		want    string
	}{
		{
			name:    "the model changed nothing",
			changed: false,
			stages:  pairs(pass(), pass(), pass(), pass()),
			want:    "unchanged",
		},
		{
			name:    "a failing entry now matches perl",
			changed: true,
			stages:  pairs(pass(), pass(), fail("stdout differs"), pass()),
			want:    "improved",
		},
		{
			name:    "output that no longer matches perl is rejected outright",
			changed: true,
			stages:  pairs(pass(), pass(), pass(), fail("stdout differs")),
			want:    "rejected",
		},
		{
			name:    "a broken compile is rejected even when equivalence could not run",
			changed: true,
			stages:  pairs(pass(), fail("does not compile"), fail("did not compile"), fail("did not compile")),
			want:    "rejected",
		},
		{
			name:    "a rewrite that fixes one stage and breaks another is rejected",
			changed: true,
			stages: []stagePair{
				{"compiled", fail("x"), pass()},
				{"equivalent", pass(), fail("stdout differs")},
			},
			want: "rejected",
		},
		{
			name:    "a rewrite that changes nothing measurable",
			changed: true,
			stages:  pairs(pass(), pass(), pass(), pass()),
			want:    "changed",
		},
		{
			name:    "a skip turning into a pass proves nothing",
			changed: true,
			stages:  pairs(pass(), pass(), skip("short mode"), pass()),
			want:    "changed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := aiOutcome(tt.changed, tt.stages)
			if got != tt.want {
				t.Fatalf("outcome = %q (%s), want %q", got, reason, tt.want)
			}
			if got == "rejected" && !strings.Contains(reason, "pass ->") {
				t.Errorf("a rejection should name the stage that got worse, got %q", reason)
			}
		})
	}
}

func TestSummarizeAI(t *testing.T) {
	entries := []EntryResult{
		{AI: &AIEntry{Outcome: "improved", Accepted: 2}},
		{AI: &AIEntry{Outcome: "rejected", Rejected: 1}},
		{AI: &AIEntry{Outcome: "unchanged"}},
		{AI: &AIEntry{Outcome: "changed", Accepted: 1}},
		{}, // an entry the comparison skipped
	}
	got := SummarizeAI(entries)
	want := AITotals{Entries: 5, Improved: 1, Changed: 1, Unchanged: 1, Rejected: 1,
		Skipped: 1, Accepted: 3, RejectedChanges: 1}
	if got != want {
		t.Fatalf("SummarizeAI = %+v, want %+v", got, want)
	}
}
