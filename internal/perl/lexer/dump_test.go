package lexer_test

import (
	"fmt"
	"os"
	"testing"

	"perl2golang/internal/perl/lexer"
)

func fmtSscan(s string, n *int) (int, error) { return fmt.Sscan(s, n) }

// TestDump is a debugging aid: point PERL2GOLANG_DUMP at a Perl file and run this
// test with -v to see the token stream and any lexical diagnostics.
func TestDump(t *testing.T) {
	path := os.Getenv("PERL2GOLANG_DUMP")
	if path == "" {
		t.Skip("set PERL2GOLANG_DUMP to a Perl file to dump its tokens")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	toks, diags := lexer.Lex(src)
	for _, d := range diags {
		t.Logf("diag %s", d)
	}
	from, to := 0, len(toks)
	if s := os.Getenv("PERL2GOLANG_DUMP_FROM"); s != "" {
		var n int
		if _, err := fmtSscan(s, &n); err == nil && n < len(toks) {
			from = n
		}
	}
	if s := os.Getenv("PERL2GOLANG_DUMP_TO"); s != "" {
		var n int
		if _, err := fmtSscan(s, &n); err == nil && n <= len(toks) {
			to = n
		}
	}
	for i := from; i < to; i++ {
		t.Logf("%4d %s", i, toks[i])
	}
}
