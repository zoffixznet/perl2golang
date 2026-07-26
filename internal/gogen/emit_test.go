package gogen

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"perl2go/internal/ir"
)

// block is a shorthand for a statement list in the test trees.
func block(stmts ...ir.Stmt) *ir.Block {
	b := &ir.Block{}
	b.Add(stmts...)
	return b
}

// TestFileGolden pins the exact text of a few small trees in both renderings.
// Anything that changes the shape of the output has to change these strings
// too, which is the point: the output is the product.
func TestFileGolden(t *testing.T) {
	tests := []struct {
		name      string
		file      *ir.File
		clean     string
		annotated string
	}{
		{
			name: "hello",
			file: &ir.File{
				Name:    "main.go",
				Package: "main",
				Decls: []ir.Decl{
					&ir.FuncDecl{
						Meta: noted(1, "sub main {", "Println writes its arguments and a newline."),
						Name: "main",
						Body: block(&ir.ExprStmt{
							X: ir.CallOf(ir.Pkg("fmt", "fmt", "Println", ir.TVoid), ir.TVoid, ir.Str(`"hi"`)),
						}),
					},
				},
			},
			clean: `package main

import "fmt"

func main() {
	fmt.Println("hi")
}
`,
			annotated: `package main

import "fmt"

// Perl: sub main {
// Println writes its arguments and a newline.
func main() {
	fmt.Println("hi")
}
`,
		},
		{
			name: "vars",
			file: &ir.File{
				Name:    "cfg.go",
				Package: "cfg",
				Doc:     []string{"Package cfg holds the settings."},
				Decls: []ir.Decl{
					&ir.VarDecl{
						Names:  []string{"limit"},
						Type:   ir.TInt,
						Values: []ir.Expr{ir.IntLit("64")},
						Const:  true,
						Doc:    []string{"limit caps the size of a report."},
					},
					&ir.VarDecl{
						Meta:  prov(2, "our @names;"),
						Names: []string{"names"},
						Type:  ir.SliceOf(ir.TString),
					},
				},
			},
			clean: `// Package cfg holds the settings.
package cfg

// limit caps the size of a report.
const limit int = 64

var names []string
`,
			annotated: `// Package cfg holds the settings.
package cfg

// limit caps the size of a report.
const limit int = 64

// Perl: our @names;
var names []string
`,
		},
		{
			name: "loop",
			file: &ir.File{
				Name:    "count.go",
				Package: "main",
				Decls: []ir.Decl{
					&ir.FuncDecl{
						Name:    "count",
						Params:  []ir.Param{{Name: "words", Type: ir.SliceOf(ir.TString)}},
						Results: []*ir.Type{ir.TInt, ir.TError},
						Body: block(
							&ir.DeclStmt{Names: []string{"total"}, Type: ir.TInt},
							&ir.Range{
								Meta:   prov(4, "foreach my $w (@words) {"),
								Value:  ir.NewIdent("w", ir.TString),
								X:      ir.NewIdent("words", ir.SliceOf(ir.TString)),
								Define: true,
								Body: block(&ir.Assign{
									Op:  "+=",
									LHS: []ir.Expr{ir.NewIdent("total", ir.TInt)},
									RHS: []ir.Expr{ir.CallOf(ir.NewIdent("len", nil), ir.TInt, ir.NewIdent("w", ir.TString))},
								}),
							},
							&ir.Return{Results: []ir.Expr{ir.NewIdent("total", ir.TInt), ir.Nil(ir.TError)}},
						),
					},
				},
			},
			clean: `package main

func count(words []string) (int, error) {
	var total int
	for _, w := range words {
		total += len(w)
	}
	return total, nil
}
`,
			annotated: `package main

func count(words []string) (int, error) {
	var total int
	// Perl: foreach my $w (@words) {
	for _, w := range words {
		total += len(w)
	}
	return total, nil
}
`,
		},
		{
			name: "todo",
			file: &ir.File{
				Name:    "slurp.go",
				Package: "main",
				Decls: []ir.Decl{
					&ir.FuncDecl{
						Name: "slurp",
						Body: block(&ir.TodoStmt{
							Meta: prov(3, "local $/ = undef;"),
							Info: ir.Todo{
								Code:    "P2G1301",
								Short:   "reading a whole file at once is not implemented here",
								Message: "the input record separator has no equivalent",
								Perl:    "local $/ = undef;",
							},
							Panic: true,
						}),
					},
				},
			},
			clean: `package main

func slurp() {
	// TODO: reading a whole file at once is not implemented here
	panic("P2G1301: reading a whole file at once is not implemented here")
}
`,
			annotated: `package main

func slurp() {
	// Perl: local $/ = undef;
	// TODO: P2G1301: the input record separator has no equivalent
	//   local $/ = undef;
	panic("P2G1301: reading a whole file at once is not implemented here")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, m := range []struct {
				mode Mode
				want string
			}{{Clean, tt.clean}, {Annotated, tt.annotated}} {
				got, err := New(m.mode).File(tt.file)
				if err != nil {
					t.Fatalf("mode %v: %v\n%s", m.mode, err, got)
				}
				if string(got) != m.want {
					t.Errorf("mode %v output differs\n--- got ---\n%s\n--- want ---\n%s", m.mode, got, m.want)
				}
			}
		})
	}
}

// TestModesDifferOnlyInComments proves the two renderings cannot drift into
// being different programs: strip the comments from each and they are the same
// source.
func TestModesDifferOnlyInComments(t *testing.T) {
	for _, s := range samples() {
		t.Run(s.name, func(t *testing.T) {
			clean, err := New(Clean).File(s.file)
			if err != nil {
				t.Fatalf("clean: %v", err)
			}
			annotated, err := New(Annotated).File(s.file)
			if err != nil {
				t.Fatalf("annotated: %v", err)
			}
			a := stripComments(t, clean)
			b := stripComments(t, annotated)
			if a != b {
				t.Errorf("renderings differ once comments are removed\n--- clean ---\n%s\n--- annotated ---\n%s", a, b)
			}
		})
	}
}

// TestCleanCarriesNoConversionVocabulary is the product rule: the clean
// program is a Go program, not a report about a conversion. Every word that
// would give the game away is banned from its comments.
func TestCleanCarriesNoConversionVocabulary(t *testing.T) {
	banned := []string{"perl", "convert", "translate", "generated"}
	for _, s := range samples() {
		t.Run(s.name, func(t *testing.T) {
			src, err := New(Clean).File(s.file)
			if err != nil {
				t.Fatalf("clean: %v", err)
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, s.name+".go", src, parser.ParseComments|parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			for _, group := range f.Comments {
				text := strings.ToLower(group.Text())
				for _, word := range banned {
					if strings.Contains(text, word) {
						t.Errorf("clean comment contains %q: %q", word, group.Text())
					}
				}
			}
			// The annotated rendering is where all of that lives, so the same
			// tree must in fact have something to hide.
			ann, err := New(Annotated).File(s.file)
			if err != nil {
				t.Fatalf("annotated: %v", err)
			}
			if !bytes.Contains(ann, []byte("// Perl:")) && !bytes.Contains(ann, []byte("/* Perl:")) {
				t.Errorf("sample carries no provenance, so it cannot prove anything")
			}
		})
	}
}

// TestSamplesParse checks that everything the emitter produces is Go. A
// failure here is a bug in this package, never in the input.
func TestSamplesParse(t *testing.T) {
	for _, s := range samples() {
		for _, m := range []Mode{Clean, Annotated} {
			src, err := New(m).File(s.file)
			if err != nil {
				t.Fatalf("%s mode %v: %v\n%s", s.name, m, err, src)
			}
			if err := Parse(s.file.Name, src); err != nil {
				t.Errorf("%s mode %v: %v", s.name, m, err)
			}
		}
	}
}

// TestPrecedence asserts that the emitter parenthesises where Go's grammar
// needs it and nowhere else. Redundant parentheses would teach the reader the
// wrong lesson about Go.
func TestPrecedence(t *testing.T) {
	a := ir.NewIdent("a", ir.TInt)
	b := ir.NewIdent("b", ir.TInt)
	c := ir.NewIdent("c", ir.TInt)
	p := ir.NewIdent("p", ir.TBool)
	q := ir.NewIdent("q", ir.TBool)
	r := ir.NewIdent("r", ir.TBool)

	tests := []struct {
		name string
		expr ir.Expr
		want string
	}{
		{"mul binds tighter than add", ir.Bin("+", a, ir.Bin("*", b, c, ir.TInt), ir.TInt), "a + b*c"},
		{"add under mul needs parens", ir.Bin("*", ir.Bin("+", a, b, ir.TInt), c, ir.TInt), "(a + b) * c"},
		{"left associative chain is flat", ir.Bin("-", ir.Bin("-", a, b, ir.TInt), c, ir.TInt), "a - b - c"},
		{"right nested subtraction keeps parens", ir.Bin("-", a, ir.Bin("-", b, c, ir.TInt), ir.TInt), "a - (b - c)"},
		{"and binds tighter than or", ir.Bin("||", ir.Bin("&&", p, q, ir.TBool), r, ir.TBool), "p && q || r"},
		{"or under and needs parens", ir.Bin("&&", ir.Bin("||", p, q, ir.TBool), r, ir.TBool), "(p || q) && r"},
		{"comparison under and is flat", ir.Bin("&&", ir.Bin("<", a, b, ir.TBool), p, ir.TBool), "a < b && p"},
		{"shift takes a parenthesised sum", ir.Bin("<<", a, ir.Bin("+", b, c, ir.TInt), ir.TInt), "a << (b + c)"},
		{"not of a comparison needs parens", ir.Un("!", ir.Bin("==", a, b, ir.TBool), ir.TBool), "!(a == b)"},
		{"not of a name does not", ir.Bin("==", ir.Un("!", p, ir.TBool), q, ir.TBool), "!p == q"},
		{"negation binds tighter than product", ir.Bin("*", ir.Un("-", a, ir.TInt), b, ir.TInt), "-a * b"},
		{"double negation keeps a space", ir.Un("-", ir.Un("-", a, ir.TInt), ir.TInt), "- -a"},
		{"selector on a sum needs parens", &ir.Selector{X: ir.Bin("+", a, b, ir.TInt), Sel: "field"}, "(a + b).field"},
		{"index on a name does not", &ir.Index{X: ir.NewIdent("xs", nil), Index: ir.Bin("+", a, b, ir.TInt)}, "xs[a+b]"},
		{"call arguments are unparenthesised", ir.CallOf(ir.NewIdent("f", nil), ir.TInt, ir.Bin("+", a, b, ir.TInt)), "f(a + b)"},
		{"pointer conversion parenthesises the type",
			&ir.Conversion{To: ir.PointerTo(ir.NamedType("Record", "")), X: ir.NewIdent("v", nil)}, "(*Record)(v)"},
		{"slice conversion does not",
			&ir.Conversion{To: ir.SliceOf(ir.NamedType("byte", "")), X: ir.NewIdent("s", ir.TString)}, "[]byte(s)"},
		{"explicit parens are honoured", &ir.Paren{X: a}, "(a)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenderExpr(Clean, tt.expr); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRenderFragments covers the isolated renderers the walkthrough document
// uses, including the shapes that must not panic.
func TestRenderFragments(t *testing.T) {
	stmt := &ir.If{
		Meta: noted(5, "if ($n) { $n++ }", "Go has no truthiness, so the condition is a comparison."),
		Cond: ir.Bin("!=", ir.NewIdent("n", ir.TInt), ir.IntLit("0"), ir.TBool),
		Then: block(&ir.IncDec{X: ir.NewIdent("n", ir.TInt)}),
	}
	wantClean := "if n != 0 {\n\tn++\n}"
	if got := RenderStmt(Clean, stmt); got != wantClean {
		t.Errorf("RenderStmt clean:\n%s\nwant:\n%s", got, wantClean)
	}
	wantAnnotated := "// Perl: if ($n) { $n++ }\n// Go has no truthiness, so the condition is a comparison.\nif n != 0 {\n\tn++\n}"
	if got := RenderStmt(Annotated, stmt); got != wantAnnotated {
		t.Errorf("RenderStmt annotated:\n%s\nwant:\n%s", got, wantAnnotated)
	}

	decl := &ir.FuncDecl{Name: "run", Doc: []string{"run does the work."}, Body: block()}
	wantDecl := "// run does the work.\nfunc run() {\n}"
	if got := RenderDecl(Clean, decl); got != wantDecl {
		t.Errorf("RenderDecl:\n%s\nwant:\n%s", got, wantDecl)
	}

	// A fragment renderer feeds a document, so it must survive anything.
	if got := RenderStmt(Clean, nil); got != "" {
		t.Errorf("RenderStmt(nil) = %q, want empty", got)
	}
	if got := RenderDecl(Clean, nil); got != "" {
		t.Errorf("RenderDecl(nil) = %q, want empty", got)
	}
	if got := RenderExpr(Clean, nil); got != "" {
		t.Errorf("RenderExpr(nil) = %q, want empty", got)
	}
	if got := RenderExpr(Clean, &ir.Binary{Op: "+"}); got != "nil + nil" {
		t.Errorf("RenderExpr of a half-built binary = %q, want a parseable placeholder", got)
	}
	if got := RenderStmt(Clean, &ir.Assign{}); got != "" {
		t.Errorf("RenderStmt of an empty assignment = %q, want empty", got)
	}
	if got := RenderStmt(Clean, &ir.ExprStmt{X: &ir.FuncLit{Body: block()}}); got != "func() {\n}" {
		t.Errorf("RenderStmt of a function literal = %q", got)
	}
	// defer and go accept only a call, so anything else has to be wrapped.
	if got := RenderStmt(Clean, &ir.Defer{Call: ir.NewIdent("x", nil)}); got != "defer func() { _ = x }()" {
		t.Errorf("RenderStmt of a defer without a call = %q", got)
	}
	if got := RenderStmt(Clean, &ir.Branch{Kind: "goto"}); got != "" {
		t.Errorf("RenderStmt of a goto with no label = %q, want empty", got)
	}
}

// TestFragmentsNeverPanic walks one node of every kind through the isolated
// renderers with as little filled in as the IR allows.
func TestFragmentsNeverPanic(t *testing.T) {
	stmts := []ir.Stmt{
		&ir.Block{}, &ir.Assign{}, &ir.DeclStmt{}, &ir.ExprStmt{}, &ir.IncDec{},
		&ir.If{}, &ir.For{}, &ir.Range{}, &ir.Return{}, &ir.Branch{}, &ir.Labeled{},
		&ir.Switch{}, &ir.Defer{}, &ir.Go{}, &ir.BlockStmt{}, &ir.CommentStmt{},
		&ir.TodoStmt{Panic: true}, &ir.RawStmt{},
	}
	exprs := []ir.Expr{
		&ir.Ident{}, &ir.Lit{}, &ir.Call{}, &ir.Selector{}, &ir.Index{}, &ir.IndexComma{},
		&ir.SliceExpr{}, &ir.Binary{}, &ir.Unary{}, &ir.Paren{}, &ir.CompositeLit{},
		&ir.FuncLit{}, &ir.TypeAssert{}, &ir.Conversion{}, &ir.RawExpr{},
	}
	decls := []ir.Decl{
		&ir.FuncDecl{}, &ir.VarDecl{}, &ir.TypeDecl{}, &ir.ImportDecl{}, &ir.RawDecl{},
	}
	for _, m := range []Mode{Clean, Annotated} {
		for _, s := range stmts {
			RenderStmt(m, s)
		}
		for _, x := range exprs {
			RenderExpr(m, x)
		}
		for _, d := range decls {
			RenderDecl(m, d)
		}
	}
}

// TestStatementCoverage renders one file containing every statement kind and
// checks that the result is still Go.
func TestStatementCoverage(t *testing.T) {
	x := ir.NewIdent("x", ir.TInt)
	body := block(
		&ir.BlockStmt{Body: block(&ir.Assign{Op: ":=", LHS: []ir.Expr{x}, RHS: []ir.Expr{ir.IntLit("1")}})},
		&ir.Labeled{Label: "again", Stmt: &ir.For{Body: block(&ir.Branch{Kind: "break", Label: "again"})}},
		&ir.Switch{
			Init: &ir.Assign{Op: ":=", LHS: []ir.Expr{ir.NewIdent("y", ir.TInt)}, RHS: []ir.Expr{ir.IntLit("2")}},
			Tag:  ir.NewIdent("y", ir.TInt),
			Cases: []ir.Case{
				{Values: []ir.Expr{ir.IntLit("2")}, Body: block(&ir.Branch{Kind: "break"})},
			},
		},
		&ir.Go{Call: ir.CallOf(ir.NewIdent("work", nil), ir.TVoid)},
		&ir.Defer{Call: ir.CallOf(ir.NewIdent("done", nil), ir.TVoid)},
		&ir.If{
			Init: &ir.Assign{Op: ":=", LHS: []ir.Expr{ir.NewIdent("z", ir.TInt)}, RHS: []ir.Expr{ir.IntLit("3")}},
			Cond: ir.Bin(">", ir.NewIdent("z", ir.TInt), ir.IntLit("0"), ir.TBool),
			Then: block(&ir.ExprStmt{X: ir.CallOf(ir.NewIdent("work", nil), ir.TVoid)}),
			Else: &ir.ExprStmt{X: ir.CallOf(ir.NewIdent("done", nil), ir.TVoid)},
		},
		&ir.For{Body: block(&ir.Branch{Kind: "break"})},
		&ir.Range{X: ir.NewIdent("list", ir.SliceOf(ir.TInt)), Body: block()},
		&ir.Range{Key: ir.NewIdent("k", ir.TInt), X: ir.NewIdent("list", ir.SliceOf(ir.TInt)), Define: true,
			Body: block(&ir.ExprStmt{X: ir.CallOf(ir.NewIdent("work", nil), ir.TVoid, ir.NewIdent("k", ir.TInt))})},
		&ir.CommentStmt{Lines: []string{"a comment the developer wrote"}},
		&ir.RawStmt{Source: "_ = x"},
		&ir.Return{},
	)
	f := &ir.File{
		Name:    "all.go",
		Package: "main",
		Decls: []ir.Decl{
			&ir.FuncDecl{Name: "all", Params: []ir.Param{{Name: "list", Type: ir.SliceOf(ir.TInt)}}, Body: body},
		},
	}
	for _, m := range []Mode{Clean, Annotated} {
		src, err := New(m).File(f)
		if err != nil {
			t.Fatalf("mode %v: %v\n%s", m, err, src)
		}
		if err := Parse(f.Name, src); err != nil {
			t.Errorf("mode %v: %v", m, err)
		}
	}
}

// TestCompositeLiteralInHeader covers the parser ambiguity Go resolves with
// parentheses: a composite literal at the head of a statement.
func TestCompositeLiteralInHeader(t *testing.T) {
	lit := &ir.CompositeLit{LitType: ir.NamedType("Record", "")}
	f := &ir.File{
		Name:    "hdr.go",
		Package: "main",
		Decls: []ir.Decl{&ir.FuncDecl{
			Name: "hdr",
			Body: block(&ir.If{
				Cond: ir.Bin("==", ir.NewIdent("r", nil), lit, ir.TBool),
				Then: block(&ir.Return{}),
			}),
		}},
	}
	src, err := New(Clean).File(f)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	if !strings.Contains(string(src), "if r == (Record{}) {") {
		t.Errorf("composite literal in an if header was not parenthesised:\n%s", src)
	}
}

// TestExpressionAnnotations covers annotations attached to an expression
// rather than a statement. A line comment there would swallow the rest of the
// expression, so they are written as block comments.
func TestExpressionAnnotations(t *testing.T) {
	n := ir.NewIdent("n", ir.TInt)
	n.Meta = prov(6, "$n")
	ir.Annotate(n, "n counts the lines read so far.")
	n.Meta.Todo = &ir.Todo{
		Code:    "P2G1401",
		Short:   "the count ignores blank lines",
		Message: "the original counted blank lines too",
	}
	sum := ir.Bin("+", n, ir.IntLit("1"), ir.TInt)

	wantClean := "/* TODO: the count ignores blank lines */ n + 1"
	if got := RenderExpr(Clean, sum); got != wantClean {
		t.Errorf("clean: got %q, want %q", got, wantClean)
	}
	// Provenance is not repeated inside an expression: the statement above it
	// already quotes the source.
	wantAnnotated := "/* n counts the lines read so far. */ " +
		"/* TODO: P2G1401: the original counted blank lines too */ n + 1"
	if got := RenderExpr(Annotated, sum); got != wantAnnotated {
		t.Errorf("annotated: got %q, want %q", got, wantAnnotated)
	}
}

// TestBlockAnnotationsGoInsideTheBraces checks that a block's own notes are
// not lost when its opening brace shares a line with an if or a for.
func TestBlockAnnotationsGoInsideTheBraces(t *testing.T) {
	body := block(&ir.Return{})
	body.Meta = noted(8, "{ ... }", "This block was a bare Perl block used for scoping.")
	stmt := &ir.If{Cond: ir.NewIdent("ok", ir.TBool), Then: body}

	want := "if ok {\n\t// Perl: { ... }\n\t// This block was a bare Perl block used for scoping.\n\treturn\n}"
	if got := RenderStmt(Annotated, stmt); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := RenderStmt(Clean, stmt); got != "if ok {\n\treturn\n}" {
		t.Errorf("clean rendering picked up commentary:\n%s", got)
	}
}

// TestFileGuards covers the two ways a caller can hand the emitter something
// that is not a Go file, neither of which may produce source that fails to
// compile.
func TestFileGuards(t *testing.T) {
	if _, err := New(Clean).File(nil); err == nil {
		t.Error("a nil file was accepted")
	}
	src, err := New(Clean).File(&ir.File{Name: "x.go"})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if string(src) != "package main\n" {
		t.Errorf("a file with no package clause rendered as:\n%s", src)
	}
}

// stripComments reparses source without comments and reprints it, which is the
// only reliable way to compare two renderings for sameness of program.
func stripComments(t *testing.T, src []byte) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, src)
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		t.Fatalf("print: %v", err)
	}
	// Removing a comment leaves the blank line it stood on behind.
	var kept []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
