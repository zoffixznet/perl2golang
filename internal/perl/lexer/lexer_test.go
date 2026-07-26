package lexer

import (
	"fmt"
	"strings"
	"testing"

	"perl2go/internal/perl/token"
)

// stream renders the significant tokens of src as "Kind" or "Kind(text)"
// strings, which keeps the test tables readable.
func stream(t *testing.T, src string) []string {
	t.Helper()
	toks, _ := Lex([]byte(src))
	var out []string
	for _, tk := range toks {
		switch tk.Kind {
		case token.EOF, token.Comment, token.Pod:
			continue
		}
		switch tk.Kind {
		case token.Ident, token.ScalarVar, token.ArrayVar, token.HashVar,
			token.FuncVar, token.GlobVar, token.ArrayLen, token.Number,
			token.Version, token.FileTest, token.Readline, token.Glob:
			out = append(out, fmt.Sprintf("%s(%s)", tk.Kind, tk.Text))
		default:
			out = append(out, tk.Kind.String())
		}
	}
	return out
}

func expectStream(t *testing.T, src string, want ...string) {
	t.Helper()
	got := stream(t, src)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("lex %q:\n got: %s\nwant: %s", src, strings.Join(got, " "), strings.Join(want, " "))
	}
}

// firstOfKind returns the first token of the given kind.
func firstOfKind(t *testing.T, src string, kind token.Kind) token.Token {
	t.Helper()
	toks, _ := Lex([]byte(src))
	for _, tk := range toks {
		if tk.Kind == kind {
			return tk
		}
	}
	t.Fatalf("no %s token in %q", kind, src)
	return token.Token{}
}

func TestSlashDisambiguation(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"division after scalar", `my $r = $a / $b;`,
			[]string{"Ident(my)", "ScalarVar($r)", "Assign", "ScalarVar($a)", "Slash", "ScalarVar($b)", "Semi"}},
		{"division no spaces", `my $r = $a/$b;`,
			[]string{"Ident(my)", "ScalarVar($r)", "Assign", "ScalarVar($a)", "Slash", "ScalarVar($b)", "Semi"}},
		{"match after bind", `"aaa" =~ /a/g;`,
			[]string{"StrDouble", "MatchBind", "Match", "Semi"}},
		{"match after assign", `my $m = /x/;`,
			[]string{"Ident(my)", "ScalarVar($m)", "Assign", "Match", "Semi"}},
		{"match after return", `return /y/;`,
			[]string{"Ident(return)", "Match", "Semi"}},
		{"match after split", `split /,/, $s;`,
			[]string{"Ident(split)", "Match", "Comma", "ScalarVar($s)", "Semi"}},
		{"match in grep block", `grep { /err/ } @lines;`,
			[]string{"Ident(grep)", "LBrace", "Match", "RBrace", "ArrayVar(@lines)", "Semi"}},
		{"match after and", `$x and /z/;`,
			[]string{"ScalarVar($x)", "AndLow", "Match", "Semi"}},
		{"match after open paren", `if (/q/) { }`,
			[]string{"Ident(if)", "LParen", "Match", "RParen", "LBrace", "RBrace"}},
		{"division after paren close", `my $d = ($a + $b) / 2;`,
			[]string{"Ident(my)", "ScalarVar($d)", "Assign", "LParen", "ScalarVar($a)", "Plus",
				"ScalarVar($b)", "RParen", "Slash", "Number(2)", "Semi"}},
		{"division after subscript", `my $d = $n[0] / 2;`,
			[]string{"Ident(my)", "ScalarVar($d)", "Assign", "ScalarVar($n)", "LBracket",
				"Number(0)", "RBracket", "Slash", "Number(2)", "Semi"}},
		{"division after bareword call", `my $z = f / 2;`,
			[]string{"Ident(my)", "ScalarVar($z)", "Assign", "Ident(f)", "Slash", "Number(2)", "Semi"}},
		{"defined-or", `my $v = $x // 5;`,
			[]string{"Ident(my)", "ScalarVar($v)", "Assign", "ScalarVar($x)", "DefinedOr", "Number(5)", "Semi"}},
		{"defined-or assign", `$x //= 5;`,
			[]string{"ScalarVar($x)", "OpAssign", "Number(5)", "Semi"}},
		{"empty match at term", `my $c = () = $s =~ //;`,
			[]string{"Ident(my)", "ScalarVar($c)", "Assign", "LParen", "RParen", "Assign",
				"ScalarVar($s)", "MatchBind", "Match", "Semi"}},
		{"slash equals", `$total /= 4;`,
			[]string{"ScalarVar($total)", "OpAssign", "Number(4)", "Semi"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectStream(t, tt.src, tt.want...) })
	}
}

func TestBarewordSlashDiag(t *testing.T) {
	_, diags := Lex([]byte("my $z = f / 2;\n"))
	found := false
	for _, d := range diags {
		if strings.Contains(d.Msg, "ambiguous /") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a prototype-ambiguity diagnostic, got %v", diags)
	}
}

func TestAngleForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"readline lexical", `while (my $l = <$fh>) { }`,
			[]string{"Ident(while)", "LParen", "Ident(my)", "ScalarVar($l)", "Assign",
				"Readline(<$fh>)", "RParen", "LBrace", "RBrace"}},
		{"readline bareword", `my @l = <FH>;`,
			[]string{"Ident(my)", "ArrayVar(@l)", "Assign", "Readline(<FH>)", "Semi"}},
		{"readline stdin", `my $x = <STDIN>;`,
			[]string{"Ident(my)", "ScalarVar($x)", "Assign", "Readline(<STDIN>)", "Semi"}},
		{"diamond", `while (<>) { }`,
			[]string{"Ident(while)", "LParen", "Readline(<>)", "RParen", "LBrace", "RBrace"}},
		{"secure diamond", `while (<<>>) { }`,
			[]string{"Ident(while)", "LParen", "Readline(<<>>)", "RParen", "LBrace", "RBrace"}},
		{"glob", `my @pl = <*.pl>;`,
			[]string{"Ident(my)", "ArrayVar(@pl)", "Assign", "Glob(<*.pl>)", "Semi"}},
		{"less than", `if ($a < $b) { }`,
			[]string{"Ident(if)", "LParen", "ScalarVar($a)", "NumLt", "ScalarVar($b)",
				"RParen", "LBrace", "RBrace"}},
		{"left shift after number", `my $n = 1 << 3;`,
			[]string{"Ident(my)", "ScalarVar($n)", "Assign", "Number(1)", "ShiftLeft", "Number(3)", "Semi"}},
		{"right shift", `my $n = 8 >> 1;`,
			[]string{"Ident(my)", "ScalarVar($n)", "Assign", "Number(8)", "ShiftRight", "Number(1)", "Semi"}},
		{"spaceship", `sort { $a <=> $b } @n;`,
			[]string{"Ident(sort)", "LBrace", "ScalarVar($a)", "NumCmp", "ScalarVar($b)",
				"RBrace", "ArrayVar(@n)", "Semi"}},
		{"shift with space before bareword", `my $n = $x << 2;`,
			[]string{"Ident(my)", "ScalarVar($n)", "Assign", "ScalarVar($x)", "ShiftLeft", "Number(2)", "Semi"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectStream(t, tt.src, tt.want...) })
	}
}

func TestHeredocs(t *testing.T) {
	t.Run("simple double", func(t *testing.T) {
		src := "my $x = <<EOF;\nhello $name\nEOF\nprint $x;\n"
		tk := firstOfKind(t, src, token.Heredoc)
		if tk.Tag != "EOF" || !tk.Interp || tk.Indented {
			t.Errorf("marker fields wrong: %+v", tk)
		}
		if tk.Parts[0] != "hello $name\n" {
			t.Errorf("body = %q", tk.Parts[0])
		}
	})
	t.Run("quoted single", func(t *testing.T) {
		src := "my $x = <<'RAW';\nno $interp here\nRAW\n"
		tk := firstOfKind(t, src, token.Heredoc)
		if tk.Interp {
			t.Error("single-quoted heredoc must not interpolate")
		}
		if tk.Parts[0] != "no $interp here\n" {
			t.Errorf("body = %q", tk.Parts[0])
		}
	})
	t.Run("two on one line", func(t *testing.T) {
		src := "my $b = <<\"ONE\" . \"-mid-\" . <<\"TWO\";\nbody one\nONE\nbody two\nTWO\n"
		toks, diags := Lex([]byte(src))
		if len(diags) != 0 {
			t.Fatalf("diags: %v", diags)
		}
		var hd []token.Token
		for _, tk := range toks {
			if tk.Kind == token.Heredoc {
				hd = append(hd, tk)
			}
		}
		if len(hd) != 2 {
			t.Fatalf("want 2 heredocs, got %d", len(hd))
		}
		if hd[0].Parts[0] != "body one\n" || hd[1].Parts[0] != "body two\n" {
			t.Errorf("bodies = %q, %q", hd[0].Parts[0], hd[1].Parts[0])
		}
	})
	t.Run("same terminator twice", func(t *testing.T) {
		src := "my $s = <<'X' . <<'X';\np\nX\nq\nX\n"
		toks, _ := Lex([]byte(src))
		var bodies []string
		for _, tk := range toks {
			if tk.Kind == token.Heredoc {
				bodies = append(bodies, tk.Parts[0])
			}
		}
		if len(bodies) != 2 || bodies[0] != "p\n" || bodies[1] != "q\n" {
			t.Errorf("bodies = %q", bodies)
		}
	})
	t.Run("indented", func(t *testing.T) {
		src := "my $x = <<~EOT;\n    line one\n      more\n    EOT\n"
		tk := firstOfKind(t, src, token.Heredoc)
		if !tk.Indented {
			t.Fatal("Indented not set")
		}
		if tk.Parts[0] != "line one\n  more\n" {
			t.Errorf("dedented body = %q", tk.Parts[0])
		}
	})
	t.Run("indented with quoted tag and space", func(t *testing.T) {
		src := "my $x = <<~ 'EOT';\n  a\n  EOT\n"
		tk := firstOfKind(t, src, token.Heredoc)
		if tk.Interp {
			t.Error("quoted-single heredoc interpolates")
		}
		if tk.Parts[0] != "a\n" {
			t.Errorf("body = %q", tk.Parts[0])
		}
	})
	t.Run("backslash tag", func(t *testing.T) {
		src := "my $x = <<\\END;\nraw $stuff\nEND\n"
		tk := firstOfKind(t, src, token.Heredoc)
		if tk.Interp {
			t.Error("backslash-quoted heredoc must not interpolate")
		}
	})
	t.Run("unterminated", func(t *testing.T) {
		src := "my $x = <<EOF;\nnever ends\n"
		_, diags := Lex([]byte(src))
		if len(diags) == 0 {
			t.Fatal("expected a diagnostic for unterminated heredoc")
		}
	})
	t.Run("marker in arg list", func(t *testing.T) {
		src := "printf(<<FMT, 1, 2);\n%d-%d\nFMT\n"
		tk := firstOfKind(t, src, token.Heredoc)
		if tk.Parts[0] != "%d-%d\n" {
			t.Errorf("body = %q", tk.Parts[0])
		}
	})
}

func TestQuoteLikes(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		kind  token.Kind
		body  string
		body2 string
		mods  string
	}{
		{"q paren", `q(hello)`, token.StrSingle, "hello", "", ""},
		{"qq brace", `qq{a b}`, token.StrDouble, "a b", "", ""},
		{"q bracket", `q[x]`, token.StrSingle, "x", "", ""},
		{"q angle", `q<y>`, token.StrSingle, "y", "", ""},
		{"q bang", `q!z!`, token.StrSingle, "z", "", ""},
		{"q hash delim", `q#c#`, token.StrSingle, "c", "", ""},
		{"q alnum delim", `q qhelloq`, token.StrSingle, "hello", "", ""},
		{"nested parens", `q(a (b) c)`, token.StrSingle, "a (b) c", "", ""},
		{"regex braces nested", `m{a{2,3}}`, token.Match, "a{2,3}", "", ""},
		{"regex escaped brace", `m{a\{}`, token.Match, `a\{`, "", ""},
		{"subst slash", `s/a/b/g`, token.Substitute, "a", "b", "g"},
		{"subst braces", `s{a}{b}`, token.Substitute, "a", "b", ""},
		{"subst mixed brackets", `s{a}[b]`, token.Substitute, "a", "b", ""},
		{"subst parens", `s(a)(b)`, token.Substitute, "a", "b", ""},
		{"subst comment between", "s{a}   # swap\n{b}g", token.Substitute, "a", "b", "g"},
		{"tr ranges", `tr[a-z][A-Z]`, token.Transliterate, "a-z", "A-Z", ""},
		{"y form", `y/abc/xyz/`, token.Transliterate, "abc", "xyz", ""},
		{"tr single-quote delim", `tr'ab'cd'`, token.Transliterate, "ab", "cd", ""},
		{"qr with mods", `qr/^\d+$/i`, token.QuoteRegex, `^\d+$`, "", "i"},
		{"x flag hash not comment", "m/ a # not comment\nb /x", token.Match, " a # not comment\nb ", "", "x"},
		{"qw multiline", "qw(\n  a\n  b\n)", token.QwList, "\n  a\n  b\n", "", ""},
		{"empty q", `q()`, token.StrSingle, "", "", ""},
		{"subst e flag", `s/(\d+)/$1*2/e`, token.Substitute, `(\d+)`, `$1*2`, "e"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := firstOfKind(t, "my $v = "+tt.src+";", tt.kind)
			if tk.Parts[0] != tt.body {
				t.Errorf("part1 = %q, want %q", tk.Parts[0], tt.body)
			}
			if tt.body2 != "" || len(tk.Parts) > 1 && tk.Parts[1] != "" {
				if len(tk.Parts) < 2 || tk.Parts[1] != tt.body2 {
					t.Errorf("part2 = %v, want %q", tk.Parts, tt.body2)
				}
			}
			if tk.Mods != tt.mods {
				t.Errorf("mods = %q, want %q", tk.Mods, tt.mods)
			}
		})
	}
}

func TestCharClassSlashEndsScan(t *testing.T) {
	// The delimiter scan is purely lexical: a / inside [...] still ends the
	// pattern, exactly as perl's own scanner behaves.
	tk := firstOfKind(t, `my $m = /[/]/;`, token.Match)
	if tk.Parts[0] != "[" {
		t.Errorf("pattern = %q, want %q", tk.Parts[0], "[")
	}
}

func TestQuoteOpsAsNames(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"fat comma", `my %h = (q => 1, s => 2);`,
			[]string{"Ident(my)", "HashVar(%h)", "Assign", "LParen", "Ident(q)", "FatComma",
				"Number(1)", "Comma", "Ident(s)", "FatComma", "Number(2)", "RParen", "Semi"}},
		{"hash subscript", `print $h{s};`,
			[]string{"Ident(print)", "ScalarVar($h)", "LBrace", "Ident(s)", "RBrace", "Semi"}},
		{"method call", `$o->q;`,
			[]string{"ScalarVar($o)", "Arrow", "Ident(q)", "Semi"}},
		{"sub named s", `sub s { 1 }`,
			[]string{"Ident(sub)", "Ident(s)", "LBrace", "Number(1)", "RBrace"}},
		{"scalar named q", `my $q = 5;`,
			[]string{"Ident(my)", "ScalarVar($q)", "Assign", "Number(5)", "Semi"}},
		{"tr as key", `my %g = (tr => 7);`,
			[]string{"Ident(my)", "HashVar(%g)", "Assign", "LParen", "Ident(tr)", "FatComma",
				"Number(7)", "RParen", "Semi"}},
		{"q operator still works", `print q(ok);`,
			[]string{"Ident(print)", "StrSingle", "Semi"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectStream(t, tt.src, tt.want...) })
	}
}

func TestSigils(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"array len", `my $n = $#list;`,
			[]string{"Ident(my)", "ScalarVar($n)", "Assign", "ArrayLen($#list)", "Semi"}},
		{"array len deref", `my $n = $#{$r};`,
			[]string{"Ident(my)", "ScalarVar($n)", "Assign", "ArrayLen($#)", "LBrace",
				"ScalarVar($r)", "RBrace", "Semi"}},
		{"array len dollar deref", `my $n = $#$r;`,
			[]string{"Ident(my)", "ScalarVar($n)", "Assign", "ArrayLen($#)", "ScalarVar($r)", "Semi"}},
		{"scalar deref", `my $v = $$ref;`,
			[]string{"Ident(my)", "ScalarVar($v)", "Assign", "ScalarVar($)", "ScalarVar($ref)", "Semi"}},
		{"block deref", `my $v = ${$ref};`,
			[]string{"Ident(my)", "ScalarVar($v)", "Assign", "ScalarVar($)", "LBrace",
				"ScalarVar($ref)", "RBrace", "Semi"}},
		{"named block var", `print ${name};`,
			[]string{"Ident(print)", "ScalarVar(${name})", "Semi"}},
		{"caret block var", `print ${^GLOBAL_PHASE};`,
			[]string{"Ident(print)", "ScalarVar(${^GLOBAL_PHASE})", "Semi"}},
		{"array deref", `my @a = @$ref;`,
			[]string{"Ident(my)", "ArrayVar(@a)", "Assign", "ArrayVar(@)", "ScalarVar($ref)", "Semi"}},
		{"array block deref slice", `my @s = @{$r}[0,1];`,
			[]string{"Ident(my)", "ArrayVar(@s)", "Assign", "ArrayVar(@)", "LBrace",
				"ScalarVar($r)", "RBrace", "LBracket", "Number(0)", "Comma", "Number(1)",
				"RBracket", "Semi"}},
		{"hash deref", `my %h = %$ref;`,
			[]string{"Ident(my)", "HashVar(%h)", "Assign", "HashVar(%)", "ScalarVar($ref)", "Semi"}},
		{"code deref", `&$cb;`,
			[]string{"FuncVar(&)", "ScalarVar($cb)", "Semi"}},
		{"func sigil call", `&foo;`,
			[]string{"FuncVar(&foo)", "Semi"}},
		{"glob", `*alias = \&real;`,
			[]string{"GlobVar(*alias)", "Assign", "Backslash", "FuncVar(&real)", "Semi"}},
		{"package var", `print $Foo::Bar::baz;`,
			[]string{"Ident(print)", "ScalarVar($Foo::Bar::baz)", "Semi"}},
		{"pid var", `print $$;`,
			[]string{"Ident(print)", "ScalarVar($$)", "Semi"}},
		{"capture vars", `print $1, $2, $10;`,
			[]string{"Ident(print)", "ScalarVar($1)", "Comma", "ScalarVar($2)", "Comma",
				"ScalarVar($10)", "Semi"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectStream(t, tt.src, tt.want...) })
	}
}

func TestMagicVariables(t *testing.T) {
	for _, v := range []string{"$!", "$@", "$0", "$;", "$,", "$/", "$\\", `$"`, "$|",
		"$)", "$(", "$&", "$'", "$`", "$+", "$.", "$?", "$_"} {
		t.Run(v, func(t *testing.T) {
			tk := firstOfKind(t, "print "+v+";", token.ScalarVar)
			if tk.Text != v {
				t.Errorf("lexed %q, want %q", tk.Text, v)
			}
		})
	}
	t.Run("$^W", func(t *testing.T) {
		tk := firstOfKind(t, `print $^W;`, token.ScalarVar)
		if tk.Text != "$^W" {
			t.Errorf("lexed %q", tk.Text)
		}
	})
	t.Run("local list separator", func(t *testing.T) {
		expectStream(t, `local $, = "-";`,
			"Ident(local)", "ScalarVar($,)", "Assign", "StrDouble", "Semi")
	})
	t.Run("dollar-quote does not open a string", func(t *testing.T) {
		expectStream(t, `local $" = "+";`,
			"Ident(local)", `ScalarVar($")`, "Assign", "StrDouble", "Semi")
	})
	t.Run("at magic", func(t *testing.T) {
		expectStream(t, `print @_;`, "Ident(print)", "ArrayVar(@_)", "Semi")
	})
}

func TestNumbers(t *testing.T) {
	tests := []struct {
		src  string
		kind token.Kind
		text string
	}{
		{"42", token.Number, "42"},
		{"3.14", token.Number, "3.14"},
		{".5", token.Number, ".5"},
		{"1e10", token.Number, "1e10"},
		{"1.5e-3", token.Number, "1.5e-3"},
		{"0x1F", token.Number, "0x1F"},
		{"0b101", token.Number, "0b101"},
		{"017", token.Number, "017"},
		{"0o17", token.Number, "0o17"},
		{"1_000_000", token.Number, "1_000_000"},
		{"5.36.1", token.Version, "5.36.1"},
		{"v5.36", token.Version, "v5.36"},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			tk := firstOfKind(t, "my $x = "+tt.src+";", tt.kind)
			if tk.Text != tt.text {
				t.Errorf("lexed %q, want %q", tk.Text, tt.text)
			}
		})
	}
	t.Run("string ten vs 1e1", func(t *testing.T) {
		expectStream(t, `"10" == "1e1";`, "StrDouble", "NumEq", "StrDouble", "Semi")
	})
	t.Run("range stays a range", func(t *testing.T) {
		expectStream(t, `my @r = (1..5);`,
			"Ident(my)", "ArrayVar(@r)", "Assign", "LParen", "Number(1)", "DotDot",
			"Number(5)", "RParen", "Semi")
	})
}

func TestPodAndData(t *testing.T) {
	t.Run("pod block", func(t *testing.T) {
		src := "my $x = 1;\n=pod\n\nAnything /goes $here\n\n=cut\nmy $y = 2;\n"
		toks, diags := Lex([]byte(src))
		if len(diags) != 0 {
			t.Fatalf("diags: %v", diags)
		}
		var kinds []string
		for _, tk := range toks {
			kinds = append(kinds, tk.Kind.String())
		}
		joined := strings.Join(kinds, " ")
		if !strings.Contains(joined, "Pod") {
			t.Errorf("no Pod token in %s", joined)
		}
		expectStream(t, src,
			"Ident(my)", "ScalarVar($x)", "Assign", "Number(1)", "Semi",
			"Ident(my)", "ScalarVar($y)", "Assign", "Number(2)", "Semi")
	})
	t.Run("equals continuation is not pod", func(t *testing.T) {
		src := "my $x\n= 5;\n"
		expectStream(t, src, "Ident(my)", "ScalarVar($x)", "Assign", "Number(5)", "Semi")
	})
	t.Run("begin cut", func(t *testing.T) {
		src := "=begin html\n<p>hi</p>\n=cut\nprint 1;\n"
		expectStream(t, src, "Ident(print)", "Number(1)", "Semi")
	})
	t.Run("end marker", func(t *testing.T) {
		src := "print 1;\n__END__\nfree /text $goes here\n"
		tk := firstOfKind(t, src, token.Data)
		if tk.Tag != "__END__" || tk.Parts[0] != "free /text $goes here\n" {
			t.Errorf("data token: tag=%q parts=%q", tk.Tag, tk.Parts)
		}
	})
	t.Run("data marker", func(t *testing.T) {
		src := "print 1;\n__DATA__\nrow1\nrow2\n"
		tk := firstOfKind(t, src, token.Data)
		if tk.Tag != "__DATA__" || tk.Parts[0] != "row1\nrow2\n" {
			t.Errorf("data token: tag=%q parts=%q", tk.Tag, tk.Parts)
		}
	})
}

func TestFiletestAndMinus(t *testing.T) {
	expectStream(t, `die unless -e $file;`,
		"Ident(die)", "Ident(unless)", "FileTest(-e)", "ScalarVar($file)", "Semi")
	expectStream(t, `my $d = -s $f;`,
		"Ident(my)", "ScalarVar($d)", "Assign", "FileTest(-s)", "ScalarVar($f)", "Semi")
	expectStream(t, `my $d = $a - $b;`,
		"Ident(my)", "ScalarVar($d)", "Assign", "ScalarVar($a)", "Minus", "ScalarVar($b)", "Semi")
	expectStream(t, `my $n = -5;`,
		"Ident(my)", "ScalarVar($n)", "Assign", "Minus", "Number(5)", "Semi")
	// -foo is unary minus on a bareword, not a filetest (f is a test letter
	// but followed by identifier chars).
	expectStream(t, `my $s = -foo;`,
		"Ident(my)", "ScalarVar($s)", "Assign", "Minus", "Ident(foo)", "Semi")
}

func TestWordOperators(t *testing.T) {
	expectStream(t, `$a eq $b;`, "ScalarVar($a)", "StrEq", "ScalarVar($b)", "Semi")
	expectStream(t, `$a cmp $b;`, "ScalarVar($a)", "StrCmp", "ScalarVar($b)", "Semi")
	expectStream(t, `$a and $b or $c;`,
		"ScalarVar($a)", "AndLow", "ScalarVar($b)", "OrLow", "ScalarVar($c)", "Semi")
	expectStream(t, `not $x;`, "NotLow", "ScalarVar($x)", "Semi")
	expectStream(t, `"ab" x 3;`, "StrDouble", "Repeat", "Number(3)", "Semi")
	expectStream(t, `"ab"x3;`, "StrDouble", "Repeat", "Number(3)", "Semi")
	expectStream(t, `$s x= 2;`, "ScalarVar($s)", "OpAssign", "Number(2)", "Semi")
	// xor after a value; x followed by letters is a bareword.
	expectStream(t, `$a xor $b;`, "ScalarVar($a)", "XorLow", "ScalarVar($b)", "Semi")
}

func TestBraceClassification(t *testing.T) {
	// After a subscript } the lexer expects an operator, so / divides.
	expectStream(t, `my $r = $h{n} / 2;`,
		"Ident(my)", "ScalarVar($r)", "Assign", "ScalarVar($h)", "LBrace", "Ident(n)",
		"RBrace", "Slash", "Number(2)", "Semi")
	// After a block } it expects a term, so / starts a match.
	expectStream(t, `for (@x) { } /re/;`,
		"Ident(for)", "LParen", "ArrayVar(@x)", "RParen", "LBrace", "RBrace", "Match", "Semi")
	// Chained subscripts stay subscripts.
	expectStream(t, `my $v = $h{a}{b};`,
		"Ident(my)", "ScalarVar($v)", "Assign", "ScalarVar($h)", "LBrace", "Ident(a)",
		"RBrace", "LBrace", "Ident(b)", "RBrace", "Semi")
	// Arrow subscript.
	expectStream(t, `my $v = $r->{key}[0];`,
		"Ident(my)", "ScalarVar($v)", "Assign", "ScalarVar($r)", "Arrow", "LBrace",
		"Ident(key)", "RBrace", "LBracket", "Number(0)", "RBracket", "Semi")
}

func TestMiscOperators(t *testing.T) {
	expectStream(t, `$x ** 2;`, "ScalarVar($x)", "StarStar", "Number(2)", "Semi")
	expectStream(t, `$x **= 2;`, "ScalarVar($x)", "OpAssign", "Number(2)", "Semi")
	expectStream(t, `$i++;`, "ScalarVar($i)", "PlusPlus", "Semi")
	expectStream(t, `--$i;`, "MinusMinus", "ScalarVar($i)", "Semi")
	expectStream(t, `$s .= "x";`, "ScalarVar($s)", "OpAssign", "StrDouble", "Semi")
	expectStream(t, `$a .. $b;`, "ScalarVar($a)", "DotDot", "ScalarVar($b)", "Semi")
	expectStream(t, `@a[1..3];`,
		"ArrayVar(@a)", "LBracket", "Number(1)", "DotDot", "Number(3)", "RBracket", "Semi")
	expectStream(t, `$x ? $a : $b;`,
		"ScalarVar($x)", "Question", "ScalarVar($a)", "Colon", "ScalarVar($b)", "Semi")
	expectStream(t, `%h = ();`, "HashVar(%h)", "Assign", "LParen", "RParen", "Semi")
	expectStream(t, `$n % 3;`, "ScalarVar($n)", "Percent", "Number(3)", "Semi")
	expectStream(t, `$x !~ /a/;`, "ScalarVar($x)", "NotMatchBind", "Match", "Semi")
	expectStream(t, `MAIN: for (;;) { last MAIN }`,
		"Ident(MAIN)", "Colon", "Ident(for)", "LParen", "Semi", "Semi", "RParen",
		"LBrace", "Ident(last)", "Ident(MAIN)", "RBrace")
}

func TestSeedCaseFilesLexCleanly(t *testing.T) {
	// The corpus of hazard scripts: every one must lex without diagnostics
	// (they are all valid Perl), except the deliberately broken ones.
	files := seedCaseSources(t)
	if len(files) == 0 {
		t.Skip("no seed case files available")
	}
	allowDiags := map[string]bool{
		"heredoc-11-unterminated.pl":   true, // deliberately unterminated
		"slash-08-after-identifier.pl": true, // exercises the prototype ambiguity
	}
	for name, src := range files {
		t.Run(name, func(t *testing.T) {
			toks, diags := Lex([]byte(src))
			if len(diags) > 0 && !allowDiags[name] {
				t.Errorf("diagnostics on valid input: %v", diags)
			}
			for _, tk := range toks {
				if tk.Kind == token.Illegal && !allowDiags[name] {
					t.Errorf("illegal token at %s: %q", tk.Pos, tk.Text)
				}
			}
		})
	}
}

func TestCoverage(t *testing.T) {
	sources := []string{
		"my ($a, $b) = (10, 4);\nmy $r = $a / $b;\nprint \"r=$r\\n\";\n",
		"my $x = <<EOF . <<'RAW';\nhi $n\nEOF\nraw\nRAW\nprint $x;\n",
		"# comment\nmy %h = (q => 1);\nprint $h{q}, \"\\n\";\n=pod\ndocs\n=cut\nprint 2;\n__DATA__\ntail\n",
		"while (my $l = <STDIN>) { chomp $l; print $l if $l =~ /x/; }\n",
	}
	for i, src := range sources {
		t.Run(fmt.Sprintf("src%d", i), func(t *testing.T) {
			assertFullCoverage(t, src)
		})
	}
}

// assertFullCoverage checks that every non-whitespace byte of src is covered
// by exactly one token's text span or heredoc body span.
func assertFullCoverage(t *testing.T, src string) {
	t.Helper()
	toks, _ := Lex([]byte(src))
	covered := make([]int, len(src))
	mark := func(from, to int) {
		for i := from; i < to && i < len(covered); i++ {
			covered[i]++
		}
	}
	for _, tk := range toks {
		if tk.Kind == token.EOF {
			continue
		}
		mark(tk.Pos.Offset, tk.Pos.Offset+len(tk.Text))
		if tk.Kind == token.Heredoc {
			mark(tk.BodyStart, tk.BodyEnd)
		}
	}
	for i, c := range covered {
		b := src[i]
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' {
			continue
		}
		if c == 0 {
			t.Fatalf("byte %d %q not covered by any token\nsource: %q", i, b, src)
		}
		if c > 1 {
			t.Fatalf("byte %d %q covered %d times\nsource: %q", i, b, c, src)
		}
	}
}
