package runtime

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestNamesAreSortedAndComplete(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Fatal("Names returned nothing; the embedded sources did not load")
	}
	if !slices.IsSorted(names) {
		t.Errorf("Names is not sorted: %v", names)
	}
	if slices.Compact(slices.Clone(names)) == nil || len(slices.Compact(slices.Clone(names))) != len(names) {
		t.Errorf("Names repeats itself: %v", names)
	}
	for _, want := range []string{"parseNum", "formatNum", "splitPattern", "sprintf", "truthy"} {
		if !slices.Contains(names, want) {
			t.Errorf("Names is missing %q", want)
		}
		if !Has(want) {
			t.Errorf("Has(%q) = false", want)
		}
	}
	if Has("noSuchHelper") {
		t.Error(`Has("noSuchHelper") = true`)
	}

	// The embed pattern picks up the helpers' own test file. Nothing in it
	// may become a helper, not even the declarations that do not look like
	// tests: stringer is one such type in src_test.go.
	for _, name := range names {
		if strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") {
			t.Errorf("%q comes from a test file and must not be a helper", name)
		}
	}
	if Has("stringer") {
		t.Error("a declaration from the helpers' test file leaked into the catalog")
	}
}

func TestNamesIsACopy(t *testing.T) {
	names := Names()
	names[0] = "clobbered"
	if got := Names()[0]; got == "clobbered" {
		t.Error("Names hands out the catalog's own slice")
	}
}

func TestDocAndNoteCoverEveryHelper(t *testing.T) {
	for _, name := range Names() {
		doc := Doc(name)
		switch {
		case doc == "":
			t.Errorf("%s has no doc comment", name)
		case !strings.HasPrefix(doc, name+" "):
			t.Errorf("%s: doc comment should start with the name, got %q", name, firstLine(doc))
		}
		if Note(name) == "" {
			t.Errorf("%s has no note in notes.go", name)
		}
	}
	if Doc("noSuchHelper") != "" {
		t.Error("Doc of an unknown helper should be empty")
	}
	if Note("noSuchHelper") != "" {
		t.Error("Note of an unknown helper should be empty")
	}
	for name := range notes {
		if !Has(name) {
			t.Errorf("notes.go describes %q, which is not a helper", name)
		}
	}
}

func TestEmitNothingForNoNames(t *testing.T) {
	for _, names := range [][]string{nil, {}} {
		for _, emit := range []func([]string, string) ([]byte, error){Emit, EmitAnnotated} {
			out, err := emit(names, "main")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if out != nil {
				t.Errorf("expected no file, got %q", out)
			}
		}
	}
}

func TestEmitRejectsBadInput(t *testing.T) {
	if _, err := Emit([]string{"noSuchHelper"}, "main"); err == nil {
		t.Error("expected an error for an unknown helper")
	} else if !strings.Contains(err.Error(), "noSuchHelper") {
		t.Errorf("error should name the helper, got %v", err)
	}
	for _, pkg := range []string{"", "not a package", "func", "1st"} {
		if _, err := Emit([]string{"mod"}, pkg); err == nil {
			t.Errorf("expected an error for package name %q", pkg)
		}
	}
}

func TestEmitClosesOverDependencies(t *testing.T) {
	tests := []struct {
		names []string
		want  []string
	}{
		{[]string{"mod"}, []string{"mod"}},
		{[]string{"truthy"}, []string{"truthy"}},
		{[]string{"chop"}, []string{"chop"}},
		{[]string{"splitPattern"}, []string{"splitPattern"}},
		{[]string{"indexOf"}, []string{"indexOf"}},
		{[]string{"lastIndexOf"}, []string{"lastIndexOf"}},
		{[]string{"at"}, []string{"at"}},
		{[]string{"seq"}, []string{"seq"}},
		{[]string{"toText"}, []string{"formatNum", "toText"}},
		{[]string{"strInc"}, []string{"formatNum", "magicStr", "parseNum", "strInc"}},
		{[]string{"toNum"}, []string{"formatNum", "parseNum", "toNum", "toText"}},
		{[]string{"sprintf"}, []string{"formatNum", "parseNum", "sprintf", "toNum", "toText"}},
		{[]string{"isTrue"}, []string{"isTrue", "truthy"}},
		{[]string{"joinList"}, []string{"formatNum", "joinList", "toText"}},
		{[]string{"strRange"}, []string{"formatNum", "magicStr", "parseNum", "strInc", "strRange"}},
		{[]string{"mod", "truthy"}, []string{"mod", "truthy"}},
		{[]string{"toText", "formatNum"}, []string{"formatNum", "toText"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.names, "+"), func(t *testing.T) {
			out, err := Emit(tt.names, "main")
			if err != nil {
				t.Fatal(err)
			}
			if got := declared(t, out); !slices.Equal(got, tt.want) {
				t.Errorf("emitted %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmitImportsExactlyWhatItUses(t *testing.T) {
	tests := []struct {
		names []string
		want  []string
	}{
		{[]string{"mod"}, nil},
		{[]string{"powInt"}, nil},
		{[]string{"repeatList"}, nil},
		{[]string{"seq"}, nil},
		{[]string{"at"}, nil},
		{[]string{"magicStr"}, nil},
		{[]string{"chop"}, []string{"unicode/utf8"}},
		{[]string{"splitPattern"}, []string{"regexp"}},
		{[]string{"reverseStr"}, []string{"slices"}},
		{[]string{"isTrue"}, []string{"reflect"}},
		{[]string{"ucFirst"}, []string{"unicode", "unicode/utf8"}},
		{[]string{"joinList"}, []string{"fmt", "math", "strconv", "strings"}},
		{[]string{"strInc"}, []string{"math", "strconv", "strings"}},
		{[]string{"strRange"}, []string{"math", "strconv", "strings"}},
		{[]string{"sprintf"}, []string{"fmt", "math", "strconv", "strings"}},
		{[]string{"chop", "mod"}, []string{"unicode/utf8"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.names, "+"), func(t *testing.T) {
			out, err := Emit(tt.names, "main")
			if err != nil {
				t.Fatal(err)
			}
			if got := imported(t, out); !slices.Equal(got, tt.want) {
				t.Errorf("imports %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmitIsDeterministic(t *testing.T) {
	names := []string{"sprintf", "splitPattern", "chop"}
	first, err := Emit(names, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range [][]string{
		{"sprintf", "splitPattern", "chop"},
		{"chop", "splitPattern", "sprintf"},
		{"splitPattern", "chop", "sprintf", "chop"},
		// Naming a dependency explicitly changes nothing.
		{"splitPattern", "chop", "sprintf", "parseNum", "formatNum"},
	} {
		out, err := Emit(other, "main")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, out) {
			t.Errorf("Emit(%v) differs from Emit(%v)", other, names)
		}
	}
}

func TestEmitIsFormattedGo(t *testing.T) {
	for name, emit := range map[string]func([]string, string) ([]byte, error){
		"Emit":          Emit,
		"EmitAnnotated": EmitAnnotated,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := emit(Names(), "helpers")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), "helpers.go", out, parser.AllErrors); err != nil {
				t.Fatalf("emitted file does not parse: %v", err)
			}
			formatted, err := format.Source(out)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, formatted) {
				t.Error("emitted file is not gofmt clean")
			}
			if !bytes.HasSuffix(out, []byte("\n")) {
				t.Error("emitted file does not end in a newline")
			}
			if !bytes.Contains(out, []byte("\npackage helpers\n")) {
				t.Error("emitted file is not in the requested package")
			}
		})
	}
}

// The plain file ships inside a program that has nothing to do with this
// tool, so nothing in it may say where it came from: not the header, not a
// doc comment, and not an identifier a call site will spell out.
func TestEmitSaysNothingAboutWhereItCameFrom(t *testing.T) {
	out, err := Emit(Names(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if got := firstLine(string(out)); got != "// Small helpers used by this program." {
		t.Errorf("first line is %q", got)
	}
	lower := strings.ToLower(string(out))
	for _, word := range []string{"perl", "convert", "translat", "generated"} {
		if i := strings.Index(lower, word); i >= 0 {
			t.Errorf("emitted file says %q: %s", word, firstLine(string(out)[max(0, i-40):]))
		}
	}
	// The same words must not reach a call site either, which means no
	// helper may be named after them.
	for _, name := range Names() {
		lower := strings.ToLower(name)
		for _, word := range []string{"perl", "convert", "translat", "generated"} {
			if strings.Contains(lower, word) {
				t.Errorf("helper %q is named after %q", name, word)
			}
		}
	}
}

func TestEmitAnnotatedTeachesAndStillCompilesTheSameCode(t *testing.T) {
	names := []string{"mod", "sprintf"}
	plain, err := Emit(names, "main")
	if err != nil {
		t.Fatal(err)
	}
	annotated, err := EmitAnnotated(names, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(annotated, []byte("Perl")) {
		t.Error("annotated output should explain the Perl rules it implements")
	}
	for _, name := range declared(t, annotated) {
		if !bytes.Contains(annotated, []byte("\n// "+name+"\n")) {
			t.Errorf("annotated output has no note for %s", name)
		}
	}
	if !slices.Equal(declared(t, plain), declared(t, annotated)) {
		t.Error("annotated output declares different helpers")
	}
	if !slices.Equal(imported(t, plain), imported(t, annotated)) {
		t.Error("annotated output imports different packages")
	}
	// Both files carry the helpers' own doc comments and identical bodies;
	// only the leading block differs.
	if body := annotated[bytes.Index(annotated, []byte("\npackage main\n")):]; !bytes.Equal(body, plain[bytes.Index(plain, []byte("\npackage main\n")):]) {
		t.Error("annotated output changed the helpers themselves")
	}
}

// TestEmittedHelpersBuild is the test that matters: an unused import is a
// compile error in Go, so only the real toolchain can prove that Emit gets
// the import list exactly right. Every helper is built on its own as well as
// all together, because an import that one helper needs is unused in a file
// that does not have that helper.
func TestEmittedHelpersBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a temporary module")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go toolchain on PATH")
	}

	dir := t.TempDir()
	write := func(pkg string, out []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, pkg), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, pkg, "helpers.go"), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	emit := func(pkg string, names []string, annotate bool) {
		t.Helper()
		out, err := Emit(names, pkg)
		if annotate {
			out, err = EmitAnnotated(names, pkg)
		}
		if err != nil {
			t.Fatal(err)
		}
		write(pkg, out)
	}

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module helpers\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i, name := range Names() {
		emit(fmt.Sprintf("one%02d", i), []string{name}, false)
	}
	emit("all", Names(), false)
	emit("annotated", Names(), true)

	cmd := exec.Command(goBin, "build", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GO111MODULE=on")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./...: %v\n%s", err, out)
	}
}

// The remaining tests drive the parser with fixtures rather than the real
// helpers, so that shapes the helpers do not use yet are still covered.

func TestParseFSKeepsMethodsWithTheirType(t *testing.T) {
	c := fixture(t, fstest.MapFS{
		"src/counter.go": file(`
// Counter counts.
type Counter struct{ n int }

// Bump raises the count.
func (c *Counter) Bump() int {
	c.n += step()
	return c.n
}

// step is how much a bump is worth.
func step() int { return 1 }
`),
	})
	if _, ok := c.byName["Bump"]; ok {
		t.Error("a method should not be a helper of its own")
	}
	out, err := c.emit([]string{"Counter"}, "main", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type Counter struct", "func (c *Counter) Bump()", "func step() int"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("emitted file is missing %q:\n%s", want, out)
		}
	}
}

func TestParseFSHandlesCyclesAndGroups(t *testing.T) {
	c := fixture(t, fstest.MapFS{
		"src/cycle.go": file(`
// alpha calls beta.
func alpha(n int) int { return beta(n - 1) }

// beta calls alpha right back.
func beta(n int) int {
	if n <= 0 {
		return 0
	}
	return alpha(n)
}
`),
		"src/group.go": file(`
// first and second are declared together and travel together.
const (
	first = iota
	second
)
`),
	})

	out, err := c.emit([]string{"alpha"}, "main", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := declared(t, out); !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Errorf("a cycle emitted %v, want [alpha beta]", got)
	}

	out, err = c.emit([]string{"second"}, "main", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "first = iota") {
		t.Errorf("a grouped declaration must be emitted whole:\n%s", out)
	}
}

func TestParseFSRejectsDuplicateNames(t *testing.T) {
	_, err := parseFS(fstest.MapFS{
		"src/a.go": file("func twice() {}"),
		"src/b.go": file("func twice() {}"),
	}, "src")
	if err == nil {
		t.Fatal("expected an error for a name declared twice")
	}
	if !strings.Contains(err.Error(), "twice") {
		t.Errorf("error should name the declaration, got %v", err)
	}
}

func TestParseFSSkipsTestFiles(t *testing.T) {
	c := fixture(t, fstest.MapFS{
		"src/a.go":      file("func real() {}"),
		"src/a_test.go": file("func pretend() {}"),
	})
	if _, ok := c.byName["pretend"]; ok {
		t.Error("a declaration from a test file became a helper")
	}
	if _, ok := c.byName["real"]; !ok {
		t.Error("the ordinary declaration went missing")
	}
}

func TestParseFSReadsImportsPerDeclaration(t *testing.T) {
	c := fixture(t, fstest.MapFS{
		"src/a.go": rawFile(`package src

import (
	"strings"
	"unicode"
)

// upper needs one import.
func upper(s string) string { return strings.ToUpper(s) }

// letter needs the other, and a field named after an import must not be
// mistaken for one.
func letter(r rune) bool {
	type strings struct{ unicode int }
	_ = strings{unicode: 1}
	return unicode.IsLetter(r)
}
`),
	})
	if got := c.byName["upper"].imports; !slices.Equal(got, []string{"strings"}) {
		t.Errorf("upper imports %v", got)
	}
	if got := c.byName["letter"].imports; !slices.Equal(got, []string{"unicode"}) {
		t.Errorf("letter imports %v", got)
	}
}

func fixture(t *testing.T, files fstest.MapFS) *catalog {
	t.Helper()
	c, err := parseFS(files, "src")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// file wraps a fixture body in a package clause.
func file(body string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("package src\n" + body)}
}

// rawFile takes a fixture that brings its own package clause and imports.
func rawFile(text string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(text)}
}

// declared lists the top-level names an emitted file declares, sorted.
func declared(t *testing.T, src []byte) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "helpers.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("emitted file does not parse: %v\n%s", err, src)
	}
	var names []string
	for _, d := range f.Decls {
		switch n := d.(type) {
		case *ast.FuncDecl:
			if n.Recv == nil {
				names = append(names, n.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range n.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					names = append(names, s.Name.Name)
				case *ast.ValueSpec:
					for _, ident := range s.Names {
						names = append(names, ident.Name)
					}
				}
			}
		}
	}
	slices.Sort(names)
	return names
}

// imported lists the import paths an emitted file declares, sorted.
func imported(t *testing.T, src []byte) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "helpers.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("emitted file does not parse: %v\n%s", err, src)
	}
	var paths []string
	for _, spec := range f.Imports {
		paths = append(paths, strings.Trim(spec.Path.Value, `"`))
	}
	slices.Sort(paths)
	return paths
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
