package ai

import (
	"strings"
	"testing"
)

func TestApplyRenamesLocals(t *testing.T) {
	got, err := Apply("main.go", sampleGo, Decisions{Renames: []Rename{
		{Old: "c", New: "wordCount"},
		{Old: "item4", New: "word"},
		{Old: "byKey", New: "counts"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(got, "wordCount", "word", "counts") {
		t.Fatalf("the new names are not in the result:\n%s", got)
	}
	for _, old := range []string{"item4", "byKey"} {
		if strings.Contains(got, old) {
			t.Errorf("%q survived the rename:\n%s", old, got)
		}
	}
	if err := VerifyGo(got); err != nil {
		t.Fatalf("the result does not parse: %v\n%s", err, got)
	}
	if err := checkRenamed(sampleGo, got); err != nil {
		t.Fatalf("the result is not the same program: %v", err)
	}
}

// The one rewrite that must never happen: a rename reaching into a package
// qualifier and turning strings.Fields into something else.
func TestApplyLeavesPackageMembersAlone(t *testing.T) {
	src := `package main

import "strings"

type row struct {
	Fields int
}

func main() {
	r := row{Fields: len(strings.Fields("a b"))}
	_ = r
}
`
	got, err := Apply("main.go", src, Decisions{Types: []TypeNaming{
		{Name: "row", NewName: "Row", Fields: []FieldName{{Key: "Fields", Name: "Words"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "strings.Fields(") {
		t.Fatalf("the rename reached into a package qualifier:\n%s", got)
	}
	if !containsAll(got, "type Row struct", "Words int", "Row{Words:") {
		t.Fatalf("the type and its field were not renamed:\n%s", got)
	}
	if err := VerifyGo(got); err != nil {
		t.Fatalf("the result does not parse: %v\n%s", err, got)
	}
}

// A string literal is what the program prints, so nothing may touch one, even
// when it happens to contain the identifier being renamed.
func TestApplyLeavesLiteralsAlone(t *testing.T) {
	src := `package main

import "fmt"

func main() {
	c := 3
	fmt.Println("c is", c)
}
`
	got, err := Apply("main.go", src, Decisions{Renames: []Rename{{Old: "c", New: "count"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"c is"`) {
		t.Fatalf("a string literal was rewritten:\n%s", got)
	}
	if !strings.Contains(got, "count := 3") {
		t.Fatalf("the local was not renamed:\n%s", got)
	}
	if err := checkRenamed(src, got); err != nil {
		t.Fatalf("checkRenamed rejected a good rewrite: %v", err)
	}
}

// A method call on a local is a selector too, and a value rename must not
// follow the dot.
func TestApplyLeavesMethodNamesAlone(t *testing.T) {
	src := `package main

import (
	"bufio"
	"os"
)

func main() {
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		_ = s.Text()
	}
}
`
	got, err := Apply("main.go", src, Decisions{Renames: []Rename{{Old: "s", New: "lines"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(got, "lines.Scan()", "lines.Text()") {
		t.Fatalf("the receiver was not renamed:\n%s", got)
	}
	if err := VerifyGo(got); err != nil {
		t.Fatalf("the result does not parse: %v\n%s", err, got)
	}
}

func TestApplyInsertsDocComments(t *testing.T) {
	src := `package main

func Total(xs []int) int {
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return sum
}

func main() {}
`
	got, err := Apply("main.go", src, Decisions{Comments: []DocComment{
		{Name: "Total", Comment: "Total returns the sum of xs."},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "// Total returns the sum of xs.\nfunc Total(") {
		t.Fatalf("the doc comment was not attached to the declaration:\n%s", got)
	}
	if err := VerifyGo(got); err != nil {
		t.Fatalf("the result does not parse: %v\n%s", err, got)
	}
}

// A declaration the converter already documented keeps that comment: it was
// written from the Perl and is better grounded than anything added here.
func TestApplyKeepsExistingDocComments(t *testing.T) {
	src := `package main

// Total adds up the readings.
func Total(xs []int) int { return len(xs) }

func main() {}
`
	got, err := Apply("main.go", src, Decisions{Comments: []DocComment{
		{Name: "Total", Comment: "Total does something else entirely."},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "something else entirely") {
		t.Fatalf("an existing doc comment was replaced:\n%s", got)
	}
}

// Two renderings of one program are given one set of decisions, and both have
// to come out with the same names. This is the property that stops the clean
// and the annotated program disagreeing about what a variable is called.
func TestApplyIsStableAcrossRenderings(t *testing.T) {
	annotated := `package main

import (
	"fmt"
	"strings"
)

func main() {
	// A Perl hash becomes a Go map with a declared key and value type.
	byKey := map[string]int{}
	// Perl: for my $w (split ' ', $line)
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
	d := Decisions{Renames: []Rename{{Old: "c", New: "total"}, {Old: "item4", New: "word"}}}
	clean, err := Apply("main.go", sampleGo, d)
	if err != nil {
		t.Fatal(err)
	}
	ann, err := Apply("annotated/main.go", annotated, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"total", "word"} {
		if !strings.Contains(clean, name) || !strings.Contains(ann, name) {
			t.Fatalf("%q is not in both renderings", name)
		}
	}
	if !strings.Contains(ann, "// Perl: for my $w (split ' ', $line)") {
		t.Fatalf("the annotated rendering lost its explanation:\n%s", ann)
	}
}

// checkRenamed is the last line of defence, so it has to actually reject the
// things a naming pass is not allowed to do.
func TestCheckRenamedRejects(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		wantGate  string
	}{
		{
			name: "a changed literal",
			candidate: `package main

import "fmt"

func main() {
	c := 3
	fmt.Println("d is", c)
}
`,
			wantGate: "literals",
		},
		{
			name: "an added import",
			candidate: `package main

import (
	"fmt"
	"os"
)

func main() {
	c := 3
	fmt.Println("c is", c)
	_ = os.Args
}
`,
			wantGate: "imports",
		},
		{
			name: "a new way to end the program",
			candidate: `package main

import "fmt"

func main() {
	c := 3
	fmt.Println("c is", c)
	recover()
}
`,
			wantGate: "control flow",
		},
	}
	baseline := `package main

import "fmt"

func main() {
	c := 3
	fmt.Println("c is", c)
}
`
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkRenamed(baseline, tt.candidate)
			if err == nil {
				t.Fatal("accepted a rewrite that is a different program")
			}
			if gate, _ := gateOf(err); gate != tt.wantGate {
				t.Fatalf("gate = %q, want %q (%v)", gate, tt.wantGate, err)
			}
		})
	}
}
