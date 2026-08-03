package convert_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"perl2golang/internal/convert"
	"perl2golang/internal/gogen"
	"perl2golang/internal/lower"
	"perl2golang/internal/perl/parser"
	"perl2golang/internal/project"
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

// TestDebugCompile reports how many corpus entries in a tier produce Go that
// the real toolchain accepts, and why the rest do not. Set PERL2GO_TIER.
func TestDebugCompile(t *testing.T) {
	tier := os.Getenv("PERL2GO_TIER")
	if tier == "" {
		t.Skip("set PERL2GO_TIER to a corpus tier")
	}
	paths := corpusFiles(t, tier, 0)
	built := 0
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		res, err := convert.Convert(src, convert.Options{Path: path, NoDocs: true})
		if err != nil {
			t.Logf("FAIL   %s: %v", path, err)
			continue
		}
		if res.Report.Verified.Built {
			built++
			continue
		}
		first := strings.SplitN(res.Report.Verified.Error, "\n", 3)
		t.Logf("NOBUILD %s: %s", filepath.Base(filepath.Dir(path)), strings.Join(first, " | "))
	}
	t.Logf("%s: %d of %d compile", tier, built, len(paths))
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
