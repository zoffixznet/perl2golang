package gogen

import (
	"perl2go/internal/ir"
)

// prov builds a provenance record for a test node.
func prov(line int, text string) ir.Meta {
	return ir.Meta{Prov: ir.Provenance{Line: line, Col: 1, Text: text}}
}

// noted builds a meta with provenance and one explanatory note.
func noted(line int, text, note string) ir.Meta {
	m := prov(line, text)
	m.Notes = []ir.Note{{Text: note}}
	return m
}

// sample is a named IR file used by several tests at once.
type sample struct {
	name string
	file *ir.File
}

// samples returns the IR trees every whole-file test runs over. They are
// deliberately small and each one exercises a different corner of the emitter.
func samples() []sample {
	return []sample{
		{"greet", greetFile()},
		{"control", controlFile()},
		{"types", typesFile()},
		{"todo", todoFile()},
	}
}

// greetFile covers a function, a doc comment, an import registered through a
// selector, and a note-carrying statement.
func greetFile() *ir.File {
	call := ir.CallOf(ir.Pkg("fmt", "fmt", "Printf", nil), ir.TVoid,
		ir.Str(`"hello, %s\n"`),
		ir.NewIdent("name", ir.TString),
	)
	body := &ir.Block{}
	body.Add(&ir.ExprStmt{
		Meta: noted(4, `print "hello, $name\n";`, "fmt.Printf takes a format string and the values to fill it in."),
		X:    call,
	})
	return &ir.File{
		Name:    "greet.go",
		Package: "main",
		Doc:     []string{"Command greet says hello."},
		Decls: []ir.Decl{
			&ir.FuncDecl{
				Meta:   noted(3, "sub greet {", "A Perl sub with one scalar argument becomes a function taking a string."),
				Name:   "greet",
				Params: []ir.Param{{Name: "name", Type: ir.TString}},
				Doc:    []string{"greet writes a greeting to standard output."},
				Body:   body,
			},
		},
	}
}

// controlFile covers if/else if/else, for, range, switch, and a labelled
// break.
func controlFile() *ir.File {
	n := ir.NewIdent("n", ir.TInt)
	items := ir.NewIdent("items", ir.SliceOf(ir.TString))

	thenBlock := &ir.Block{}
	thenBlock.Add(&ir.Return{Results: []ir.Expr{ir.Str(`"small"`)}})
	elseIf := &ir.If{
		Meta: prov(9, "elsif ($n < 100) {"),
		Cond: ir.Bin("<", n, ir.IntLit("100"), ir.TBool),
		Then: func() *ir.Block {
			b := &ir.Block{}
			b.Add(&ir.Return{Results: []ir.Expr{ir.Str(`"medium"`)}})
			return b
		}(),
		Else: func() *ir.Block {
			b := &ir.Block{}
			b.Add(&ir.Return{Results: []ir.Expr{ir.Str(`"large"`)}})
			return b
		}(),
	}

	loop := &ir.For{
		Meta:  prov(14, "for (my $i = 0; $i < $n; $i++) {"),
		Label: "outer",
		Init:  &ir.Assign{LHS: []ir.Expr{ir.NewIdent("i", ir.TInt)}, RHS: []ir.Expr{ir.IntLit("0")}, Op: ":="},
		Cond:  ir.Bin("<", ir.NewIdent("i", ir.TInt), n, ir.TBool),
		Post:  &ir.IncDec{X: ir.NewIdent("i", ir.TInt)},
		Body: func() *ir.Block {
			b := &ir.Block{}
			b.Add(&ir.Branch{Kind: "continue", Label: "outer"})
			return b
		}(),
	}

	rng := &ir.Range{
		Meta:   prov(18, "foreach my $item (@items) {"),
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  ir.NewIdent("item", ir.TString),
		X:      items,
		Define: true,
		Body: func() *ir.Block {
			b := &ir.Block{}
			b.Add(&ir.ExprStmt{X: ir.CallOf(ir.Pkg("fmt", "fmt", "Println", nil), ir.TVoid, ir.NewIdent("item", ir.TString))})
			return b
		}(),
	}

	sw := &ir.Switch{
		Meta: prov(22, "for ($mode) { ... }"),
		Tag:  ir.NewIdent("mode", ir.TString),
		Cases: []ir.Case{
			{
				Values: []ir.Expr{ir.Str(`"read"`), ir.Str(`"write"`)},
				Body: func() *ir.Block {
					b := &ir.Block{}
					b.Add(&ir.ExprStmt{X: ir.CallOf(ir.NewIdent("open", nil), ir.TVoid)})
					return b
				}(),
			},
			{
				Body: func() *ir.Block {
					b := &ir.Block{}
					b.Add(&ir.ExprStmt{X: ir.CallOf(ir.NewIdent("closeAll", nil), ir.TVoid)})
					return b
				}(),
			},
		},
	}

	body := &ir.Block{}
	body.Add(
		&ir.If{
			Meta: prov(7, "if ($n < 10) {"),
			Cond: ir.Bin("<", n, ir.IntLit("10"), ir.TBool),
			Then: thenBlock,
			Else: elseIf,
		},
		loop,
		rng,
		sw,
		&ir.Defer{Call: ir.CallOf(ir.NewIdent("cleanup", nil), ir.TVoid)},
		&ir.Return{Results: []ir.Expr{ir.Str(`""`)}},
	)

	return &ir.File{
		Name:    "control.go",
		Package: "main",
		Decls: []ir.Decl{
			&ir.FuncDecl{
				Name:    "size",
				Params:  []ir.Param{{Name: "n", Type: ir.TInt}, {Name: "items", Type: ir.SliceOf(ir.TString)}, {Name: "mode", Type: ir.TString}},
				Results: []*ir.Type{ir.TString},
				Body:    body,
			},
		},
	}
}

// typesFile covers a type declaration, a var and a const, a composite
// literal, a function literal, a type assertion, a conversion, and the
// comma-ok index form.
func typesFile() *ir.File {
	record := &ir.TypeDecl{
		Meta: prov(2, "# a record"),
		Name: "Record",
		Doc:  []string{"Record is one row of the report."},
		Fields: []ir.Field{
			{Name: "Name", Type: ir.TString, Tag: `json:"name"`, Doc: "Name is the account name."},
			{Name: "Size", Type: ir.TInt},
		},
	}

	body := &ir.Block{}
	body.Add(
		&ir.DeclStmt{Names: []string{"seen"}, Type: ir.MapOf(ir.TBool)},
		&ir.Assign{
			Op:  ":=",
			LHS: []ir.Expr{ir.NewIdent("rec", nil)},
			RHS: []ir.Expr{&ir.CompositeLit{
				LitType: ir.NamedType("Record", ""),
				Keys:    []ir.Expr{ir.NewIdent("Name", nil), ir.NewIdent("Size", nil)},
				Elems:   []ir.Expr{ir.Str(`"root"`), ir.IntLit("1")},
			}},
		},
		&ir.Assign{
			Op:  ":=",
			LHS: []ir.Expr{ir.NewIdent("v", nil), ir.NewIdent("ok", nil)},
			RHS: []ir.Expr{&ir.IndexComma{X: ir.NewIdent("seen", nil), Index: ir.Str(`"root"`)}},
		},
		&ir.If{
			Cond: ir.NewIdent("ok", ir.TBool),
			Then: func() *ir.Block {
				b := &ir.Block{}
				b.Add(&ir.ExprStmt{X: ir.CallOf(ir.NewIdent("use", nil), ir.TVoid, ir.NewIdent("v", nil))})
				return b
			}(),
		},
		&ir.Assign{
			Op:  "=",
			LHS: []ir.Expr{ir.NewIdent("total", ir.TFloat)},
			RHS: []ir.Expr{&ir.Conversion{To: ir.TFloat, X: &ir.Selector{X: ir.NewIdent("rec", nil), Sel: "Size"}}},
		},
		&ir.Assign{
			Op:  ":=",
			LHS: []ir.Expr{ir.NewIdent("f", nil)},
			RHS: []ir.Expr{&ir.FuncLit{
				Params:  []ir.Param{{Name: "s", Type: ir.TString}},
				Results: []*ir.Type{ir.TInt},
				Body: func() *ir.Block {
					b := &ir.Block{}
					b.Add(&ir.Return{Results: []ir.Expr{ir.CallOf(ir.NewIdent("len", nil), ir.TInt, ir.NewIdent("s", ir.TString))}})
					return b
				}(),
			}},
		},
		&ir.ExprStmt{X: ir.CallOf(ir.NewIdent("keep", nil), ir.TVoid,
			&ir.TypeAssert{X: ir.NewIdent("any1", ir.TAny), Assert: ir.TString},
			ir.NewIdent("total", ir.TFloat),
			ir.CallOf(ir.NewIdent("f", nil), ir.TInt, ir.Str(`"x"`)),
		)},
	)

	return &ir.File{
		Name:    "types.go",
		Package: "report",
		Decls: []ir.Decl{
			&ir.ImportDecl{Path: "strings"},
			record,
			&ir.VarDecl{
				Names: []string{"limit"},
				Type:  ir.TInt,
				Values: []ir.Expr{
					ir.Bin("*", ir.IntLit("64"), ir.IntLit("1024"), ir.TInt),
				},
				Const: true,
				Doc:   []string{"limit is the largest report this tool will write."},
			},
			&ir.FuncDecl{
				Name:   "build",
				Recv:   &ir.Param{Name: "r", Type: ir.PointerTo(ir.NamedType("Record", ""))},
				Params: []ir.Param{{Name: "any1", Type: ir.TAny}, {Name: "total", Type: ir.TFloat}, {Name: "parts", Type: ir.TString, Variadic: true}},
				Body:   body,
			},
			&ir.RawDecl{
				Doc:    []string{"join is the one helper this file needs."},
				Source: "func join(parts []string) string {\n\treturn strings.Join(parts, \" \")\n}",
			},
		},
	}
}

// todoFile carries a Todo, a note, provenance, and a raw statement: the tree
// the banned-vocabulary test runs over.
func todoFile() *ir.File {
	body := &ir.Block{}
	body.Add(
		&ir.CommentStmt{Lines: []string{"the developer wrote this comment"}},
		&ir.TodoStmt{
			Meta: prov(11, "local $/ = undef;"),
			Info: ir.Todo{
				Code:    "P2G1301",
				Short:   "reading a whole file in one go is not implemented here",
				Message: "the input record separator cannot be converted faithfully",
				Perl:    "local $/ = undef;",
			},
			Panic: true,
		},
		&ir.RawStmt{
			Meta:   noted(12, "# raw", "This line is written out as it stands."),
			Source: "_ = 0",
		},
	)
	return &ir.File{
		Name:    "todo.go",
		Package: "main",
		Decls: []ir.Decl{
			&ir.FuncDecl{
				Meta: ir.Meta{
					Prov:  ir.Provenance{Line: 10, Col: 1, Text: "sub slurp {"},
					Notes: []ir.Note{{Text: "Perl's slurp mode has no direct equivalent, so the reader is told."}},
					Todo: &ir.Todo{
						Code:    "P2G1302",
						Short:   "this function does not read its input yet",
						Message: "the body of slurp could not be converted",
					},
				},
				Name: "slurp",
				Doc:  []string{"slurp reads a whole file."},
				Body: body,
			},
		},
	}
}
