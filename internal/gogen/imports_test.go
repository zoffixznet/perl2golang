package gogen

import (
	"strings"
	"testing"

	"perl2go/internal/ir"
)

// TestImportsRender covers the import block in every shape it takes, including
// the alias a second package with the same base name forces.
func TestImportsRender(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  string
	}{
		{
			name: "none",
			want: "",
		},
		{
			name:  "one is written on a single line",
			paths: []string{"fmt"},
			want:  "import \"fmt\"\n",
		},
		{
			name:  "several are grouped and sorted",
			paths: []string{"os", "fmt", "strings"},
			want:  "import (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)\n",
		},
		{
			name:  "third party follows the standard library",
			paths: []string{"example.com/x/y", "fmt"},
			want:  "import (\n\t\"fmt\"\n\n\t\"example.com/x/y\"\n)\n",
		},
		{
			name:  "a repeated path is registered once",
			paths: []string{"fmt", "fmt", "fmt"},
			want:  "import \"fmt\"\n",
		},
		{
			name:  "a clashing base name is aliased",
			paths: []string{"text/template", "html/template"},
			want:  "import (\n\ttemplate2 \"html/template\"\n\t\"text/template\"\n)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			im := NewImports()
			for _, p := range tt.paths {
				im.Add(p)
			}
			if got := im.Render(); got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

// TestImportAliasReachesTheCode checks that a selector uses the alias the
// import set handed out, rather than the package's own base name, which would
// refer to the wrong package.
func TestImportAliasReachesTheCode(t *testing.T) {
	f := &ir.File{
		Name:    "tpl.go",
		Package: "main",
		Decls: []ir.Decl{&ir.FuncDecl{
			Name: "render",
			Body: block(
				&ir.ExprStmt{X: ir.CallOf(ir.Pkg("text/template", "template", "New", nil), ir.TVoid, ir.Str(`"t"`))},
				&ir.ExprStmt{X: ir.CallOf(ir.Pkg("html/template", "template", "New", nil), ir.TVoid, ir.Str(`"h"`))},
			),
		}},
	}
	src, err := New(Clean).File(f)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	got := string(src)
	if !strings.Contains(got, `template2 "html/template"`) {
		t.Errorf("the clashing import was not aliased:\n%s", got)
	}
	if !strings.Contains(got, "template2.New(\"h\")") {
		t.Errorf("the selector did not use the alias:\n%s", got)
	}
	if !strings.Contains(got, "template.New(\"t\")") {
		t.Errorf("the first import lost its plain name:\n%s", got)
	}
}

// TestNoUnusedImports is the rule that makes the output compile: an import is
// registered only when something that needs it is actually written.
func TestNoUnusedImports(t *testing.T) {
	// A file whose only mention of a package sits in a declaration that is
	// never emitted must not import it.
	e := New(Clean)
	f := &ir.File{Name: "empty.go", Package: "main", Decls: nil}
	src, err := e.File(f)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if strings.Contains(string(src), "import") {
		t.Errorf("empty file gained an import:\n%s", src)
	}

	// A type mentioned by an emitted declaration does bring its import in.
	e2 := New(Clean)
	src2, err := e2.File(&ir.File{
		Name:    "t.go",
		Package: "main",
		Decls: []ir.Decl{&ir.VarDecl{
			Names: []string{"out"},
			Type:  ir.PointerTo(ir.NamedType("os.File", "os")),
		}},
	})
	if err != nil {
		t.Fatalf("%v\n%s", err, src2)
	}
	if !strings.Contains(string(src2), `import "os"`) {
		t.Errorf("a used type did not register its import:\n%s", src2)
	}
}

// TestImportDeclEmitsNothing checks the forced-import escape hatch: it changes
// the import block and leaves no trace of itself in the body.
func TestImportDeclEmitsNothing(t *testing.T) {
	src, err := New(Annotated).File(&ir.File{
		Name:    "imp.go",
		Package: "main",
		Decls: []ir.Decl{
			&ir.ImportDecl{Meta: prov(1, "use POSIX;"), Path: "os"},
			&ir.FuncDecl{Name: "main", Body: block()},
		},
	})
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	want := "package main\n\nimport \"os\"\n\nfunc main() {\n}\n"
	if string(src) != want {
		t.Errorf("got:\n%s\nwant:\n%s", src, want)
	}
}
