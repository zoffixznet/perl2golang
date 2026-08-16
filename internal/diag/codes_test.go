package diag

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"perl2golang/internal/report"
)

// kbDir is the teaching knowledge base, read as files rather than through the
// teach package so that diag keeps no dependency on it.
const kbDir = "../teach/kb"

// TestConceptsExist fails when the catalogue points at a teaching concept that
// nobody wrote. A dangling concept id is a promise the tool cannot keep.
func TestConceptsExist(t *testing.T) {
	entries, err := os.ReadDir(kbDir)
	if err != nil {
		t.Fatalf("read knowledge base: %v", err)
	}
	have := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		have[strings.TrimSuffix(name, ".md")] = true
	}
	if len(have) == 0 {
		t.Fatalf("no concept files found in %s", kbDir)
	}
	for _, code := range Codes() {
		for _, id := range catalogue[code].Concepts {
			if !have[id] {
				t.Errorf("%s references concept %q, and %s is not there",
					code, id, filepath.Join(kbDir, id+".md"))
			}
		}
	}
}

// raiserDirs are the packages that raise diagnostics by writing a code as a
// string literal. They are read as files rather than imported, so the registry
// keeps no dependency on the code that uses it.
var raiserDirs = []string{"../lower", "../convert"}

// codeLiteral matches a diagnostic code written as a Go string literal.
var codeLiteral = regexp.MustCompile(`"(P2G[0-9]{4})"`)

// TestRaisedCodesAreRegistered fails when a package raises a code the registry
// does not know. The registry is the only place a code is defined, so a literal
// that is not in it would reach the reader as a code with no explanation behind
// it, which `perl2golang explain` could not answer for.
func TestRaisedCodesAreRegistered(t *testing.T) {
	found := 0
	for _, dir := range raiserDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") {
				continue
			}
			path := filepath.Join(dir, name)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			for _, m := range codeLiteral.FindAllSubmatchIndex(src, -1) {
				found++
				code := Code(src[m[2]:m[3]])
				if Known(code) {
					continue
				}
				line := 1 + strings.Count(string(src[:m[0]]), "\n")
				t.Errorf("%s:%d raises %s, and the registry has no entry for it",
					path, line, code)
			}
		}
	}
	if found == 0 {
		t.Fatalf("no diagnostic codes found in %v, so this test proves nothing", raiserDirs)
	}
	t.Logf("checked %d code literals against %d registered codes", found, len(catalogue))
}

// TestKnown covers the answer both ways, because a Known that always said yes
// would let TestRaisedCodesAreRegistered pass on a registry that is missing
// half its entries.
func TestKnown(t *testing.T) {
	if !Known(RegexLookahead) {
		t.Errorf("Known(%s) = false, and it is in the registry", RegexLookahead)
	}
	if Known("P2G0000") {
		t.Error("Known(P2G0000) = true, and no such code is registered")
	}
}

// TestCatalogueStyle runs the same checks init runs, one code at a time, so a
// failure names the code and the rule instead of stopping the whole package.
func TestCatalogueStyle(t *testing.T) {
	for _, code := range Codes() {
		t.Run(string(code), func(t *testing.T) {
			if err := validate(code, catalogue[code]); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestCodeShape checks the format and the range allocation.
func TestCodeShape(t *testing.T) {
	shape := regexp.MustCompile(`^P2G[0-9]{4}$`)
	for _, code := range Codes() {
		if !shape.MatchString(string(code)) {
			t.Errorf("code %q is not P2G plus four digits", code)
		}
	}
}

// TestCodesSorted checks that Codes returns a stable, sorted list, which is
// what `perl2golang explain` lists and what a diff of the registry compares.
func TestCodesSorted(t *testing.T) {
	got := Codes()
	if !slices.IsSorted(got) {
		t.Errorf("Codes() is not sorted: %v", got)
	}
	if len(got) != len(catalogue) {
		t.Errorf("Codes() returned %d codes, the catalogue holds %d", len(got), len(catalogue))
	}
}

// TestRangeAllocation checks that a code sits in the range its subsystem owns,
// because the ranges are what makes `grep P2G4` a useful question.
func TestRangeAllocation(t *testing.T) {
	ranges := []struct {
		lo, hi int
		name   string
	}{
		{1, 99, "tool level"},
		{1000, 1499, "lexing"},
		{1500, 1999, "parsing"},
		{2000, 2499, "scope and context"},
		{2500, 2999, "references and mutation"},
		{3000, 3499, "type inference"},
		{3500, 3999, "IR lowering"},
		{4000, 4499, "regex features RE2 lacks"},
		{4500, 4999, "regex and splitting semantics"},
		{5000, 5499, "strings"},
		{5500, 5999, "numbers and sort"},
		{6000, 6499, "file I/O"},
		{6500, 6999, "processes"},
		{7000, 7499, "OO"},
		{7500, 7999, "modules"},
		{8000, 8499, "dynamic Perl"},
		{8500, 8999, "self-verification"},
		{9500, 9999, "REPL"},
	}
	for _, code := range Codes() {
		n := 0
		for _, r := range string(code)[3:] {
			n = n*10 + int(r-'0')
		}
		found := false
		for _, r := range ranges {
			if n >= r.lo && n <= r.hi {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s falls outside every allocated range", code)
		}
	}
}

// TestSituationsCovered pins the codes for the constructs the converter is
// known to meet. Renaming or dropping one of these is a decision, not an
// accident, so it has to break a test.
func TestSituationsCovered(t *testing.T) {
	want := map[string]Code{
		"a statement that did not parse": StatementNotParsed,
		"local":                          LocalDynamicScope,
		"wantarray":                      Wantarray,
		"redo":                           Redo,
		"goto":                           GotoLabel,
		"regex backreference":            RegexBackreference,
		"regex lookahead":                RegexLookahead,
		"regex lookbehind":               RegexLookbehind,
		"s///e":                          SubstEval,
		"tr/// with modifiers":           TrModifiers,
		"sprintf with no Go verb":        SprintfFormat,
		"default stringwise sort":        DefaultSortIsStringwise,
		"magic string increment":         MagicIncrement,
		"modulo with a negative operand": ModuloSign,
		"two-argument open":              TwoArgOpen,
		"backticks":                      Backticks,
		"system":                         SystemCall,
		"bless":                          Bless,
		"method dispatch by name":        DynamicMethodName,
		"eval STRING":                    EvalString,
		"a module with no Go mapping":    ModuleUnmapped,
		"type inference fell back":       DynamicScalar,
		"generated Go does not compile":  OutputDoesNotCompile,
	}
	for situation, code := range want {
		if _, ok := Lookup(code); !ok {
			t.Errorf("%s: %s is not in the registry", situation, code)
		}
	}
}

// TestSeverityIsUsed checks that all three severities are in play, which is the
// cheapest guard against a registry that quietly turns into all warnings.
func TestSeverityIsUsed(t *testing.T) {
	count := map[report.Severity]int{}
	for _, code := range Codes() {
		count[catalogue[code].Severity]++
	}
	for _, s := range []report.Severity{report.Note, report.Warn, report.Refuse} {
		if count[s] == 0 {
			t.Errorf("no code has severity %s", s)
		}
	}
	t.Logf("notes %d, warnings %d, refusals %d, total %d",
		count[report.Note], count[report.Warn], count[report.Refuse], len(catalogue))
}

// TestValidateCatchesBadEntries checks the style checker itself, so the init
// panic is known to fire on the things it claims to catch.
func TestValidateCatchesBadEntries(t *testing.T) {
	good := Entry{
		Severity: report.Warn,
		Message:  "`local $/` changes the record separator for every sub this call reaches",
		Short:    "local $/ changes reads elsewhere",
		Advice:   "pass the separator explicitly",
	}
	if err := validate("P2G2004", good); err != nil {
		t.Fatalf("a good entry did not validate: %v", err)
	}
	cases := []struct {
		name string
		code Code
		mut  func(e *Entry)
	}{
		{"bad code", "P2G-4", func(e *Entry) {}},
		{"empty message", "P2G2004", func(e *Entry) { e.Message = "" }},
		{"upper case message", "P2G2004", func(e *Entry) { e.Message = "Local changes the separator" }},
		{"trailing period", "P2G2004", func(e *Entry) { e.Message += "." }},
		{"long message", "P2G2004", func(e *Entry) { e.Message = strings.Repeat("a", 111) }},
		{"empty short", "P2G2004", func(e *Entry) { e.Short = "" }},
		{"long short", "P2G2004", func(e *Entry) { e.Short = strings.Repeat("a", 45) }},
		{"no advice", "P2G2004", func(e *Entry) { e.Advice = "" }},
		{"emoji", "P2G2004", func(e *Entry) { e.Advice = "pass it explicitly ✅" }},
		{"em dash", "P2G2004", func(e *Entry) { e.Advice = "pass it — explicitly" }},
		{"apology", "P2G2004", func(e *Entry) { e.Advice = "sorry, pass it explicitly" }},
		{"first person", "P2G2004", func(e *Entry) { e.Advice = "we pass it explicitly" }},
		{"session vocabulary", "P2G2004", func(e *Entry) { e.Cost = "untested on this machine" }},
		{"flat refusal", "P2G2004", func(e *Entry) { e.Message = "`local $/` is not supported" }},
		{"newline", "P2G2004", func(e *Entry) { e.Advice = "pass it\nexplicitly" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := good
			tc.mut(&e)
			if err := validate(tc.code, e); err == nil {
				t.Errorf("validate accepted %s", tc.name)
			}
		})
	}
}

// TestCapitalisedLeadersAllowed documents which upper-case openings are the
// spelling of an identifier rather than sentence case.
func TestCapitalisedLeadersAllowed(t *testing.T) {
	ok := []string{
		"`BEGIN` and `END` blocks became `init`",
		"`File::Find` walks in readdir order",
		"JSON numbers decode to `float64`",
		"Storable files have no Go reader",
		"RE2 has no lookahead, so the pattern is refused",
		"`__DATA__` section read as 314 bytes",
	}
	for _, s := range ok {
		if !startsLowerCase(s) {
			t.Errorf("rejected a correctly spelled opening: %q", s)
		}
	}
	bad := []string{
		"Lookahead is not available in Go",
		"The pattern is built at run time",
	}
	for _, s := range bad {
		if startsLowerCase(s) {
			t.Errorf("accepted sentence case: %q", s)
		}
	}
}
