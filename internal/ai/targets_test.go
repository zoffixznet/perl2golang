package ai

import (
	"slices"
	"strings"
	"testing"
)

func TestWeakName(t *testing.T) {
	tests := []struct {
		name string
		weak bool
		why  string
	}{
		{"c", true, "a single letter says nothing"},
		{"w2", true, "numbered and short"},
		{"item4", true, "a generator numbered it"},
		{"pattern22", true, "same"},
		{"tmp", true, "a vacuum word"},
		{"byKey", true, "describes the shape, not the contents"},
		{"err", false, "Go's own convention"},
		{"ok", false, "same"},
		{"_", false, "not a name at all"},
		{"wordCount", false, "already says what it holds"},
		{"scanner", false, "already says what it holds"},
		{"stop", false, "short but meaningful"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := weakName(tt.name); got != tt.weak {
				t.Errorf("weakName(%q) = %v, want %v: %s", tt.name, got, tt.weak, tt.why)
			}
		})
	}
}

func TestScanTargetsPicksGeneratedNames(t *testing.T) {
	tg, err := scanTargets("main.go", sampleGo, DefaultJobs())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range tg.renames {
		names = append(names, r.Name)
	}
	for _, want := range []string{"byKey", "item4", "c", "v"} {
		if !slices.Contains(names, want) {
			t.Errorf("%q was not offered for renaming; offered %v", want, names)
		}
	}
	for _, r := range tg.renames {
		if len(r.Usage) == 0 {
			t.Errorf("%q was offered with no use sites, so nothing could name it", r.Name)
		}
	}
}

// A loop index is the one short name Go actually wants, so the scan has to
// leave it alone.
func TestScanTargetsLeavesLoopIndices(t *testing.T) {
	src := `package main

func main() {
	xs := []int{1, 2, 3}
	for i := 0; i < len(xs); i++ {
		xs[i]++
	}
	for j := range xs {
		xs[j]--
	}
}
`
	tg, err := scanTargets("main.go", src, DefaultJobs())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range tg.renames {
		if r.Name == "i" || r.Name == "j" {
			t.Errorf("the loop index %q was offered for renaming", r.Name)
		}
	}
}

// A name declared twice cannot be renamed by a whole-file substitution, so it
// is never offered in the first place.
func TestScanTargetsSkipsShadowedNames(t *testing.T) {
	src := `package main

func a() { x := 1; _ = x }
func b() { x := 2; _ = x }
`
	tg, err := scanTargets("main.go", src, DefaultJobs())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range tg.renames {
		if r.Name == "x" {
			t.Fatal("a name declared in two scopes was offered for renaming")
		}
	}
}

func TestCheckNewName(t *testing.T) {
	tg, err := scanTargets("main.go", sampleGo, DefaultJobs())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		old, replacement string
		exported         bool
		wantGate         string
	}{
		{"c", "wordCount", false, ""},
		{"c", "range", false, "naming"},
		{"c", "len", false, "naming"},
		{"c", "strings", false, "naming"},
		{"c", "word_count", false, "naming"},
		{"c", "WordCount", false, "naming"},
		{"c", "byKey", false, "naming"},
		{"c", "c", false, "naming"},
		{"c", "aNameSoLongThatNobodyCouldPossiblyReadItInOneGo", false, "naming"},
	}
	for _, tt := range tests {
		t.Run(tt.old+"->"+tt.replacement, func(t *testing.T) {
			err := tg.checkNewName(tt.old, tt.replacement, tt.exported)
			switch {
			case tt.wantGate == "" && err != nil:
				t.Fatalf("rejected a good name: %v", err)
			case tt.wantGate != "" && err == nil:
				t.Fatal("accepted a name that breaks the rules")
			case tt.wantGate != "":
				gate, _ := gateOf(err)
				if gate != tt.wantGate {
					t.Fatalf("gate = %q, want %q", gate, tt.wantGate)
				}
			}
		})
	}
}

func TestCheckDocComment(t *testing.T) {
	tests := []struct {
		name, comment string
		wantGate      string
	}{
		{"Load", "Load reads the table from r.", ""},
		{"Load", "Loads the table from r.", "convention"},
		{"Load", "", "shape"},
		{"Load", "Load reads the table. It was converted from a Perl sub.", "provenance"},
		{"Load", "Load is the Go version of the original script's loader.", "provenance"},
		{"Load", "Load // does things", "shape"},
		{"Load", "Load does one. And two. And three. And four.", "bounds"},
		{"Load", "Load " + strings.Repeat("x", maxDocCommentChars), "bounds"},
	}
	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			got, err := checkDocComment(tt.name, tt.comment)
			if tt.wantGate == "" {
				if err != nil {
					t.Fatalf("rejected a good comment: %v", err)
				}
				if !strings.HasPrefix(got, tt.name) {
					t.Fatalf("comment = %q, which does not start with the name", got)
				}
				return
			}
			if err == nil {
				t.Fatalf("accepted %q", tt.comment)
			}
			if gate, _ := gateOf(err); gate != tt.wantGate {
				t.Fatalf("gate = %q, want %q (%v)", gate, tt.wantGate, err)
			}
		})
	}
}

// A doc comment with no full stop is finished rather than refused, because the
// missing stop is the one defect that is safe to fix without guessing.
func TestCheckDocCommentTerminates(t *testing.T) {
	got, err := checkDocComment("Load", "Load reads the table from r")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, ".") {
		t.Fatalf("comment = %q, want it to end with a full stop", got)
	}
}
