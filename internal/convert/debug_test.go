package convert_test

import (
	"os"
	"strings"
	"testing"

	"perl2go/internal/gogen"
	"perl2go/internal/lower"
	"perl2go/internal/perl/parser"
)

// TestDebugEmit prints the raw emitted text for one corpus entry, formatted or
// not, so a failure to parse can be read. It is skipped unless PERL2GO_DEBUG
// names an input file.
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
