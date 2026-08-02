package lower

import (
	"strings"
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
		{"bless on an array", `my $o = bless [1, 2], "C"; print $o->[0];`, "P2G7001"},
		{"overload", `package M; use overload '+' => sub { 1 }; sub new { bless {}, shift }`, "P2G7025"},
		{"AUTOLOAD", `package M; sub new { bless {}, shift } our $AUTOLOAD; sub AUTOLOAD { 1 }`, "P2G7035"},
		{"DESTROY", `package M; sub new { bless {}, shift } sub DESTROY { my $s = shift; $s->{n} }`, "P2G7030"},
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

// TestStubTypeIsSpellableWithoutAnImport covers the type argument of the
// stand-in a refusal becomes. The type is written into the source as text, and
// a type needing an import that nothing registered would produce Go that does
// not compile, which is a worse outcome than the any it started with.
func TestStubTypeIsSpellableWithoutAnImport(t *testing.T) {
	tests := []struct {
		name string
		in   *ir.Type
		want string
	}{
		{"nothing at all", nil, "any"},
		{"a whole number", ir.TInt, "int"},
		{"text", ir.TString, "string"},
		{"a list of text", ir.SliceOf(ir.TString), "[]string"},
		{"a keyed collection", ir.MapOf(ir.TInt), "map[string]int"},
		{"a local type", ir.NamedType("Record", ""), "Record"},
		{"a type from another package", ir.NamedType("os.File", "os"), "any"},
		{"a list of them", ir.SliceOf(ir.NamedType("time.Time", "time")), "any"},
		{"the absence of a value", ir.TVoid, "any"},
		{"a function", ir.FuncOf(nil, nil), "any"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stubType(tt.in).Go(nil); got != tt.want {
				t.Errorf("stubType(%s) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestARefusedExpressionBecomesOneReadableCall pins the stand-in's shape at the
// point it is built, so a change to it has to be deliberate.
func TestARefusedExpressionBecomesOneReadableCall(t *testing.T) {
	res := Lower(parser.Parse([]byte("my $x = $obj->name;\n")), []byte("my $x = $obj->name;\n"), Options{File: "t.pl"})
	found := false
	for _, name := range res.Helpers {
		if name == hNotImplemented {
			found = true
		}
	}
	if !found {
		t.Errorf("the stand-in helper was not requested; helpers are %v", res.Helpers)
	}
	if res.Report.Stats.Refused == 0 {
		t.Error("a refused construct was not counted")
	}
}

// TestLowerBuildsClasses checks the shape a package plus bless comes out as:
// one struct per package, fields named after the hash keys, methods on a
// pointer receiver, and an accessor replaced by the field it read.
func TestLowerBuildsClasses(t *testing.T) {
	const perl = `
package Counter;
sub new { my ($class, %args) = @_; return bless { name => $args{name}, n => 0 }, $class }
sub name { $_[0]{name} }
sub bump { my ($self, $by) = @_; $self->{n} = $self->{n} + $by; return $self }
package Loud;
our @ISA = ('Counter');
sub shout { my ($self) = @_; return uc $self->{name} }
package main;
my $c = Counter->new(name => 'hits');
$c->bump(2);
print $c->name, "\n";
`
	src := []byte(perl)
	res := Lower(parser.Parse(src), src, Options{File: "t.pl", Program: "t", Module: "t"})
	got := renderDecls(res)

	for _, want := range []string{
		"type Counter struct",
		"Name string",
		"N int",
		"func NewCounter(name string) *Counter",
		"func (c *Counter) Bump(by int) *Counter",
		"type Loud struct",
		"Counter",                // the parent is embedded, not copied field by field
		"func (l *Loud) Shout()", // a subclass method of its own
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated declarations do not contain %q\n%s", want, got)
		}
	}
	// A sub that only reads one key is not a method at all: the field took
	// its place, and emitting both would not compile.
	if strings.Contains(got, "func (c *Counter) Name(") {
		t.Errorf("the accessor was emitted as a method as well as a field\n%s", got)
	}
}

// renderDecls prints a lowered program's declarations for a test to match
// against, without pulling the emitter into this package's tests.
func renderDecls(res *Result) string {
	var sb strings.Builder
	for _, f := range res.Program.Files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ir.TypeDecl:
				sb.WriteString("type " + d.Name + " struct {\n")
				for _, fl := range d.Fields {
					sb.WriteString("\t" + fl.Name + " " + fl.Type.String() + "\n")
				}
				sb.WriteString("}\n")
			case *ir.FuncDecl:
				sb.WriteString("func ")
				if d.Recv != nil {
					sb.WriteString("(" + d.Recv.Name + " " + d.Recv.Type.String() + ") ")
				}
				sb.WriteString(d.Name + "(")
				for i, p := range d.Params {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(p.Name + " " + p.Type.String())
				}
				sb.WriteString(")")
				for _, r := range d.Results {
					sb.WriteString(" " + r.String())
				}
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}
