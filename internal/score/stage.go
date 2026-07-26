package score

import (
	"fmt"
	"strconv"
	"strings"

	"perl2go/internal/report"
)

// Stage is one rung of the conversion ladder. An entry reaches a stage or it
// does not, and the scorecard is the count of entries that reached each one.
type Stage int

// The ladder, in the order an entry climbs it.
const (
	// StageParsed means the Perl parsed and no statement was left
	// untranslated.
	StageParsed Stage = iota
	// StageTyped means type inference resolved every variable it tracked
	// without falling back to the dynamic value type.
	StageTyped
	// StageEmitted means Go came out, and that Go is valid Go.
	StageEmitted
	// StageCompiled means the real Go toolchain built it.
	StageCompiled
	// StageEquivalent means the built program produced what the Perl
	// produced: same stdout, same exit status, same files.
	StageEquivalent
	// StageHonest is tier 4's standard. Those entries pass by saying
	// something true about a construct, not by translating it.
	StageHonest
	numStages
)

var stageNames = [numStages]string{"parsed", "typed", "emitted", "compiled", "equivalent", "honest"}

// String returns the stage's name as it appears in the table and in JSON.
func (s Stage) String() string {
	if s < 0 || s >= numStages {
		return "stage" + strconv.Itoa(int(s))
	}
	return stageNames[s]
}

// Stages returns every stage in ladder order.
func Stages() []Stage {
	out := make([]Stage, 0, numStages)
	for s := Stage(0); s < numStages; s++ {
		out = append(out, s)
	}
	return out
}

// Outcome is what happened at one stage for one entry.
type Outcome int

// The three outcomes. A skip is never a pass: it means the check could not be
// made at all, which is a different claim from the check succeeding.
const (
	Fail Outcome = iota
	Pass
	Skip
	// NotApplicable marks a stage that is not this entry's standard, such as
	// equivalence for a tier 4 entry. It counts towards nothing.
	NotApplicable
)

// String returns the outcome as it appears in JSON.
func (o Outcome) String() string {
	switch o {
	case Pass:
		return "pass"
	case Skip:
		return "skip"
	case NotApplicable:
		return "n/a"
	default:
		return "fail"
	}
}

// StageResult is one stage's outcome plus the reason behind it. The reason is
// the whole point: a number that went down should say what went down.
type StageResult struct {
	Outcome Outcome `json:"outcome"`
	Reason  string  `json:"reason,omitempty"`
}

// MarshalJSON writes the outcome as its name.
func (o Outcome) MarshalJSON() ([]byte, error) { return []byte(`"` + o.String() + `"`), nil }

// UnmarshalJSON reads an outcome written as its name.
func (o *Outcome) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	switch s {
	case "pass":
		*o = Pass
	case "skip":
		*o = Skip
	case "n/a":
		*o = NotApplicable
	case "fail":
		*o = Fail
	default:
		return fmt.Errorf("unknown outcome %q", s)
	}
	return nil
}

func pass() StageResult              { return StageResult{Outcome: Pass} }
func fail(reason string) StageResult { return StageResult{Outcome: Fail, Reason: reason} }
func skip(reason string) StageResult { return StageResult{Outcome: Skip, Reason: reason} }
func notApplicable() StageResult     { return StageResult{Outcome: NotApplicable} }
func (r StageResult) passed() bool   { return r.Outcome == Pass }

// Classification is what a conversion report says about the stages that can be
// judged from the report alone.
type Classification struct {
	Parsed  StageResult
	Typed   StageResult
	Emitted StageResult

	// Refusals counts constructs the tool declined to translate. Entries the
	// tool raises about its own output are not counted here: those are
	// defects in the tool, and they are judged at the compiled stage.
	Refusals int
	// Approximations counts constructs translated with a reported change in
	// meaning.
	Approximations int
	// ParseErrors counts front-end diagnostics.
	ParseErrors int
	// Todos counts the TODO markers left in the generated code.
	Todos int
	// Symbols and SymbolsTyped drive the dynamic-fallback rate.
	Symbols      int
	SymbolsTyped int
	// SelfReported is true when the tool raised an entry about its own
	// output, which means the generated Go did not parse or did not build.
	SelfReported bool
}

// selfCheckFloor is the first diagnostic code in the band the tool uses to
// report on its own output rather than on the input. Those entries say the
// generated Go is broken, which is a compile-stage fact, not a statement the
// input failed to translate.
const selfCheckFloor = 8500

// selfCheck reports whether a diagnostic code belongs to the band the tool
// reserves for verifying its own output.
func selfCheck(code string) bool {
	digits := strings.TrimPrefix(code, "P2G")
	n, err := strconv.Atoi(digits)
	return err == nil && n >= selfCheckFloor
}

// Classify reads a conversion report and decides the three stages that need
// nothing but the report: whether the Perl went through untranslated, whether
// type inference resolved, and whether valid Go came out.
func Classify(rep *report.Report) Classification {
	var c Classification
	if rep == nil {
		reason := "the conversion produced no report"
		return Classification{Parsed: fail(reason), Typed: fail(reason), Emitted: fail(reason)}
	}

	selfRefusals := 0
	for _, e := range rep.Entries {
		if selfCheck(e.Code) {
			if e.Severity == report.Refuse {
				c.SelfReported = true
				selfRefusals++
			}
			continue
		}
		switch e.Severity {
		case report.Refuse:
			c.Refusals++
		case report.Warn:
			c.Approximations++
		}
	}
	c.ParseErrors = rep.Stats.ParseErrors
	// The tool counts one TODO per refused construct. Refusals it raised
	// about its own output leave no TODO in the generated code, so they are
	// taken back out of the count rather than inflating it.
	if c.Todos = rep.Stats.Todos - selfRefusals; c.Todos < 0 {
		c.Todos = 0
	}
	c.Symbols = rep.Stats.Symbols
	c.SymbolsTyped = rep.Stats.SymbolsTyped

	switch {
	case c.ParseErrors > 0 && c.Refusals > 0:
		c.Parsed = fail(fmt.Sprintf("%s and %s",
			plural(c.ParseErrors, "parse error"), plural(c.Refusals, "untranslated construct")))
	case c.ParseErrors > 0:
		c.Parsed = fail(plural(c.ParseErrors, "parse error"))
	case c.Refusals > 0:
		c.Parsed = fail(plural(c.Refusals, "untranslated construct"))
	default:
		c.Parsed = pass()
	}

	dynamic := c.Symbols - c.SymbolsTyped
	if dynamic > 0 {
		c.Typed = fail(fmt.Sprintf("%s left dynamic out of %d", plural(dynamic, "variable"), c.Symbols))
	} else {
		c.Typed = pass()
	}

	if rep.Verified.Parsed {
		c.Emitted = pass()
	} else {
		reason := "the generated Go is not valid Go"
		if rep.Verified.Error != "" {
			reason += ": " + firstLine(rep.Verified.Error)
		}
		c.Emitted = fail(reason)
	}
	return c
}

// plural renders a count with its noun, pluralised the lazy English way that
// happens to be right for every noun this file uses.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// firstLine trims a multi-line error down to something a table row can hold.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + " ..."
	}
	return s
}

// truncate shortens a reason to n runes, so one runaway compiler message cannot
// take over the failure list.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-3]) + "..."
}
