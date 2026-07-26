package lower

import (
	"testing"

	"perl2go/internal/ir"
	"perl2go/internal/perl/parser"
)

func TestGoName(t *testing.T) {
	tests := []struct {
		perl string
		want string
	}{
		{"count", "count"},
		{"total_bytes", "totalBytes"},
		{"$total_bytes", "totalBytes"},
		{"@line_items", "lineItems"},
		{"_private", "private"},
		{"Foo::Bar::baz", "baz"},
		{"HTML_text", "htmlText"},
		{"range", "rangeVal"}, // a Go keyword
		{"len", "lenVal"},     // a Go builtin
		{"2fast", "v2fast"},   // cannot start with a digit
		{"", "v"},
	}
	for _, tt := range tests {
		if got := goName(tt.perl); got != tt.want {
			t.Errorf("goName(%q) = %q, want %q", tt.perl, got, tt.want)
		}
	}
}

func TestNameSetHandsOutUniqueNames(t *testing.T) {
	ns := newNameSet()
	ns.reserve("main")
	got := []string{ns.take("x"), ns.take("x"), ns.take("x"), ns.take("main")}
	want := []string{"x", "x2", "x3", "main2"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("take %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name string
		a, b *ir.Type
		want *ir.Type
	}{
		{"nothing with int", nil, ir.TInt, ir.TInt},
		{"int with int", ir.TInt, ir.TInt, ir.TInt},
		{"int widens to float", ir.TInt, ir.TFloat, ir.TFloat},
		{"float widens from int", ir.TFloat, ir.TInt, ir.TFloat},
		{"int and text have no common type", ir.TInt, ir.TString, ir.TAny},
		{"void carries no information", ir.TVoid, ir.TString, ir.TString},
		{"slices join elementwise", ir.SliceOf(ir.TInt), ir.SliceOf(ir.TFloat), ir.SliceOf(ir.TFloat)},
		{"maps join elementwise", ir.MapOf(ir.TInt), ir.MapOf(ir.TString), ir.MapOf(ir.TAny)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := join(tt.a, tt.b); !got.Equal(tt.want) {
				t.Errorf("join = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestLowerInfersTypes checks the whole inference path on small programs: the
// type a variable ends up with is the one every use of it agrees on.
func TestLowerInfersTypes(t *testing.T) {
	tests := []struct {
		name string
		perl string
		vari string
		want string
	}{
		{"integer literal", `my $n = 3;  print $n;`, "$n", "int"},
		{"widened by a float", "my $n = 3;\n$n += 0.5;\nprint $n;", "$n", "float64"},
		{"text", `my $s = "hi"; print $s;`, "$s", "string"},
		{"division always yields a float", "my $r = 7 / 2;\nprint $r;", "$r", "float64"},
		{"array of text", `my @a = ("x", "y"); print "@a";`, "@a", "[]string"},
		{"hash of counts", "my %c;\n$c{a}++;\nprint $c{a};", "%c", "map[string]int"},
		{"mixed uses fall back", "my $x = 1;\n$x = \"two\";\nprint $x;", "$x", "any"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.perl)
			res := Lower(parser.Parse(src), src, Options{File: "t.pl", Program: "t", Module: "t"})
			for _, sym := range res.Report.Symbols {
				if sym.Name == tt.vari {
					if sym.Type != tt.want {
						t.Errorf("%s inferred as %s, want %s", tt.vari, sym.Type, tt.want)
					}
					return
				}
			}
			t.Errorf("no symbol named %s in the report", tt.vari)
		})
	}
}

// TestLowerReportsRefusals checks that a construct with no Go counterpart is
// reported rather than quietly dropped.
func TestLowerReportsRefusals(t *testing.T) {
	tests := []struct {
		name string
		perl string
		code string
	}{
		{"local", `our $x; sub f { local $x = 1; }`, "P2G2001"},
		{"wantarray", `sub f { return wantarray ? 1 : 2; }`, "P2G2031"},
		{"lookahead", `my $s = "x"; print 1 if $s =~ /(?=foo)/;`, "P2G4004"},
		{"backreference", `my $s = "x"; print 1 if $s =~ /(a)\1/;`, "P2G4001"},
		{"bless", `my $o = bless {}, "C";`, "P2G7001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.perl)
			res := Lower(parser.Parse(src), src, Options{File: "t.pl", Program: "t", Module: "t"})
			for _, e := range res.Report.Entries {
				if e.Code == tt.code {
					if e.Message == "" || e.Advice == "" {
						t.Errorf("%s has no message or no advice", tt.code)
					}
					return
				}
			}
			t.Errorf("no entry with code %s; got %v", tt.code, codesOf(res))
		})
	}
}

func codesOf(res *Result) []string {
	var out []string
	for _, e := range res.Report.Entries {
		out = append(out, e.Code)
	}
	return out
}

// TestLowerNeverPanics runs a set of deliberately awkward inputs through the
// whole pass. A panic here is a defect in the tool, not a problem with the
// input, so the test exists to catch one early.
func TestLowerNeverPanics(t *testing.T) {
	inputs := []string{
		"",
		";;;",
		"my",
		"my $x =",
		"sub {",
		"print",
		"if (1) {",
		"my @a = (1,2,3)[0];",
		"my %h; $h{a}{b}{c} = 1;",
		"for (;;) { last }",
		"my $s = <<EOT;\nbody\nEOT\n",
		"$_ = 1; s/a/b/; tr/a/b/;",
		"my @x = map { $_ } grep { $_ } sort { $a <=> $b } (3,1,2);",
	}
	for _, in := range inputs {
		src := []byte(in)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on %q: %v", in, r)
				}
			}()
			Lower(parser.Parse(src), src, Options{File: "t.pl", Program: "t", Module: "t"})
		}()
	}
}
