package parser_test

import (
	"os"
	"testing"

	"perl2go/internal/perl/parser"
)

// TestDiagFile is a debugging aid: point PERL2GO_PARSE at a Perl file and run
// this test with -v to see every diagnostic the parser produces for it.
func TestDiagFile(t *testing.T) {
	path := os.Getenv("PERL2GO_PARSE")
	if path == "" {
		t.Skip("set PERL2GO_PARSE to a Perl file to list its parse diagnostics")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res := parser.Parse(src)
	for _, d := range res.Diags {
		t.Logf("diag %s", d.Error())
	}
}
