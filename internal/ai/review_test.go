package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"perl2golang/internal/report"
)

// These tests pin down what the idiom review may and may not do. The job
// exists for one class of fix above all: a hand-rolled loop replaced by the
// standard library call that does the same thing. That fix necessarily adds
// an import and drops the loop's steering numbers, so the gate must allow
// both, while still refusing new string text, new numbers, and anything that
// reshapes the program's declarations or error handling.

const containsLoopGo = `package main

import "fmt"

func main() {
	names := []string{"alpha", "beta"}
	found := false
	for _, n := range names {
		if n == "beta" {
			found = true
		}
	}
	fmt.Println(found)
}
`

func TestReviewAcceptsAStdlibRewriteWithItsImport(t *testing.T) {
	finding := Finding{
		Line: 7, Kind: "stdlib_exists",
		OldCode: "\tfound := false\n\tfor _, n := range names {\n\t\tif n == \"beta\" {\n\t\t\tfound = true\n\t\t}\n\t}",
		NewCode: "\tfound := slices.Contains(names, \"beta\")",
		Imports: []string{"slices"},
		Why:     "a linear search is slices.Contains",
	}
	got, used, err := applyFinding(containsLoopGo, containsLoopGo, finding, nil)
	if err != nil {
		t.Fatalf("the owner's own example, a loop that is a stdlib call, was rejected: %v", err)
	}
	if !strings.Contains(got, `"slices"`) || !strings.Contains(got, "slices.Contains(names, ") {
		t.Fatalf("the import or the call is missing:\n%s", got)
	}
	if len(used) != 1 || used[0] != "slices" {
		t.Fatalf("the accepted finding should report its import, got %v", used)
	}
	if err := VerifyGo(got); err != nil {
		t.Fatalf("the result does not parse: %v", err)
	}
}

func TestReviewAcceptsTheMinBuiltin(t *testing.T) {
	src := `package main

import "fmt"

func main() {
	a, b := 3, 9
	limit := 0
	if a < b {
		limit = a
	} else {
		limit = b
	}
	fmt.Println(limit)
}
`
	finding := Finding{
		Line: 8, Kind: "stdlib_exists",
		OldCode: "\tif a < b {\n\t\tlimit = a\n\t} else {\n\t\tlimit = b\n\t}",
		NewCode: "\tlimit = min(a, b)",
		Why:     "this is the built-in min",
	}
	got, _, err := applyFinding(src, src, finding, nil)
	if err != nil {
		t.Fatalf("a hand-rolled min was rejected: %v", err)
	}
	if !strings.Contains(got, "min(a, b)") {
		t.Fatalf("the rewrite is missing:\n%s", got)
	}
}

func TestReviewMayDropAVerbOnlyFormatString(t *testing.T) {
	src := `package main

import "fmt"

func main() {
	a, b := "x", "y"
	c := fmt.Sprintf("%s%s", a, b)
	fmt.Println(c)
}
`
	finding := Finding{
		Line: 7, Kind: "sprintf_concat",
		OldCode: `c := fmt.Sprintf("%s%s", a, b)`,
		NewCode: `c := a + b`,
		Why:     "Sprintf of only verbs is concatenation",
	}
	got, _, err := applyFinding(src, src, finding, nil)
	if err != nil {
		t.Fatalf("folding a verb-only Sprintf was rejected: %v", err)
	}
	if !strings.Contains(got, "c := a + b") {
		t.Fatalf("the rewrite is missing:\n%s", got)
	}
}

func TestReviewRejects(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		finding  Finding
		wantGate string
	}{
		{
			name: "a new string literal",
			src:  containsLoopGo,
			finding: Finding{Kind: "other",
				OldCode: `fmt.Println(found)`,
				NewCode: `fmt.Println("found:", found)`,
				Why:     "clearer output"},
			wantGate: "literals",
		},
		{
			name: "a dropped content string",
			src:  containsLoopGo,
			finding: Finding{Kind: "dead_code",
				OldCode: "\tnames := []string{\"alpha\", \"beta\"}",
				NewCode: "\tnames := []string{\"beta\"}",
				Why:     "alpha is never matched"},
			wantGate: "literals",
		},
		{
			name: "a number the file does not contain",
			src:  containsLoopGo,
			finding: Finding{Kind: "other",
				OldCode: `names := []string{"alpha", "beta"}`,
				NewCode: `names := make([]string, 0, 16)`,
				Why:     "preallocate"},
			wantGate: "literals",
		},
		{
			name: "an import outside the standard library",
			src:  containsLoopGo,
			finding: Finding{Kind: "stdlib_exists",
				OldCode: `fmt.Println(found)`,
				NewCode: `pretty.Println(found)`,
				Imports: []string{"github.com/kr/pretty"},
				Why:     "prettier"},
			wantGate: "imports",
		},
		{
			name: "a package that was never imported nor declared",
			src:  containsLoopGo,
			finding: Finding{Kind: "stdlib_exists",
				OldCode: "\tfound := false\n\tfor _, n := range names {\n\t\tif n == \"beta\" {\n\t\t\tfound = true\n\t\t}\n\t}",
				NewCode: "\tfound := slices.Contains(names, \"beta\")",
				Why:     "no imports declared for the package it uses"},
			wantGate: "grounding",
		},
		{
			name: "a deleted error check",
			src: `package main

import "os"

func main() {
	f, err := os.Open("in.txt")
	if err != nil {
		return
	}
	f.Close()
}
`,
			finding: Finding{Kind: "dead_code",
				OldCode: "\tif err != nil {\n\t\treturn\n\t}\n\tf.Close()",
				NewCode: "\tf.Close()",
				Why:     "the file always exists"},
			wantGate: "error handling",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := applyFinding(tt.src, tt.src, tt.finding, nil)
			if err == nil {
				t.Fatal("the finding was accepted")
			}
			re, ok := err.(*RejectedError)
			if !ok {
				t.Fatalf("wanted a rejection, got %T: %v", err, err)
			}
			if re.Gate != tt.wantGate {
				t.Fatalf("gate = %q (%s), want %q", re.Gate, re.Reason, tt.wantGate)
			}
		})
	}
}

func TestReviewCarriesEarlierImportsForward(t *testing.T) {
	// Two findings, each bringing its own import. The second is checked
	// against the deterministic baseline, which has neither, so the check
	// must know what the first already added.
	src := `package main

import "fmt"

func main() {
	names := []string{"alpha", "beta"}
	found := false
	for _, n := range names {
		if n == "beta" {
			found = true
		}
	}
	sorted := false
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			sorted = true
		}
	}
	fmt.Println(found, sorted)
}
`
	first := Finding{Kind: "stdlib_exists",
		OldCode: "\tfound := false\n\tfor _, n := range names {\n\t\tif n == \"beta\" {\n\t\t\tfound = true\n\t\t}\n\t}",
		NewCode: "\tfound := slices.Contains(names, \"beta\")",
		Imports: []string{"slices"},
		Why:     "a linear search is slices.Contains"}
	second := Finding{Kind: "stdlib_exists",
		OldCode: "\tsorted := false\n\tfor i := 1; i < len(names); i++ {\n\t\tif names[i-1] > names[i] {\n\t\t\tsorted = true\n\t\t}\n\t}",
		NewCode: "\tsorted := !slices.IsSorted(names)",
		Imports: []string{"slices"},
		Why:     "a hand-rolled order check is slices.IsSorted"}

	current, used, err := applyFinding(src, src, first, nil)
	if err != nil {
		t.Fatalf("first finding rejected: %v", err)
	}
	current, _, err = applyFinding(current, src, second, used)
	if err != nil {
		t.Fatalf("second finding rejected after the first added its import: %v", err)
	}
	if !strings.Contains(current, "slices.IsSorted(names)") {
		t.Fatalf("the second rewrite is missing:\n%s", current)
	}
}

func TestReviewCodeEndToEnd(t *testing.T) {
	payload := map[string]any{"findings": []any{map[string]any{
		"line": 7, "kind": "stdlib_exists",
		"old_code": "\tfound := false\n\tfor _, n := range names {\n\t\tif n == \"beta\" {\n\t\t\tfound = true\n\t\t}\n\t}",
		"new_code": "\tfound := slices.Contains(names, \"beta\")",
		"imports":  []string{"slices"},
		"why":      "a linear search is slices.Contains",
	}}}
	answer, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	m := newMockRuntime(t, string(answer))
	c := testClient(t, m, JobSet{JobIdiomReview})

	res, err := c.ReviewCode(context.Background(), CodeRequest{
		Path:        "main.go",
		Source:      containsLoopGo,
		ReviewKinds: []string{"stdlib_exists"},
	})
	if err != nil {
		t.Fatalf("ReviewCode: %v", err)
	}
	if !res.Changed || len(res.Applied) != 1 {
		t.Fatalf("the finding was not applied: %+v", res)
	}
	if !strings.Contains(res.Source, `"slices"`) {
		t.Fatalf("the declared import was not added:\n%s", res.Source)
	}
	if !strings.Contains(m.lastPrompt(), "stdlib_exists") {
		t.Fatal("the suspected classes were not put in front of the model")
	}
	if !strings.Contains(m.lastSchema(), `"imports"`) {
		t.Fatal("the schema on the wire does not ask for imports")
	}
}

func TestReviewRollsBackWhenTheWholeFileFailsToCompile(t *testing.T) {
	// The finding parses and passes every structural check, but applying it
	// leaves `found` declared and unused, which only the compiler can see.
	// The whole file is then rolled back, and the summary has to agree with
	// what was written: nothing.
	payload := map[string]any{"findings": []any{map[string]any{
		"line": 12, "kind": "other",
		"old_code": "fmt.Println(found)",
		"new_code": "fmt.Println(names)",
		"imports":  []string{},
		"why":      "print the slice instead",
	}}}
	answer, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	m := newMockRuntime(t, string(answer))
	c := testClient(t, m, JobSet{JobIdiomReview})
	im := NewImprover(c)

	got, _ := im.Improve(context.Background(), artifact("main.go", containsLoopGo, nil))
	if string(got) != containsLoopGo {
		t.Fatalf("a file that does not compile was handed back:\n%s", got)
	}
	s := c.Summary()
	if s.Accepted != 0 {
		t.Errorf("accepted = %d after a whole-file rollback, want 0", s.Accepted)
	}
	if s.Rejected != 1 || len(s.Rejections) != 1 {
		t.Fatalf("rejected = %d with %d recorded rejections, want 1 and 1", s.Rejected, len(s.Rejections))
	}
	if s.Rejections[0].Gate != "compile" {
		t.Errorf("gate = %q, want compile", s.Rejections[0].Gate)
	}
}

func TestReviewKindsComeFromTheDeterministicScan(t *testing.T) {
	src := []byte(`package main

import "fmt"

func main() {
	xs := []int{1, 2}
	sum := 0
	for i := 0; i < len(xs); i++ {
		sum += xs[i]
	}
	total := sum
	fmt.Println(total)
}
`)
	kinds := reviewKinds("main.go", src)
	want := []string{"cstyle_for", "needless_intermediate"}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
}

func TestRepairIsOfferedRefusalsOnly(t *testing.T) {
	// An approximation is documented working code. Measured over the corpus,
	// offering approximations for repair fixed nothing and produced the only
	// silent corruptions, so the work list is refusals alone.
	a := artifact("main.go", "package main\n\nfunc main() {\n\tnotImplemented[string](\"P2G4110\", \"tr///\")\n}\n", nil)
	a.Report.Add(report.Entry{Code: "P2G4110", Severity: report.Refuse, Construct: "tr///"})
	a.Report.Add(report.Entry{Code: "P2G5502", Severity: report.Warn, Construct: "numeric coercion"})

	got := deviationsFor(a)
	if len(got) != 1 {
		t.Fatalf("deviations = %d, want the refusal alone: %+v", len(got), got)
	}
	if got[0].Code != "P2G4110" || got[0].Kind != "refused" {
		t.Fatalf("the one deviation should be the refusal, got %+v", got[0])
	}
}

func TestPackageSkeleton(t *testing.T) {
	pkg := map[string][]byte{
		"helpers.go": []byte("package main\n\nfunc pick(xs []string, idx []int) []string { return nil }\n\nfunc seq(a, b int) []int { return nil }\n"),
		"notes.md":   []byte("not Go"),
	}
	got := packageSkeleton(pkg)
	if !containsAll(got, "func pick(xs []string, idx []int) []string", "func seq(a, b int) []int") {
		t.Fatalf("skeleton is missing signatures:\n%s", got)
	}
}
