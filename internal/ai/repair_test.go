package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"perl2golang/internal/report"
)

// gappedGo is a converted-looking file with one refusal standing in it, the
// way the converter really emits one: the stand-in call, and the TODO comment
// above its first occurrence.
const gappedGo = `package main

import "fmt"

func main() {
	fmt.Print("start\n")
	// TODO: this capture is read where no match is in scope
	fmt.Printf("outside: %s\n", toText(notImplemented[any]("P2G4110", "this capture is read where no match is in scope")))
	fmt.Print("done\n")
}
`

// gappedAnnotated is the same program in the annotated rendering: same code,
// explanation between the statements.
const gappedAnnotated = `package main

import "fmt"

func main() {
	// Perl: print "start\n"
	fmt.Print("start\n")
	// Perl: print "outside: $1\n"
	// TODO: P2G4110: Perl leaves the captures in globals that outlive the
	// match. Keep what the match found in a variable of your own.
	//   print "outside: $1\n";
	fmt.Printf("outside: %s\n", toText(notImplemented[any]("P2G4110", "this capture is read where no match is in scope")))
	// Perl: print "done\n"
	fmt.Print("done\n")
}
`

// gapDeviation is the report entry behind the stand-in above.
var gapDeviation = Deviation{
	Code:    "P2G4110",
	Kind:    "refused",
	Message: "Perl leaves the captures in globals that outlive the match.",
	Advice:  "Keep what the match found in a variable of your own.",
	Line:    16,
}

// fixesAnswer renders a model answer for the repair job.
func fixesAnswer(t *testing.T, fixes ...Fix) string {
	t.Helper()
	for i := range fixes {
		if fixes[i].Imports == nil {
			fixes[i].Imports = []string{}
		}
	}
	buf, err := json.Marshal(fixesPayload{Fixes: fixes})
	if err != nil {
		t.Fatal(err)
	}
	return string(buf)
}

const gapOldCode = `fmt.Printf("outside: %s\n", toText(notImplemented[any]("P2G4110", "this capture is read where no match is in scope")))`

func TestRepairClosesAGapInBothRenderings(t *testing.T) {
	m := newMockRuntime(t, fixesAnswer(t, Fix{
		OldCode: gapOldCode,
		NewCode: `fmt.Printf("outside: %s\n", "ada")`,
		Why:     "the Perl prints the capture that survived the block",
	}))
	c := testClient(t, m, DefaultJobs())

	res, err := c.RepairCode(context.Background(), RepairRequest{
		Path:        "main.go",
		Source:      gappedGo,
		Counterpart: gappedAnnotated,
		PerlSource:  "print \"outside: $1\\n\";\n",
		Deviations:  []Deviation{gapDeviation},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatalf("nothing changed; rejected: %v", res.Rejected)
	}
	if res.GapsClosed != 1 {
		t.Errorf("GapsClosed = %d, want 1", res.GapsClosed)
	}
	for name, src := range map[string]string{"clean": res.Source, "annotated": res.Counterpart} {
		if strings.Contains(src, "notImplemented") {
			t.Errorf("the %s rendering still has the stand-in:\n%s", name, src)
		}
		if !strings.Contains(src, `"ada"`) {
			t.Errorf("the %s rendering is missing the fix:\n%s", name, src)
		}
		if strings.Contains(src, "TODO") {
			t.Errorf("the %s rendering keeps a TODO above working code:\n%s", name, src)
		}
		if !strings.Contains(src, "written by a local model") {
			t.Errorf("the %s rendering does not mark the model's code:\n%s", name, src)
		}
	}
	// The annotated rendering keeps its explanation, minus the stale TODO.
	if !strings.Contains(res.Counterpart, `// Perl: print "outside: $1\n"`) {
		t.Errorf("the annotated rendering lost its explanation:\n%s", res.Counterpart)
	}
	if !containsAll(m.lastPrompt(), "P2G4110", "Keep what the match found", "$1") {
		t.Fatalf("the prompt is missing the Perl or the converter's notes:\n%s", m.lastPrompt())
	}
}

func TestRepairAddsAStdlibImport(t *testing.T) {
	m := newMockRuntime(t, fixesAnswer(t, Fix{
		OldCode: gapOldCode,
		NewCode: `fmt.Printf("outside: %s\n", strings.ToUpper("ada"))`,
		Imports: []string{"strings"},
		Why:     "the Perl upper-cases the capture",
	}))
	c := testClient(t, m, DefaultJobs())

	res, err := c.RepairCode(context.Background(), RepairRequest{
		Path:       "main.go",
		Source:     gappedGo,
		Deviations: []Deviation{gapDeviation},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatalf("nothing changed; rejected: %v", res.Rejected)
	}
	if !containsAll(res.Source, `"strings"`, "strings.ToUpper") {
		t.Fatalf("the needed import was not added:\n%s", res.Source)
	}
	if err := VerifyGo(res.Source); err != nil {
		t.Fatalf("the result does not parse: %v", err)
	}
}

func TestRepairRejects(t *testing.T) {
	tests := []struct {
		name     string
		fix      Fix
		wantGate string
	}{
		{
			name:     "code that is not in the file",
			fix:      Fix{OldCode: `fmt.Print("never there\n")`, NewCode: `fmt.Print("x\n")`},
			wantGate: "grounding",
		},
		{
			name:     "an import that is not the standard library",
			fix:      Fix{OldCode: gapOldCode, NewCode: `fmt.Println(pcre.Match("x"))`, Imports: []string{"github.com/some/pcre"}},
			wantGate: "imports",
		},
		{
			name:     "a new way to end the program",
			fix:      Fix{OldCode: gapOldCode, NewCode: "os.Exit(1)", Imports: []string{"os"}},
			wantGate: "control flow",
		},
		{
			name:     "a dropped declaration",
			fix:      Fix{OldCode: "func main() {\n\tfmt.Print(\"start\\n\")", NewCode: "func other() {\n\tfmt.Print(\"start\\n\")"},
			wantGate: "declarations",
		},
		{
			name:     "a splice that stops the file parsing",
			fix:      Fix{OldCode: gapOldCode, NewCode: "if {"},
			wantGate: "parse",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockRuntime(t, fixesAnswer(t, tt.fix))
			c := testClient(t, m, DefaultJobs())
			res, err := c.RepairCode(context.Background(), RepairRequest{
				Path:       "main.go",
				Source:     gappedGo,
				Deviations: []Deviation{gapDeviation},
			})
			if err != nil {
				t.Fatal(err)
			}
			if res.Changed || res.Source != gappedGo {
				t.Fatalf("a fix that should have been rejected was applied:\n%s", res.Source)
			}
			if len(res.Rejected) != 1 {
				t.Fatalf("rejected = %v, want exactly one", res.Rejected)
			}
			if got := res.Rejected[0].Gate; got != tt.wantGate {
				t.Errorf("gate = %q, want %q (%s)", got, tt.wantGate, res.Rejected[0].Reason)
			}
		})
	}
}

// A fix that fits the clean rendering but has no single place in the annotated
// one is dropped from both, because the renderings must keep behaving
// identically.
func TestRepairKeepsRenderingsInStep(t *testing.T) {
	m := newMockRuntime(t, fixesAnswer(t, Fix{
		OldCode: gapOldCode,
		NewCode: `fmt.Printf("outside: %s\n", "ada")`,
	}))
	c := testClient(t, m, DefaultJobs())

	res, err := c.RepairCode(context.Background(), RepairRequest{
		Path:        "main.go",
		Source:      gappedGo,
		Counterpart: "package main\n\nfunc main() {}\n", // the quote is nowhere in it
		Deviations:  []Deviation{gapDeviation},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("a fix with no home in the annotated rendering was applied anyway")
	}
	if len(res.Rejected) != 1 || res.Rejected[0].Gate != "counterpart" {
		t.Fatalf("rejected = %v, want one counterpart rejection", res.Rejected)
	}
}

// With nothing in the report to fix, the job costs no call at all.
func TestRepairSkipsCleanConversions(t *testing.T) {
	m := newMockRuntime(t, fixesAnswer(t))
	c := testClient(t, m, DefaultJobs())

	res, err := c.RepairCode(context.Background(), RepairRequest{Path: "main.go", Source: gappedGo})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("a file with no reported deviations was changed")
	}
	if m.callCount() != 0 {
		t.Fatalf("%d calls for a file with nothing to fix, want 0", m.callCount())
	}
}

// The improver runs the repair once per program, applies it to both
// renderings, and holds it to the compile gate against the whole package.
func TestImproverRepairPass(t *testing.T) {
	m := newMockRuntime(t, fixesAnswer(t, Fix{
		OldCode: gapOldCode,
		NewCode: `fmt.Printf("outside: %s\n", "ada")`,
		Why:     "prints the surviving capture",
	}))
	c := testClient(t, m, JobSet{JobRepair})
	im := NewImprover(c)

	helpers := map[string][]byte{"helpers.go": []byte(`package main

import "fmt"
import "os"

func toText(v any) string { return fmt.Sprint(v) }

func notImplemented[T any](code, what string) T {
	fmt.Fprintln(os.Stderr, "TODO "+code+": "+what)
	var zero T
	return zero
}
`)}

	rep := &report.Report{}
	rep.Add(report.Entry{
		Code:     "P2G4110",
		Severity: report.Refuse,
		Message:  gapDeviation.Message,
		Advice:   gapDeviation.Advice,
	})

	clean := artifact("main.go", gappedGo, helpers)
	clean.Report = rep
	clean.Counterpart = []byte(gappedAnnotated)
	got, err := im.Improve(context.Background(), clean)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "notImplemented") {
		t.Fatalf("the gap is still open:\n%s", got)
	}

	annotated := artifact("annotated/main.go", gappedAnnotated, helpers)
	annotated.Report = rep
	annotated.Counterpart = got
	gotAnnotated, err := im.Improve(context.Background(), annotated)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gotAnnotated), "notImplemented") {
		t.Fatalf("the annotated rendering kept the gap:\n%s", gotAnnotated)
	}
	if got := m.callCount(); got != 1 {
		t.Errorf("%d calls for two renderings of one program, want 1", got)
	}

	var closed, honest bool
	for _, e := range rep.Entries {
		if e.Code == "P2G9040" {
			closed = true
			if !strings.Contains(e.Message, "prints the surviving capture") {
				t.Errorf("the report entry does not carry the model's reason: %s", e.Message)
			}
		}
	}
	for _, e := range rep.Entries {
		if e.Severity == report.Refuse {
			honest = true
		}
	}
	if !closed {
		t.Error("the report does not say the model closed a gap")
	}
	if !honest {
		t.Error("the original refusal was removed from the report")
	}
	if got := c.Summary().Accepted; got != 1 {
		t.Errorf("accepted = %d, want the one fix counted once", got)
	}
}

// A repair that does not compile against its package is rolled back whole.
func TestImproverRepairCompileGate(t *testing.T) {
	m := newMockRuntime(t, fixesAnswer(t, Fix{
		OldCode: gapOldCode,
		NewCode: `fmt.Printf("outside: %s\n", missingHelper("ada"))`,
		Why:     "calls a helper that does not exist",
	}))
	c := testClient(t, m, JobSet{JobRepair})
	im := NewImprover(c)

	helpers := map[string][]byte{"helpers.go": []byte(`package main

import "fmt"
import "os"

func toText(v any) string { return fmt.Sprint(v) }

func notImplemented[T any](code, what string) T {
	fmt.Fprintln(os.Stderr, "TODO "+code+": "+what)
	var zero T
	return zero
}
`)}

	rep := &report.Report{}
	rep.Add(report.Entry{Code: "P2G4110", Severity: report.Refuse, Message: gapDeviation.Message})

	clean := artifact("main.go", gappedGo, helpers)
	clean.Report = rep
	clean.Counterpart = []byte(gappedAnnotated)
	got, err := im.Improve(context.Background(), clean)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != gappedGo {
		t.Fatalf("a repair that does not compile was kept:\n%s", got)
	}
	if c.Summary().Accepted != 0 {
		t.Errorf("accepted = %d for a rolled-back repair", c.Summary().Accepted)
	}
	if c.Summary().Rejected == 0 {
		t.Error("nothing was recorded about the rejection")
	}
}

// A TODO whose gap is still open keeps its comment, even when the open call
// sits a few lines below the statement the comment introduces, and even when
// another gap with the same code and message was closed.
func TestStaleTodoRemovalKeepsLiveGaps(t *testing.T) {
	src := `package main

import "fmt"

func main() {
	// TODO: P2G4110: the first gap, now closed.
	fmt.Println("fixed")
	// TODO: P2G4110: the second gap, still open, inside the if below.
	if len("x") > 0 {
		fmt.Println(notImplemented[any]("P2G4110", "this capture is read where no match is in scope"))
	}
}
`
	gap := gapCall{code: "P2G4110", message: "this capture is read where no match is in scope"}
	got := removeStaleTodos(src, []gapCall{gap, gap}, []gapCall{gap})
	if strings.Contains(got, "the first gap") {
		t.Errorf("the closed gap's TODO survived:\n%s", got)
	}
	if !strings.Contains(got, "the second gap") {
		t.Errorf("the open gap lost its TODO:\n%s", got)
	}
	if strings.Count(got, "written by a local model") != 1 {
		t.Errorf("the closed gap should carry exactly one model marker:\n%s", got)
	}
}

// gapCalls has to see through both spellings of the stand-in.
func TestGapCalls(t *testing.T) {
	src := `package main

func run() {
	notImplementedHere("P2G0001", "one thing")
	x := notImplemented[any]("P2G0002", "another thing")
	_ = x
}
`
	got := gapCalls(src)
	if len(got) != 2 {
		t.Fatalf("gapCalls found %d, want 2: %v", len(got), got)
	}
	if got[0].code != "P2G0001" || got[1].message != "another thing" {
		t.Fatalf("gapCalls misread the calls: %v", got)
	}
}

// The compile gate is measured against the deterministic version, but the
// excuse only reaches as far as the baseline's own defect. A baseline that
// builds and merely fails go vet must not excuse a candidate that does not
// build at all: exactly that combination let a repair with redeclared
// variables reach the output on a corpus entry whose deterministic file had
// one unreachable statement.
func TestCompileGateExcuseStopsAtTheBaselineDefect(t *testing.T) {
	m := newMockRuntime(t, fixesAnswer(t, Fix{
		OldCode: `fmt.Printf("outside: %s\n", toText(notImplemented[any]("P2G4110", "gap")))`,
		NewCode: "count := 1\n\tcount := 2\n\tfmt.Printf(\"outside: %d\\n\", count)",
		Why:     "redeclares a variable, which only the compiler notices",
	}))
	c := testClient(t, m, JobSet{JobRepair})
	im := NewImprover(c)

	helpers := map[string][]byte{"helpers.go": []byte(`package main

import "fmt"
import "os"

func toText(v any) string { return fmt.Sprint(v) }

func notImplemented[T any](code, what string) T {
	fmt.Fprintln(os.Stderr, "TODO "+code+": "+what)
	var zero T
	return zero
}
`)}

	// Builds clean; go vet flags the line after the return as unreachable.
	vetGappedGo := `package main

import "fmt"

func main() {
	fmt.Printf("outside: %s\n", toText(notImplemented[any]("P2G4110", "gap")))
	return
	fmt.Print("done\n")
}
`
	rep := &report.Report{}
	rep.Add(report.Entry{Code: "P2G4110", Severity: report.Refuse, Message: gapDeviation.Message})
	clean := artifact("main.go", vetGappedGo, helpers)
	clean.Report = rep

	got, _ := im.Improve(context.Background(), clean)
	if string(got) != vetGappedGo {
		t.Fatalf("a repair that does not build was kept because the baseline fails vet:\n%s", got)
	}
	if c.Summary().Accepted != 0 {
		t.Errorf("accepted = %d, want 0", c.Summary().Accepted)
	}
}
