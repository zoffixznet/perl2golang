package score

import (
	"strings"
	"testing"

	"perl2go/internal/report"
)

// makeReport builds a report the way the converter would, so the classifier is
// tested against the same shape it sees in a real run.
func makeReport(entries []report.Entry, stats report.Stats, v report.Verification) *report.Report {
	r := &report.Report{Verified: v}
	for _, e := range entries {
		r.Add(e)
	}
	// Add keeps its own counters, so the caller's stats are laid on top of
	// whatever Add worked out.
	stats.Approximated = r.Stats.Approximated
	stats.Refused = r.Stats.Refused
	r.Stats = stats
	return r
}

func refusal(code string) report.Entry {
	return report.Entry{Code: code, Severity: report.Refuse, Construct: "eval EXPR"}
}

func warning(code string) report.Entry {
	return report.Entry{Code: code, Severity: report.Warn, Construct: "sort stability"}
}

func TestClassify(t *testing.T) {
	built := report.Verification{Parsed: true, Built: true, Toolchain: true}

	tests := []struct {
		name           string
		rep            *report.Report
		wantTranslated Outcome
		wantTyped      Outcome
		wantEmitted    Outcome
		wantTodos      int
		wantReasons    []string
	}{
		{
			name: "a clean conversion reaches every report stage",
			rep: makeReport(nil,
				report.Stats{Statements: 20, Converted: 20, Symbols: 6, SymbolsTyped: 6}, built),
			wantTranslated: Pass, wantTyped: Pass, wantEmitted: Pass,
		},
		{
			name: "a refused construct means the Perl did not go through untranslated",
			rep: makeReport([]report.Entry{refusal("P2G8001")},
				report.Stats{Statements: 20, Todos: 1, Symbols: 4, SymbolsTyped: 4}, built),
			wantTranslated: Fail, wantTyped: Pass, wantEmitted: Pass,
			wantTodos:   1,
			wantReasons: []string{"1 untranslated construct"},
		},
		{
			name: "parse errors fail the parsed stage",
			rep: makeReport(nil,
				report.Stats{Statements: 5, ParseErrors: 3, Symbols: 1, SymbolsTyped: 1}, built),
			wantTranslated: Fail, wantTyped: Pass, wantEmitted: Pass,
			wantReasons: []string{"3 parse errors"},
		},
		{
			name: "parse errors and refusals are both named",
			rep: makeReport([]report.Entry{refusal("P2G8001"), refusal("P2G8020")},
				report.Stats{Statements: 5, ParseErrors: 1, Todos: 2}, built),
			wantTranslated: Fail, wantTyped: Pass, wantEmitted: Pass,
			wantTodos:   2,
			wantReasons: []string{"1 parse error and 2 untranslated constructs"},
		},
		{
			name: "a variable left dynamic fails the typed stage",
			rep: makeReport(nil,
				report.Stats{Statements: 9, Symbols: 5, SymbolsTyped: 3}, built),
			wantTranslated: Pass, wantTyped: Fail, wantEmitted: Pass,
			wantReasons: []string{"2 variables left dynamic out of 5"},
		},
		{
			name: "a program with no variables has nothing to leave dynamic",
			rep: makeReport(nil,
				report.Stats{Statements: 1}, built),
			wantTranslated: Pass, wantTyped: Pass, wantEmitted: Pass,
		},
		{
			name: "Go that does not parse fails the emitted stage",
			rep: makeReport(nil, report.Stats{Statements: 4},
				report.Verification{Parsed: false, Toolchain: true, Error: "main.go:4:1: expected ';'\nsecond line"}),
			wantTranslated: Pass, wantTyped: Pass, wantEmitted: Fail,
			wantReasons: []string{"not valid Go", "expected ';'"},
		},
		{
			name: "a refusal about the tool's own output is not an untranslated construct",
			rep: makeReport([]report.Entry{refusal("P2G8505")},
				report.Stats{Statements: 12, Todos: 1, Symbols: 3, SymbolsTyped: 3},
				report.Verification{Parsed: true, Toolchain: true}),
			wantTranslated: Pass, wantTyped: Pass, wantEmitted: Pass,
			wantTodos: 0,
		},
		{
			name: "an approximation is recorded but does not fail a stage",
			rep: makeReport([]report.Entry{warning("P2G8050")},
				report.Stats{Statements: 12, Symbols: 3, SymbolsTyped: 3}, built),
			wantTranslated: Pass, wantTyped: Pass, wantEmitted: Pass,
		},
		{
			name:           "no report at all fails everything",
			rep:            nil,
			wantTranslated: Fail, wantTyped: Fail, wantEmitted: Fail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.rep)
			if got.Translated.Outcome != tt.wantTranslated {
				t.Errorf("translated = %v (%s), want %v", got.Translated.Outcome, got.Translated.Reason, tt.wantTranslated)
			}
			if got.Typed.Outcome != tt.wantTyped {
				t.Errorf("typed = %v (%s), want %v", got.Typed.Outcome, got.Typed.Reason, tt.wantTyped)
			}
			if got.Emitted.Outcome != tt.wantEmitted {
				t.Errorf("emitted = %v (%s), want %v", got.Emitted.Outcome, got.Emitted.Reason, tt.wantEmitted)
			}
			if got.Todos != tt.wantTodos {
				t.Errorf("todos = %d, want %d", got.Todos, tt.wantTodos)
			}
			all := got.Translated.Reason + " | " + got.Typed.Reason + " | " + got.Emitted.Reason
			for _, want := range tt.wantReasons {
				if !strings.Contains(all, want) {
					t.Errorf("reasons %q do not mention %q", all, want)
				}
			}
		})
	}
}

func TestClassifyCountsApproximationsAndRefusalsSeparately(t *testing.T) {
	rep := makeReport(
		[]report.Entry{refusal("P2G8001"), warning("P2G8050"), warning("P2G7585"), refusal("P2G8505")},
		report.Stats{Statements: 30, Todos: 2, Symbols: 8, SymbolsTyped: 5},
		report.Verification{Parsed: true, Toolchain: true, Built: true},
	)
	got := Classify(rep)
	if got.Refusals != 1 {
		t.Errorf("refusals = %d, want 1 (the tool's own output does not count)", got.Refusals)
	}
	if got.Approximations != 2 {
		t.Errorf("approximations = %d, want 2", got.Approximations)
	}
	if !got.SelfReported {
		t.Error("the report says the tool's output is broken; SelfReported should say so")
	}
	if got.Todos != 1 {
		t.Errorf("todos = %d, want 1", got.Todos)
	}
	if got.Symbols != 8 || got.SymbolsTyped != 5 {
		t.Errorf("symbols = %d/%d, want 5/8", got.SymbolsTyped, got.Symbols)
	}
}

func TestSelfCheck(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{"P2G8501", true},
		{"P2G8505", true},
		{"P2G8999", true},
		{"P2G8499", false},
		{"P2G8001", false},
		{"P2G1000", false},
		{"", false},
		{"nonsense", false},
	}
	for _, tt := range tests {
		if got := selfCheck(tt.code); got != tt.want {
			t.Errorf("selfCheck(%q) = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestStageNames(t *testing.T) {
	want := []string{"translated", "typed", "emitted", "compiled", "equivalent", "honest"}
	got := Stages()
	if len(got) != len(want) {
		t.Fatalf("Stages() has %d entries, want %d", len(got), len(want))
	}
	for i, s := range got {
		if s.String() != want[i] {
			t.Errorf("stage %d = %q, want %q", i, s, want[i])
		}
	}
}
