package diag

import (
	"strings"
	"testing"

	"perl2golang/internal/report"
)

func TestNewFillsTheEntry(t *testing.T) {
	e := New(RuntimePattern, Pos{File: "logwatch.pl", Line: 88, Col: 27}, "$pat", "$pat")

	if e.Code != "P2G4080" {
		t.Errorf("code = %q, want P2G4080", e.Code)
	}
	if e.Severity != report.Warn {
		t.Errorf("severity = %v, want warning", e.Severity)
	}
	if e.SeverityName != "warning" {
		t.Errorf("severity name = %q, want warning", e.SeverityName)
	}
	if e.Construct != "$pat" {
		t.Errorf("construct = %q", e.Construct)
	}
	want := "the pattern is built from `$pat` at run time, so it compiles at run time too"
	if e.Message != want {
		t.Errorf("message = %q, want %q", e.Message, want)
	}
	if e.Short == "" || e.Advice == "" {
		t.Error("short and advice must be carried over from the registry")
	}
	if e.Line != 88 || e.Col != 27 {
		t.Errorf("position = %d:%d, want 88:27", e.Line, e.Col)
	}
	if len(e.Concepts) == 0 {
		t.Error("concepts must be carried over from the registry")
	}
}

func TestNewEscapesPercent(t *testing.T) {
	e := New(ModuloSign, Pos{Line: 4, Col: 9}, "%")
	if strings.Contains(e.Message, "%%") {
		t.Errorf("message still holds a template escape: %q", e.Message)
	}
	if !strings.Contains(e.Message, "`%`") {
		t.Errorf("message lost the operator: %q", e.Message)
	}
}

func TestNewDoesNotShareConcepts(t *testing.T) {
	e := New(RegexLookahead, Pos{Line: 1, Col: 1}, "(?!#)", "(?!#)")
	e.Concepts[0] = "mutated"
	again := New(RegexLookahead, Pos{Line: 1, Col: 1}, "(?!#)", "(?!#)")
	if again.Concepts[0] == "mutated" {
		t.Error("New handed out the registry's own slice")
	}
	reg, _ := Lookup(RegexLookahead)
	reg.Concepts[0] = "mutated"
	if fresh, _ := Lookup(RegexLookahead); fresh.Concepts[0] == "mutated" {
		t.Error("Lookup handed out the registry's own slice")
	}
}

// TestNewUnknownCode checks that a bug in the caller becomes a diagnostic
// rather than a crash on the way to reporting one.
func TestNewUnknownCode(t *testing.T) {
	e := New("P2G9999", Pos{File: "x.pl", Line: 3, Col: 1}, "something")
	if e.Code != string(UnknownCode) {
		t.Errorf("code = %q, want %s", e.Code, UnknownCode)
	}
	if !strings.Contains(e.Message, "P2G9999") {
		t.Errorf("message does not name the missing code: %q", e.Message)
	}
	if e.Severity != report.Refuse {
		t.Errorf("severity = %v, want refused", e.Severity)
	}
	if e.Line != 3 {
		t.Errorf("position was dropped: %d", e.Line)
	}
}

func TestWithPerl(t *testing.T) {
	e := New(StatementNotParsed, Pos{Line: 7, Col: 1}, "statement")
	e = WithPerl(e, "  my $x = frobnicate();  \n")
	if want := "  my $x = frobnicate();"; e.Perl != want {
		t.Errorf("perl = %q, want %q", e.Perl, want)
	}
}

func TestLookup(t *testing.T) {
	if _, ok := Lookup("P2G0000"); ok {
		t.Error("Lookup found a code that is not registered")
	}
	got, ok := Lookup(EvalString)
	if !ok {
		t.Fatal("Lookup missed a registered code")
	}
	if got.Severity != report.Refuse || got.Converted == "" || got.Advice == "" {
		t.Errorf("registry row is incomplete: %+v", got)
	}
}

// TestEntryFeedsTheReport checks the join with the report model: an entry built
// here counts in the right column and carries its concepts into the summary.
func TestEntryFeedsTheReport(t *testing.T) {
	var r report.Report
	r.Add(New(RuntimePattern, Pos{Line: 88, Col: 27}, "$pat", "$pat"))
	r.Add(New(EvalString, Pos{Line: 143, Col: 22}, "eval STRING"))
	r.Add(New(HashOrder, Pos{Line: 12, Col: 5}, "each %counts"))

	if r.Stats.Approximated != 1 {
		t.Errorf("approximated = %d, want 1", r.Stats.Approximated)
	}
	if r.Stats.Refused != 1 {
		t.Errorf("refused = %d, want 1", r.Stats.Refused)
	}
	if r.Clean() {
		t.Error("a report holding a refusal is not clean")
	}
	if len(r.Concepts) == 0 {
		t.Error("concepts did not reach the report")
	}
}
