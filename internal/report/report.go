// Package report models the honest account of what a conversion did.
//
// Every conversion produces one Report. It lists what converted cleanly, what
// was approximated and how the approximation differs, and what was refused
// outright and why. The report is the same data behind the terminal summary,
// the generated Markdown, and the JSON output, so those three can never
// disagree with each other.
package report

import "sort"

// Severity ranks an entry for the terminal summary and for --strict.
type Severity int

// The severity ladder. Note is ordinary explanation, Warn means the generated
// code differs from the Perl in a way the developer must know about, and Refuse
// means nothing was generated for the construct at all.
const (
	Note Severity = iota
	Warn
	Refuse
)

// String renders the severity as the word used in output.
func (s Severity) String() string {
	switch s {
	case Warn:
		return "warning"
	case Refuse:
		return "refused"
	default:
		return "note"
	}
}

// Entry is one thing worth saying about the conversion.
//
// Short is what goes in the clean program's TODO comment, so it never mentions
// Perl or conversion. Message is the full explanation for the annotated
// program, the report, and the terminal. Advice tells the developer what to do
// by hand.
type Entry struct {
	// Code is the stable diagnostic code, for example "P2G2104".
	Code     string   `json:"code"`
	Severity Severity `json:"-"`
	// SeverityName mirrors Severity for JSON consumers.
	SeverityName string `json:"severity"`
	// Construct names the Perl construct, for example "sort with a named sub".
	Construct string `json:"construct"`
	Short     string `json:"short"`
	Message   string `json:"message"`
	Advice    string `json:"advice,omitempty"`
	// Perl is the original source text of the construct.
	Perl string `json:"perl,omitempty"`
	// File names the source this came from when it is not the file being
	// converted, which happens once a script pulls in a module beside it.
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
	Col  int    `json:"col,omitempty"`
	// Concepts are teaching concept ids that explain the mismatch.
	Concepts []string `json:"concepts,omitempty"`
}

// Symbol records what type inference concluded about one variable, so the
// developer can see which parts of their program the tool actually understood.
type Symbol struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Inferred is false when the tool fell back to a dynamic representation.
	Inferred bool   `json:"inferred"`
	Reason   string `json:"reason,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// Stats are the countable facts about a conversion.
type Stats struct {
	Statements   int `json:"statements"`
	Converted    int `json:"converted"`
	Approximated int `json:"approximated"`
	Refused      int `json:"refused"`
	Todos        int `json:"todos"`
	// Symbols and SymbolsTyped drive the dynamic-fallback rate.
	Symbols      int `json:"symbols"`
	SymbolsTyped int `json:"symbols_typed"`
	// ParseErrors counts diagnostics from the front end.
	ParseErrors int `json:"parse_errors"`
}

// Report is the whole account of one conversion.
type Report struct {
	// Source is the input path, or "-e" / "-" for snippet and stdin input.
	Source string `json:"source"`
	// Module is the Go module path of the generated project.
	Module string `json:"module"`
	// OutputDir is where the bundle was written, empty in --stdout mode.
	OutputDir string `json:"output_dir,omitempty"`
	// Version is the perl2go version that produced this.
	Version string `json:"version"`

	Entries  []Entry  `json:"entries"`
	Symbols  []Symbol `json:"symbols"`
	Stats    Stats    `json:"stats"`
	Concepts []string `json:"concepts"`

	// Verified records whether the generated Go was checked, and how far.
	Verified Verification `json:"verified"`
}

// Verification records how thoroughly the tool checked its own output.
type Verification struct {
	// Parsed is true when go/parser accepted every emitted file.
	Parsed bool `json:"parsed"`
	// Built is true when a real Go toolchain compiled the output. It is false
	// when no toolchain was available, which is not a failure.
	Built bool `json:"built"`
	// Toolchain is true when a Go toolchain was found at all.
	Toolchain bool   `json:"toolchain"`
	Error     string `json:"error,omitempty"`
}

// Add appends an entry and keeps the counters in step.
func (r *Report) Add(e Entry) {
	e.SeverityName = e.Severity.String()
	r.Entries = append(r.Entries, e)
	switch e.Severity {
	case Warn:
		r.Stats.Approximated++
	case Refuse:
		r.Stats.Refused++
	}
	for _, c := range e.Concepts {
		r.AddConcept(c)
	}
}

// AddConcept records a triggered teaching concept, keeping first-trigger order
// and ignoring repeats.
func (r *Report) AddConcept(id string) {
	if id == "" {
		return
	}
	for _, have := range r.Concepts {
		if have == id {
			return
		}
	}
	r.Concepts = append(r.Concepts, id)
}

// Clean reports whether the conversion produced no warnings and no refusals.
func (r *Report) Clean() bool {
	return r.Stats.Approximated == 0 && r.Stats.Refused == 0 && r.Stats.ParseErrors == 0
}

// DynamicRate returns the share of symbols whose type inference fell back to a
// dynamic representation, in the range 0 to 1.
func (r *Report) DynamicRate() float64 {
	if r.Stats.Symbols == 0 {
		return 0
	}
	return float64(r.Stats.Symbols-r.Stats.SymbolsTyped) / float64(r.Stats.Symbols)
}

// SortEntries orders entries by source position, then by code, which is the
// order a developer reads their own file in.
func (r *Report) SortEntries() {
	sort.SliceStable(r.Entries, func(i, j int) bool {
		a, b := r.Entries[i], r.Entries[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return a.Code < b.Code
	})
}

// BySeverity returns the entries at one severity, in report order.
func (r *Report) BySeverity(s Severity) []Entry {
	var out []Entry
	for _, e := range r.Entries {
		if e.Severity == s {
			out = append(out, e)
		}
	}
	return out
}
