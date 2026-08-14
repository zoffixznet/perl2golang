package idioms

import (
	"strings"
	"testing"
)

// scanBody wraps statements in a function and scans them.
func scanBody(t *testing.T, body string) []Hit {
	t.Helper()
	src := "package p\n\nimport (\n\t\"fmt\"\n\t\"sort\"\n\t\"strconv\"\n)\n\nvar _ = fmt.Sprintf\nvar _ = sort.Strings\nvar _ = strconv.Itoa\n\nfunc f() {\n" + body + "\n}\n"
	hits, err := Scan("t.go", []byte(src))
	if err != nil {
		t.Fatalf("scan failed: %v\nsource:\n%s", err, src)
	}
	return hits
}

func rules(hits []Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Rule
	}
	return out
}

func hasRule(hits []Hit, rule string) bool {
	for _, h := range hits {
		if h.Rule == rule {
			return true
		}
	}
	return false
}

func TestNumberedNames(t *testing.T) {
	hits := scanBody(t, `
	content2 := 1
	_ = content2
	for _, item4 := range []int{1} {
		_ = item4
	}
`)
	n := 0
	for _, h := range hits {
		if h.Rule == RuleNumberedName {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("wanted 2 numbered-name hits (content2, item4), got %d: %v", n, rules(hits))
	}
}

func TestNumberedNameExceptions(t *testing.T) {
	hits := scanBody(t, `
	utf8 := 1
	_ = utf8
	sha256 := 2
	_ = sha256
`)
	if hasRule(hits, RuleNumberedName) {
		t.Fatalf("stdlib vocabulary must not count as a numbered name: %v", hits)
	}
}

func TestAliasCopy(t *testing.T) {
	hits := scanBody(t, `
	x := 1
	y := x
	_ = y
	ok := true
	_ = ok
`)
	n := 0
	for _, h := range hits {
		if h.Rule == RuleAliasCopy {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("wanted exactly the y := x alias, got %d: %v", n, rules(hits))
	}
}

func TestBlankDiscardOfVariable(t *testing.T) {
	hits := scanBody(t, `
	x := 1
	_ = x
`)
	if !hasRule(hits, RuleBlankDiscard) {
		t.Fatalf("_ = x is the tell; got %v", rules(hits))
	}
	// Discarding a call result is a decision, not a silenced variable.
	hits = scanBody(t, `
	_ = fmt.Sprintf("%d", 1)
`)
	if hasRule(hits, RuleBlankDiscard) {
		t.Fatalf("_ = f() must not count: %v", rules(hits))
	}
}

func TestReturnImmediately(t *testing.T) {
	src := `package p

func g() int {
	result := 1 + 2
	return result
}
`
	hits, err := Scan("t.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(hits, RuleReturnImmediately) {
		t.Fatalf("result := ...; return result is the tell; got %v", rules(hits))
	}
}

func TestCStyleFor(t *testing.T) {
	hits := scanBody(t, `
	xs := []int{1, 2}
	sum := 0
	for i := 0; i < len(xs); i++ {
		sum += xs[i]
	}
	_ = sum
`)
	if !hasRule(hits, RuleCStyleFor) {
		t.Fatalf("an index used only as xs[i] is the tell; got %v", rules(hits))
	}
}

func TestCStyleForWithArithmeticIsFine(t *testing.T) {
	hits := scanBody(t, `
	xs := []int{1, 2}
	sum := 0
	for i := 0; i < len(xs); i++ {
		if i > 0 {
			sum += xs[i]
		}
	}
	_ = sum
`)
	if hasRule(hits, RuleCStyleFor) {
		t.Fatalf("an index used beyond xs[i] must not count: %v", rules(hits))
	}
}

func TestLegacySort(t *testing.T) {
	hits := scanBody(t, `
	xs := []string{"b", "a"}
	sort.Strings(xs)
`)
	if !hasRule(hits, RuleLegacySort) {
		t.Fatalf("sort.Strings is the tell; got %v", rules(hits))
	}
}

func TestMinMaxByHand(t *testing.T) {
	hits := scanBody(t, `
	limit := 0
	n := 12
	if n-1 < 9 {
		limit = n - 1
	} else {
		limit = 9
	}
	_ = limit
`)
	if !hasRule(hits, RuleMinMaxByHand) {
		t.Fatalf("if a < b assigning a else b is min; got %v", rules(hits))
	}
}

func TestMinMaxByHandNotFooledByOtherValues(t *testing.T) {
	hits := scanBody(t, `
	limit := 0
	n := 12
	if n < 9 {
		limit = 1
	} else {
		limit = 2
	}
	_ = limit
`)
	if hasRule(hits, RuleMinMaxByHand) {
		t.Fatalf("assigned values unrelated to the comparison are not min or max: %v", rules(hits))
	}
}

func TestSprintfConcat(t *testing.T) {
	hits := scanBody(t, `
	a, b := "x", "y"
	c := fmt.Sprintf("%s%s", a, b)
	_ = c
`)
	if !hasRule(hits, RuleSprintfConcat) {
		t.Fatalf("Sprintf of only verbs is concatenation; got %v", rules(hits))
	}
	hits = scanBody(t, `
	c := fmt.Sprintf("%s: %d", "x", 1)
	_ = c
`)
	if hasRule(hits, RuleSprintfConcat) {
		t.Fatalf("a format string with real text must not count: %v", rules(hits))
	}
}

func TestStringAppendLoop(t *testing.T) {
	hits := scanBody(t, `
	out := ""
	for i := 0; i < 3; i++ {
		out += "x"
	}
	_ = out
`)
	if !hasRule(hits, RuleStringAppendLoop) {
		t.Fatalf("string += in a loop is the tell; got %v", rules(hits))
	}
	hits = scanBody(t, `
	n := 0
	for i := 0; i < 3; i++ {
		n += i
	}
	_ = n
`)
	if hasRule(hits, RuleStringAppendLoop) {
		t.Fatalf("numeric += must not count: %v", rules(hits))
	}
}

func TestElseAfterExit(t *testing.T) {
	src := `package p

func g(n int) int {
	if n > 0 {
		return n
	} else {
		return -n
	}
}
`
	hits, err := Scan("t.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(hits, RuleElseAfterExit) {
		t.Fatalf("else after return is the tell; got %v", rules(hits))
	}
}

func TestElseIfChainIsFine(t *testing.T) {
	src := `package p

func g(n int) int {
	if n > 0 {
		return n
	} else if n < -10 {
		return 0
	}
	return -n
}
`
	hits, err := Scan("t.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if hasRule(hits, RuleElseAfterExit) {
		t.Fatalf("an else-if chain is ordinary control flow: %v", rules(hits))
	}
}

func TestStringifiedKey(t *testing.T) {
	hits := scanBody(t, `
	m := map[string]int{}
	m[strconv.Itoa(3)] = 1
`)
	if !hasRule(hits, RuleStringifiedKey) {
		t.Fatalf("indexing with strconv.Itoa is the tell; got %v", rules(hits))
	}
}

func TestMapStringAnyAndInterfaceBraces(t *testing.T) {
	hits := scanBody(t, `
	m := map[string]any{}
	var v interface{}
	_ = m
	_ = v
`)
	if !hasRule(hits, RuleMapStringAny) {
		t.Fatalf("map[string]any is the tell; got %v", rules(hits))
	}
	if !hasRule(hits, RuleInterfaceBraces) {
		t.Fatalf("interface{} spelling is the tell; got %v", rules(hits))
	}
}

func TestCountSkipsHelpersAndBrokenFiles(t *testing.T) {
	files := map[string][]byte{
		"main.go":    []byte("package p\n\nfunc f() {\n\tx := 1\n\t_ = x\n}\n"),
		"helpers.go": []byte("package p\n\nfunc h() {\n\ty := 1\n\t_ = y\n}\n"),
		"broken.go":  []byte("package p\n\nfunc {"),
		"notes.md":   []byte("# not Go"),
	}
	total, byRule := Count(files)
	if total != 1 || byRule[RuleBlankDiscard] != 1 {
		t.Fatalf("wanted exactly main.go's one hit, got total %d, byRule %v", total, byRule)
	}
}

func TestScanRealShapeFromGeneratedOutput(t *testing.T) {
	// The shape of a real converter emission, distilled: numbered names, an
	// alias copy, a hand-rolled min, and a silenced variable.
	src := `package main

import "fmt"

func main() {
	byKey := map[string]int{"a": 1}
	stop := byKey
	content := []string{"a", "b"}
	var content2 int
	if len(content)-1 < 9 {
		content2 = len(content) - 1
	} else {
		content2 = 9
	}
	maxVal := content2
	_ = maxVal
	fmt.Println(stop)
}
`
	hits, err := Scan("main.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{RuleNumberedName, RuleAliasCopy, RuleMinMaxByHand, RuleBlankDiscard} {
		if !hasRule(hits, want) {
			t.Errorf("missing %s in %v", want, rules(hits))
		}
	}
	for _, h := range hits {
		if h.Line <= 0 || h.Detail == "" {
			t.Errorf("hit %s has no position or no detail", h.Rule)
		}
		if strings.Contains(h.Detail, "\n") {
			t.Errorf("detail must be one phrase: %q", h.Detail)
		}
	}
}
