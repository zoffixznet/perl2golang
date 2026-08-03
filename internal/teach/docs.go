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
	// Explain is the prose tying the two together.
	Explain string
	// Concepts are teaching concept ids relevant to this region.
	Concepts []string
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
