package lower

import (
	"strings"
	"testing"

	"perl2golang/internal/gogen"
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/parser"
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
		// Text beside numbers resolves to the string form every Perl scalar
		// carries, with the numbers converted where they are assigned.
		{"mixed scalars resolve to text", "my $x = 1;\n$x = \"two\";\nprint $x;", "$x", "string"},
		// A number beside a hash has no honest string form, so it stays
		// dynamic.
		{"mixed shapes fall back", "my $x = 1;\n$x = {a => 2};\nprint $x;", "$x", "any"},
		// A list assignment types its targets by position, not by the join
		// of everything on the right: the number stays a number however
		// text-like the tail is.
		{"list assignment head is positional",
			"my @names = (\"a\", \"b\");\nmy ($best, @pair) = (-1);\n($best, @pair) = (2.5, @names);\nprint $best, \"@pair\";",
			"$best", "float64"},
		{"list assignment tail is positional",
			"my @names = (\"a\", \"b\");\nmy ($best, @pair) = (-1);\n($best, @pair) = (2.5, @names);\nprint $best, \"@pair\";",
			"@pair", "[]string"},
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

// TestCompactEvidence covers the rule that lets discovery rounds change
// their minds: for every site, only the latest round with a definite answer
// keeps its say, and a round that answered `any` does not erase the round
// that answered.
func TestCompactEvidence(t *testing.T) {
	siteA, siteB := keyNode{"a"}, keyNode{"b"}
	obs := func(t *ir.Type, site keyNode, round int) observation {
		return observation{t: t, site: site, round: round}
	}
	tests := []struct {
		name string
		in   []observation
		want []*ir.Type
	}{
		{"a later round replaces an earlier one at the same site",
			[]observation{obs(ir.TInt, siteA, 0), obs(ir.TString, siteA, 1)},
			[]*ir.Type{ir.TString}},
		{"an any round does not erase the round that answered",
			[]observation{obs(ir.TString, siteA, 0), obs(ir.TAny, siteA, 1)},
			[]*ir.Type{ir.TString}},
		{"a site no round revisits keeps its old word",
			[]observation{obs(ir.TInt, siteA, 0), obs(ir.TString, siteB, 2)},
			[]*ir.Type{ir.TInt, ir.TString}},
		{"several observations in one round all survive",
			[]observation{obs(ir.TInt, siteA, 1), obs(ir.TFloat, siteA, 1)},
			[]*ir.Type{ir.TInt, ir.TFloat}},
		{"a site that only ever said any keeps one round of it",
			[]observation{obs(ir.TAny, siteA, 0), obs(ir.TAny, siteA, 1)},
			[]*ir.Type{ir.TAny}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := observedTypes(compactEvidence(tt.in))
			if len(got) != len(tt.want) {
				t.Fatalf("kept %d observations, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if !got[i].Equal(tt.want[i]) {
					t.Errorf("observation %d = %s, want %s", i, got[i], tt.want[i])
				}
			}
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
		{"array-backed class", `package V; sub new { bless [1], shift } package main; my $o = V->new; print 1;`, "P2G7051"},
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

// TestNoStatementVanishesSilently covers the safety net under the lowering: a
// statement that produces no code must leave a diagnostic, because a statement
// that simply disappears is a silent change of meaning.
//
// The statement used here is the last index of an anonymous array, read and
// thrown away, which no lowering rule produces anything for. If a later round
// teaches the lowering a rule for it, this test needs another statement the
// lowering cannot translate; weakening it instead would leave the net
// untested.
func TestNoStatementVanishesSilently(t *testing.T) {
	src := []byte("my $x = 1;\n$#{[]};\nprint \"$x\\n\";\n")
	res := Lower(parser.Parse(src), src, Options{File: "t.pl", Program: "t", Module: "t"})
	for _, e := range res.Report.Entries {
		if e.Code == "P2G3598" {
			if e.Line != 2 {
				t.Errorf("the vanished statement was reported at line %d, want 2", e.Line)
			}
			return
		}
	}
	t.Errorf("a statement lowered to nothing and nothing was reported; got %v", codesOf(res))
}

// TestEveryCloseSaysSomething covers the four closes, which look identical in
// Perl and are four different operations underneath. Each has to leave code
// behind: a close that lowered to nothing is a file left unflushed or a child
// status never collected, and neither failure announces itself.
func TestEveryCloseSaysSomething(t *testing.T) {
	tests := []struct {
		name string
		perl string
		want string
	}{
		{"a file", "open my $fh, '<', 'f' or die; close $fh;", "fh.Close()"},
		{"a pipe", "open my $p, '-|', 'ls' or die; my @l = <$p>; close $p;", "p.Close()"},
		{"the status a pipe close leaves",
			"open my $p, '-|', 'ls' or die; my @l = <$p>; close $p; print $? >> 8;",
			"= p.Status()"},
		{"standard output", "close STDOUT;", "os.Stdout.Close()"},
		{"the diamond's own handle", "while (<>) { print; close ARGV if eof; }", "lineNo = 0"},
		{"a handle of no fixed type", "my $x = 1; close $x;", "closeHandle(x)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.perl)
			res := Lower(parser.Parse(src), src, Options{File: "t.pl", Program: "t", Module: "t"})
			var got strings.Builder
			for _, st := range res.TopLevel {
				got.WriteString(gogen.RenderStmt(gogen.Clean, st))
				got.WriteString("\n")
			}
			if !strings.Contains(got.String(), tt.want) {
				t.Errorf("the close did not emit %q; got:\n%s", tt.want, got.String())
			}
			for _, e := range res.Report.Entries {
				if e.Code == "P2G3598" {
					t.Errorf("a statement vanished at line %d: %q", e.Line, e.Perl)
				}
			}
		})
	}
}

// TestHonestVanishingIsNotFlagged covers the statements that are allowed to
// lower to nothing: declarations hoisted to package level, a module's trailing
// `1;`, and sub and package declarations, which are lowered elsewhere.
func TestHonestVanishingIsNotFlagged(t *testing.T) {
	src := []byte("package M;\nmy %memo;\nsub f { $memo{a} = 1; }\n1;\n")
	res := Lower(parser.Parse(src), src, Options{File: "t.pl", Program: "t", Module: "t"})
	for _, e := range res.Report.Entries {
		if e.Code == "P2G3598" {
			t.Errorf("a statement that may lower to nothing was flagged: %q at line %d", e.Perl, e.Line)
		}
	}
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
		// bump returns $self, and Counter has a subclass: the receiver's
		// declared type would hand a Loud back as its embedded Counter, so
		// the method returns the whole object through the hierarchy's
		// interface instead.
		"func (c *Counter) Bump(by int) counterSelf",
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
