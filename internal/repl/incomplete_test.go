package repl

import "testing"

// TestContinuation is the decision the multi-line prompt rests on. Get it
// wrong in one direction and a finished statement never converts; get it wrong
// in the other and half a statement is handed to the converter and reported as
// a mistake the user did not make.
func TestContinuation(t *testing.T) {
	tests := []struct {
		name string
		src  string
		open bool
		what string
	}{
		{name: "a finished statement", src: "my $x = 1;"},
		{name: "a statement without its semicolon", src: "my $x = 1"},
		{name: "several finished statements", src: "my $x = 1; my $y = 2;"},
		{name: "a finished sub", src: "sub f { return 1 }"},
		{name: "a forward declaration", src: "sub f;"},
		{name: "a statement modifier", src: `print "x" if $y;`},
		{name: "a postfix loop", src: "$seen{$_}++ for @nums;"},
		{name: "a do block with a while modifier", src: "do { $i++ } while ($i < 3);"},
		{name: "an if with its block", src: `if ($x > 1) { print "big" }`},
		{name: "an else chain", src: "if ($x) { 1 } else { 2 }"},
		{name: "a hash literal", src: "my %h = (a => 1, b => 2);"},
		{name: "a nested structure", src: "my $r = { list => [1, 2], name => 'x' };"},
		{name: "a regex holding braces", src: `my @m = /x{2,3}/;`},
		{name: "a string holding a brace", src: `my $s = "{";`},
		{name: "a comment only", src: "# nothing here"},
		{name: "nothing at all", src: ""},

		{name: "an open paren", src: "my @a = (1,", open: true, what: "parenthesis"},
		{name: "an open bracket", src: "my $r = [1,", open: true, what: "bracket"},
		{name: "an open sub body", src: "sub trim {", open: true, what: "sub body"},
		{name: "an open if block", src: "if ($x > 1) {", open: true, what: "if block"},
		{name: "an open foreach block", src: "foreach my $n (@nums) {", open: true, what: "foreach block"},
		{name: "an open anonymous hash", src: "my $r = {", open: true, what: "block"},
		{name: "an if head with no block yet", src: "if ($x > 1)", open: true, what: "if statement"},
		{name: "a sub name with no block yet", src: "sub trim", open: true, what: "sub declaration"},
		{name: "a while head with no block yet", src: "while (my $l = <$fh>)", open: true, what: "while statement"},
		{name: "a trailing operator", src: "my $x = 1 +", open: true, what: "an operator with nothing after it"},
		{name: "a trailing comma", src: "my @a = 1,", open: true, what: "an operator with nothing after it"},
		{name: "a trailing fat comma", src: "my %h = (a =>", open: true, what: "parenthesis"},
		{name: "a trailing concatenation", src: `my $s = "a" .`, open: true, what: "an operator with nothing after it"},
		{name: "a trailing arrow", src: "my $v = $r->", open: true, what: "an operator with nothing after it"},
		{name: "an unterminated string", src: `my $s = "hello`, open: true, what: "string"},
		{name: "an unterminated single-quoted string", src: "my $s = 'hello", open: true, what: "string"},
		{name: "an unterminated pattern", src: "my @m = /abc", open: true, what: "pattern"},
		{name: "an unterminated substitution", src: "$s =~ s/a/b", open: true, what: "substitution"},
		{name: "an unterminated heredoc", src: "my $t = <<EOF;\nhello", open: true, what: "heredoc"},
		{name: "a nested open block", src: "sub f {\n  if ($x) {\n", open: true, what: "sub body"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, open := continuation(tc.src)
			if open != tc.open {
				t.Fatalf("continuation(%q) open = %v, want %v (%+v)", tc.src, open, tc.open, got)
			}
			if open && got.what != tc.what {
				t.Errorf("continuation(%q) what = %q, want %q", tc.src, got.what, tc.what)
			}
		})
	}
}

// TestContinuationReportsTheOpeningLine matters because the reminder the
// prompt prints after a few lines is only useful if it points at the right one.
func TestContinuationReportsTheOpeningLine(t *testing.T) {
	got, open := continuation("my $x = 1;\nsub trim {\n  my $s = shift;")
	if !open {
		t.Fatal("a half-written sub should keep reading")
	}
	if got.line != 2 {
		t.Errorf("open construct is on line %d, want 2", got.line)
	}
}

// TestContinuationTerminates guards the loop: any input at all must produce an
// answer rather than hang or panic, because the prompt calls this on every
// keystroke's worth of line.
func TestContinuationTerminates(t *testing.T) {
	for _, src := range []string{
		"}", ")", "]", "))))", "{{{{", "$", "@", "%", "\x00", "s/", "q", "<<",
		"__END__", "=pod\nnothing", "sub", "if", "for", "1", ";",
	} {
		if _, _, ok := recovered(func() { continuation(src) }); !ok {
			t.Errorf("continuation(%q) panicked", src)
		}
	}
}

// recovered runs f and reports whether it returned normally.
func recovered(f func()) (any, string, bool) {
	ok := false
	var r any
	func() {
		defer func() { r = recover() }()
		f()
		ok = true
	}()
	return r, "", ok
}

func TestPresentableStripsTheWrapper(t *testing.T) {
	const src = `package main

import "fmt"

// main is the program's entry point.
func main() {
	x := 1
	fmt.Println(x)
	_ = x
}

// f performs one step of the program's work.
func f() int {
	return 1
}
`
	got := presentable(src)
	want := []string{
		"x := 1",
		"fmt.Println(x)",
		"// f performs one step of the program's work.",
		"func f() int {",
		"\treturn 1",
		"}",
	}
	if len(got) != len(want) {
		t.Fatalf("presentable gave %d lines, want %d:\n%q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDeltaOf(t *testing.T) {
	tests := []struct {
		name    string
		prev    []string
		next    []string
		added   []string
		removed int
	}{
		{
			name:  "the first snippet",
			next:  []string{"a", "b"},
			added: []string{"a", "b"},
		},
		{
			name:  "an append",
			prev:  []string{"a"},
			next:  []string{"a", "b"},
			added: []string{"b"},
		},
		{
			name:    "a line rewritten in place",
			prev:    []string{"a", "b"},
			next:    []string{"a", "B", "c"},
			added:   []string{"B", "c"},
			removed: 1,
		},
		{
			name:    "an insertion above",
			prev:    []string{"b"},
			next:    []string{"a", "b"},
			added:   []string{"a"},
			removed: 0,
		},
		{
			name: "nothing changed",
			prev: []string{"a", "b"},
			next: []string{"a", "b"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			added, removed := deltaOf(tc.prev, tc.next)
			if removed != tc.removed {
				t.Errorf("removed = %d, want %d", removed, tc.removed)
			}
			if len(added) != len(tc.added) {
				t.Fatalf("added = %q, want %q", added, tc.added)
			}
			for i := range added {
				if added[i] != tc.added[i] {
					t.Errorf("added[%d] = %q, want %q", i, added[i], tc.added[i])
				}
			}
		})
	}
}
