package convert_test

import (
	"os"
	"strings"
	"testing"

	"perl2go/internal/convert"
	"perl2go/internal/gogen"
	"perl2go/internal/lower"
	"perl2go/internal/perl/parser"
	"perl2go/internal/project"
)

// TestDebugEmit prints the raw emitted text for one input, formatted or not, so
// a failure to parse can be read. Set PERL2GO_DEBUG to the input path.
func TestDebugEmit(t *testing.T) {
	path := os.Getenv("PERL2GO_DEBUG")
	if path == "" {
		t.Skip("set PERL2GO_DEBUG to an input.pl path")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	low := lower.Lower(parser.Parse(src), src, lower.Options{File: path, Program: "dbg", Module: "dbg"})
	out, emitErr := gogen.New(gogen.Clean).File(low.Program.Files[0])
	if emitErr != nil {
		t.Logf("emit error: %v", emitErr)
	}
	for i, line := range strings.Split(string(out), "\n") {
		t.Logf("%4d| %s", i+1, line)
	}
}

// TestDebugBundle writes a complete conversion bundle somewhere it can be read.
// Set PERL2GO_DEBUG to the input path and PERL2GO_OUT to the target directory.
func TestDebugBundle(t *testing.T) {
	path, out := os.Getenv("PERL2GO_DEBUG"), os.Getenv("PERL2GO_OUT")
	if path == "" || out == "" {
		t.Skip("set PERL2GO_DEBUG and PERL2GO_OUT")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := convert.Convert(src, convert.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if err := project.Write(out, res.Bundle(), true); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d files to %s (parsed=%v built=%v toolchain=%v)",
		len(res.Bundle()), out, res.Report.Verified.Parsed,
		res.Report.Verified.Built, res.Report.Verified.Toolchain)
	t.Logf("entries=%d concepts=%d walkthrough=%d",
		len(res.Report.Entries), len(res.Report.Concepts), len(res.Walkthrough))
}
