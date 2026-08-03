package ai

import (
	"context"
	"strings"
	"testing"

	"perl2golang/internal/convert"
	"perl2golang/internal/report"
)

// artifact builds one artefact the way the converter offers them.
func artifact(name, content string, pkg map[string][]byte) convert.Artifact {
	return convert.Artifact{
		Kind:    convert.ArtifactGo,
		Name:    name,
		Content: []byte(content),
		Perl:    []byte("my %freq; $freq{$_}++ for @words;\n"),
		Report:  &report.Report{},
		Package: pkg,
		Module:  "wordfreq",
	}
}

// The property that keeps the two programs readable side by side: one call,
// one set of names, applied to both renderings.
func TestImproverGivesBothRenderingsTheSameNames(t *testing.T) {
	m := newMockRuntime(t, goodAnswer)
	c := testClient(t, m, DefaultJobs())
	im := NewImprover(c)

	clean, err := im.Improve(context.Background(), artifact("main.go", sampleGo, nil))
	if err != nil {
		t.Fatal(err)
	}
	annotated, err := im.Improve(context.Background(), artifact("annotated/main.go", annotatedSample, nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"wordCount", "word", "counts"} {
		if !strings.Contains(string(clean), name) {
			t.Errorf("%q is missing from the clean program", name)
		}
		if !strings.Contains(string(annotated), name) {
			t.Errorf("%q is missing from the annotated program", name)
		}
	}
	if got := m.callCount(); got != 1 {
		t.Errorf("%d calls for two renderings of one program, want 1", got)
	}
	if !strings.Contains(string(annotated), "// Perl: my %freq") {
		t.Error("the annotated program lost its explanation")
	}
	// Both renderings were written, and the numbers count the decisions once.
	if got := c.Summary().Accepted; got != 3 {
		t.Errorf("accepted = %d, want 3 counted once", got)
	}
}

// The runtime support file is hand-written code the converter copies in. There
// is nothing to improve in it and every reason not to try.
func TestImproverLeavesHelpersAlone(t *testing.T) {
	m := newMockRuntime(t, goodAnswer)
	im := NewImprover(testClient(t, m, DefaultJobs()))

	helpers := `package main

func toNum(v any) float64 {
	c, ok := v.(float64)
	if !ok {
		return 0
	}
	return c
}
`
	got, err := im.Improve(context.Background(), artifact("helpers.go", helpers, nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != helpers {
		t.Error("the runtime support file was rewritten")
	}
	if m.callCount() != 0 {
		t.Errorf("%d calls for the support file, want 0", m.callCount())
	}
}

// A document is not this pass's business.
func TestImproverIgnoresDocuments(t *testing.T) {
	m := newMockRuntime(t, goodAnswer)
	im := NewImprover(testClient(t, m, DefaultJobs()))

	a := convert.Artifact{Kind: convert.ArtifactMarkdown, Name: "docs/start-here.md", Content: []byte("# Start here\n")}
	got, err := im.Improve(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# Start here\n" {
		t.Error("a document was rewritten by the naming pass")
	}
	if m.callCount() != 0 {
		t.Errorf("%d calls for a document, want 0", m.callCount())
	}
}

// A dead runtime costs one attempt per program, not one per rendering, and it
// leaves a note that says what happened in the user's terms.
func TestImproverDegradesOnce(t *testing.T) {
	m := newMockRuntime(t, goodAnswer)
	url := m.URL
	m.Close()
	t.Setenv("OLLAMA_HOST", "")
	c := New(Options{Endpoint: url, Model: "qwen2.5-coder:7b"})
	im := NewImprover(c)

	a := artifact("main.go", sampleGo, nil)
	if _, err := im.Improve(context.Background(), a); err == nil {
		t.Fatal("a dead runtime produced no error")
	}
	b := artifact("annotated/main.go", sampleGo, nil)
	b.Report = a.Report
	got, err := im.Improve(context.Background(), b)
	if err != nil {
		t.Fatalf("the second rendering tried again and failed again: %v", err)
	}
	if string(got) != sampleGo {
		t.Error("a failed run changed the file anyway")
	}
	if len(a.Report.Entries) == 0 {
		t.Fatal("nothing was recorded about a runtime that was not there")
	}
	if c.Summary().Skipped == "" {
		t.Error("the summary does not say why nothing happened")
	}
}

// The gate has to be real. A type is package scope, so renaming one can break
// a file the naming pass never saw, and nothing short of compiling the whole
// package can notice.
func TestImproverCompileGateCatchesACrossFileBreak(t *testing.T) {
	m := newMockRuntime(t, `{"renames":[],"types":[{"name":"row","new_name":"Reading","fields":[]}],"comments":[]}`)
	im := NewImprover(testClient(t, m, DefaultJobs()))

	main := `package main

import "fmt"

type row struct {
	Field1 int
}

func main() {
	fmt.Println(describe(row{Field1: 1}))
}
`
	helpers := map[string][]byte{"helpers.go": []byte(`package main

import "strconv"

func describe(r row) string { return strconv.Itoa(r.Field1) }
`)}
	a := artifact("main.go", main, helpers)
	got, err := im.Improve(context.Background(), a)
	if err == nil {
		t.Fatal("a type rename that breaks another file in the package was accepted")
	}
	if string(got) != main {
		t.Error("the deterministic file was not kept when the gate failed")
	}
	if gate, _ := gateOf(err); gate != "compile" {
		t.Errorf("gate = %q, want compile", gate)
	}
	if got := im.client.Summary().Accepted; got != 0 {
		t.Errorf("accepted = %d for a rewrite that was thrown away", got)
	}
}

// The other half of the same claim: a rename that keeps the package building
// is kept, so the gate is not simply refusing everything.
func TestImproverCompileGateKeepsGoodWork(t *testing.T) {
	m := newMockRuntime(t, `{"renames":[{"old":"item4","new":"word"}],"comments":[]}`)
	im := NewImprover(testClient(t, m, DefaultJobs()))

	main := `package main

import (
	"fmt"
	"strings"
)

func main() {
	for _, item4 := range strings.Fields("a b c") {
		fmt.Println(shout(item4))
	}
}
`
	helpers := map[string][]byte{"helpers.go": []byte(`package main

import "strings"

func shout(s string) string { return strings.ToUpper(s) }
`)}
	got, err := im.Improve(context.Background(), artifact("main.go", main, helpers))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "shout(word)") {
		t.Fatalf("a sound rename was refused:\n%s", got)
	}
}

func TestVerifyPackage(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantGate string
	}{
		{
			name: "a package that builds",
			src: `package main

import "fmt"

func main() { fmt.Println("hello") }
`,
		},
		{
			name: "a package that does not build",
			src: `package main

func main() { undefinedCall() }
`,
			wantGate: "compile",
		},
		{
			name: "a package vet objects to",
			src: `package main

import "fmt"

func main() { fmt.Printf("%d\n", "not a number") }
`,
			wantGate: "vet",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPackage(context.Background(), map[string][]byte{"main.go": []byte(tt.src)})
			if tt.wantGate == "" {
				if err != nil {
					t.Fatalf("a sound package was rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("a package that should not pass did")
			}
			if gate, _ := gateOf(err); gate != tt.wantGate {
				t.Fatalf("gate = %q, want %q (%v)", gate, tt.wantGate, err)
			}
		})
	}
}

// A package the converter itself could not compile still gets its names, on
// the strength of the structural checks, and the report says the toolchain
// could not confirm them.
func TestImproverAcceptsWhenTheBaselineIsAlreadyBroken(t *testing.T) {
	m := newMockRuntime(t, `{"renames":[{"old":"c","new":"total"}],"comments":[]}`)
	im := NewImprover(testClient(t, m, DefaultJobs()))

	broken := `package main

import "fmt"

func main() {
	c := 1
	fmt.Println(c + "not a number")
}
`
	a := artifact("main.go", broken, nil)
	got, err := im.Improve(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "total := 1") {
		t.Fatalf("the name was refused on a file nothing could have compiled anyway:\n%s", got)
	}
	var said bool
	for _, entry := range a.Report.Entries {
		if strings.Contains(entry.Message, "could not be used to check") {
			said = true
		}
	}
	if !said {
		t.Error("the report does not say the toolchain could not judge the names")
	}
}

// annotatedSample is the same program as sampleGo in the annotated rendering:
// same code, buried in the explanation.
const annotatedSample = `package main

import (
	"fmt"
	"strings"
)

func main() {
	// Perl: my %freq
	// A Go map has to be made before anything can be written to it.
	byKey := map[string]int{}
	// Perl: $freq{$_}++ for @words
	for _, item4 := range strings.Fields("a b c") {
		byKey[item4]++
	}
	var c int
	for _, v := range byKey {
		c += v
	}
	fmt.Printf("%d words\n", c)
}
`

// The prose job is off unless it is asked for by name, and it touches exactly
// one document when it is.
func TestImproverProseJobIsOptIn(t *testing.T) {
	m := newMockRuntime(t, "# The walkthrough\n\nSomething about the code that was shown.\n")
	doc := convert.Artifact{
		Kind:    convert.ArtifactMarkdown,
		Name:    "docs/walkthrough.md",
		Content: []byte("# The walkthrough\n\nThe deterministic tour of this file.\n"),
		Perl:    []byte("print \"hi\\n\";\n"),
		Report:  &report.Report{},
	}

	off := NewImprover(testClient(t, m, DefaultJobs()))
	got, err := off.Improve(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(doc.Content) {
		t.Error("the walkthrough was rewritten without the job being asked for")
	}
	if m.callCount() != 0 {
		t.Errorf("%d calls for a document with the prose job off, want 0", m.callCount())
	}

	// Even asked for, it stays away from the curated lessons and the report.
	on := NewImprover(testClient(t, m, JobSet{JobWalkthrough}))
	for _, name := range []string{"docs/concepts/map-iteration-order.md", "docs/conversion-report.md", "docs/start-here.md"} {
		other := doc
		other.Name = name
		if _, err := on.Improve(context.Background(), other); err != nil {
			t.Fatal(err)
		}
	}
	if m.callCount() != 0 {
		t.Errorf("%d calls for the curated documents, want 0", m.callCount())
	}
}
