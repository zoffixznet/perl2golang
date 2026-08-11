package parser_test

import (
	"testing"
	"time"

	"perl2golang/internal/perl/parser"
)

// TestParseAlwaysTerminates covers inputs that once sent the parser into a
// loop that never consumed another token. Each case is run under its own
// deadline so a regression fails in seconds instead of hanging the suite.
//
// The first case is distilled from a real installed module
// (Locale::RecodeData::IBM862): a trailing comma inside an if condition is
// legal Perl, and the recovery it forces left a block open above the __END__
// marker, which parseBlockBody then asked for statements past forever.
func TestParseAlwaysTerminates(t *testing.T) {
	inputs := []string{
		"if (1,) { print \"x\\n\"; }\n__END__\n",
		"if ($x->{a} eq 'y',) {\n\tprint \"x\\n\";\n}\n1;\n__END__\nleftover text\n",
		"sub f {\n__END__\n",
		"{\n__END__\n",
	}
	for _, in := range inputs {
		done := make(chan struct{})
		go func() {
			defer close(done)
			parser.Parse([]byte(in))
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("the parser never finished with %q", in)
		}
	}
}
