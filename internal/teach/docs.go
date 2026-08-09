package teach

import "perl2golang/internal/report"

// This file defines the contract between the converter and the document
// generator. The converter hands over everything it learned while translating
// one file; the generator turns that into the Markdown bundle that ships beside
// the generated code.

// Segment is one region of the walkthrough: a piece of the developer's Perl,
// the Go it became, and the explanation of why it is written that way.
type Segment struct {
	// Title names the region, for example "Reading the input file".
	Title string
	// PerlFrom and PerlTo are 1-based inclusive line numbers in the original.
	PerlFrom int
	PerlTo   int
	// Perl is the original source text of the region.
	Perl string
	// Go is the generated Go for the region, already formatted.
	Go string
	// Explain is the prose tying the two together, one paragraph per remark.
	// When Notes is set it carries the same remarks; Explain remains for
	// consumers that want the flat text, such as the REPL.
	Explain string
	// Notes are the remarks with the position each one is about, so a
	// document can anchor every remark to the line it explains. Optional:
	// when empty, Explain is the only account of the region.
	Notes []SegmentNote
	// Concepts are teaching concept ids relevant to this region.
	Concepts []string
}

// SegmentNote is one remark about a region, tied to the construct that raised
// it. A region of any size collects many remarks, and without the position a
// reader looking for the explanation of one line has to scan all of them.
type SegmentNote struct {
	// Line is the 1-based line in the original, 0 when unknown.
	Line int
	// Perl is the first line of the construct the remark is about, trimmed.
	Perl string
	// Text is the remark itself.
	Text string
}

// Exercise is a checkable task against the generated code.
type Exercise struct {
	Title string
	// Task is what to do, phrased against the developer's own code.
	Task string
	// Success is how to tell it worked.
	Success string
	// Concepts are the ids the exercise practises.
	Concepts []string
}

// DocInput is everything the document generator needs about one conversion.
type DocInput struct {
	// ScriptName is the input file's base name, for example "report.pl".
	ScriptName string
	// ProgramName is the generated program's name, for example "report".
	ProgramName string
	// Module is the generated module path.
	Module string
	// PerlSource is the complete original source.
	PerlSource string
	// GoSource is the clean generated program.
	GoSource string
	// Report is the honest account of the conversion.
	Report *report.Report
	// Concepts are the triggered concept ids, in first-trigger order.
	Concepts []string
	// Walkthrough is the ordered per-region tour of the file.
	Walkthrough []Segment
	// Exercises are tasks against this specific generated code.
	Exercises []Exercise
	// Version is the perl2golang version string.
	Version string
}
