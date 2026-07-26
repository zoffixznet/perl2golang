// Package runtime keeps the helper functions a converted program may need
// and copies the ones it actually uses into it.
//
// The helpers live in the src directory as ordinary, compilable, unit-tested
// Go rather than as strings, so they can be read and reviewed like any other
// code. This package embeds that directory, parses it, and hands out the
// subset a program asks for together with everything that subset depends on
// and nothing else, so the converted program carries no dead code and no
// dependency outside the standard library.
package runtime

import (
	"embed"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
)

//go:embed src/*.go
var sources embed.FS

// decl is one top-level declaration copied out of the embedded sources.
// A single declaration can introduce several names, as `const ( a = iota; b )`
// does, in which case all of them refer to this one decl and asking for any
// of them emits the whole thing.
type decl struct {
	names   []string // every name the declaration introduces
	doc     string   // its doc comment, with the markers stripped
	source  string   // the declaration as written, doc comment included
	imports []string // import paths it uses, sorted
	deps    []string // helpers it refers to, sorted
	methods []*decl  // methods declared on it, when it is a type
	file    string   // the embedded file it came from

	node ast.Decl // kept only while the catalog is being built
}

// name is the declaration's primary name, the one it sorts under.
func (d *decl) name() string { return d.names[0] }

// needs lists every helper the declaration cannot compile without: the ones
// its own body refers to, plus the ones its methods refer to, because a type
// always travels with its methods.
func (d *decl) needs() []string {
	if len(d.methods) == 0 {
		return d.deps
	}
	needs := slices.Clone(d.deps)
	for _, m := range d.methods {
		needs = append(needs, m.deps...)
	}
	slices.Sort(needs)
	return slices.Compact(needs)
}

// uses lists every import path the declaration and its methods need.
func (d *decl) uses() []string {
	if len(d.methods) == 0 {
		return d.imports
	}
	uses := slices.Clone(d.imports)
	for _, m := range d.methods {
		uses = append(uses, m.imports...)
	}
	slices.Sort(uses)
	return slices.Compact(uses)
}

// catalog is the parsed form of the embedded sources.
type catalog struct {
	byName map[string]*decl
	names  []string // every helper name, sorted
}

// load parses the embedded sources once, on first use.
//
// It panics if they do not parse or if two files declare the same name. Both
// mean this package was built from a broken tree, since the same sources are
// compiled as package src by the same build.
var load = sync.OnceValue(func() *catalog {
	c, err := parseFS(sources, "src")
	if err != nil {
		panic("runtime: embedded helper sources are broken: " + err.Error())
	}
	return c
})

// parseFS reads every non-test Go file in one directory of fsys and returns
// the declarations they hold. Test files are skipped: the embed pattern picks
// them up, but they are scaffolding for the helpers, not helpers.
func parseFS(fsys fs.FS, dir string) (*catalog, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	c := &catalog{byName: make(map[string]*decl)}
	var all []*decl     // every declaration, in file then source order
	var methods []*decl // methods, held back until their type is known
	owner := make(map[*decl]string)

	for _, entry := range entries {
		name := path.Join(dir, entry.Name())
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		text, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(fset, name, text, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}

		// An import is referred to by its package name, which is the last
		// element of its path unless the file gives it another one.
		packages := make(map[string]string, len(file.Imports))
		for _, spec := range file.Imports {
			p, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			local := path.Base(p)
			if spec.Name != nil {
				local = spec.Name.Name
			}
			packages[local] = p
		}

		for _, node := range file.Decls {
			d := newDecl(fset, text, node, packages)
			if d == nil {
				continue
			}
			d.file = name
			if fn, ok := node.(*ast.FuncDecl); ok && fn.Recv != nil {
				owner[d] = receiverName(fn.Recv)
				methods = append(methods, d)
				all = append(all, d)
				continue
			}
			for _, n := range d.names {
				if other, ok := c.byName[n]; ok {
					return nil, fmt.Errorf("%s: %q is already declared in %s", name, n, other.file)
				}
				c.byName[n] = d
			}
			all = append(all, d)
		}
	}

	// Methods belong to their receiver type, so that asking for the type
	// brings them along and nothing asks for them by themselves.
	for _, m := range methods {
		host := c.byName[owner[m]]
		if host == nil {
			return nil, fmt.Errorf("method %s has no declared receiver type %q", m.name(), owner[m])
		}
		host.methods = append(host.methods, m)
	}

	// Dependencies can only be worked out once every name is known.
	known := make(map[string]bool, len(c.byName))
	for n := range c.byName {
		known[n] = true
	}
	for _, d := range all {
		d.deps = helperRefs(d.node, known, d.names)
		d.node = nil
		slices.SortFunc(d.methods, func(a, b *decl) int { return strings.Compare(a.name(), b.name()) })
	}

	c.names = make([]string, 0, len(c.byName))
	for n := range c.byName {
		c.names = append(c.names, n)
	}
	slices.Sort(c.names)
	return c, nil
}

// newDecl records one top-level declaration. It returns nil for imports and
// for declarations that introduce no name a caller could ask for.
func newDecl(fset *token.FileSet, text []byte, node ast.Decl, packages map[string]string) *decl {
	d := &decl{node: node}
	var doc *ast.CommentGroup

	switch n := node.(type) {
	case *ast.FuncDecl:
		doc = n.Doc
		d.names = []string{n.Name.Name}
	case *ast.GenDecl:
		if n.Tok == token.IMPORT {
			return nil
		}
		doc = n.Doc
		for _, spec := range n.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				d.names = append(d.names, s.Name.Name)
			case *ast.ValueSpec:
				for _, ident := range s.Names {
					if ident.Name != "_" {
						d.names = append(d.names, ident.Name)
					}
				}
			}
		}
	default:
		return nil
	}
	if len(d.names) == 0 {
		return nil
	}

	start := node.Pos()
	if doc != nil {
		start = doc.Pos()
		d.doc = doc.Text()
	}
	from := fset.Position(start).Offset
	to := fset.Position(node.End()).Offset
	d.source = string(text[from:to])
	d.imports = importsOf(node, packages)
	return d
}

// receiverName is the name of the type a method is declared on, with any
// pointer and type arguments stripped off.
func receiverName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	for t := recv.List[0].Type; ; {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
		case *ast.IndexExpr:
			t = x.X
		case *ast.IndexListExpr:
			t = x.X
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}

// importsOf lists the import paths a declaration uses, by looking for
// qualified names whose qualifier is one of the file's imports.
func importsOf(node ast.Decl, packages map[string]string) []string {
	var used []string
	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); ok {
			if p, ok := packages[ident.Name]; ok {
				used = append(used, p)
			}
		}
		return true
	})
	slices.Sort(used)
	return slices.Compact(used)
}

// helperRefs lists the helpers a declaration refers to. Qualified names are
// not helpers, since their meaning comes from another package, and neither
// are the names the declaration introduces itself.
func helperRefs(node ast.Decl, known map[string]bool, own []string) []string {
	var refs []string
	var visit func(ast.Node) bool
	visit = func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			// Only the qualifier can name a helper; the selected name is a
			// field or a member of another package.
			ast.Inspect(x.X, visit)
			return false
		case *ast.Field:
			// Field, parameter and result names are declarations, not uses.
			if x.Type != nil {
				ast.Inspect(x.Type, visit)
			}
			return false
		case *ast.Ident:
			if known[x.Name] && !slices.Contains(own, x.Name) {
				refs = append(refs, x.Name)
			}
		}
		return true
	}
	ast.Inspect(node, visit)
	slices.Sort(refs)
	return slices.Compact(refs)
}

// Names returns every helper this package can emit, sorted.
func Names() []string { return slices.Clone(load().names) }

// Has reports whether name is a helper this package can emit.
func Has(name string) bool {
	_, ok := load().byName[name]
	return ok
}

// Doc returns the doc comment of the named helper, with the comment markers
// stripped, or the empty string when there is no such helper. Declarations
// that introduce several names share one doc comment.
func Doc(name string) string {
	if d, ok := load().byName[name]; ok {
		return d.doc
	}
	return ""
}

// Note returns the short explanation of why the named helper exists at all:
// the rule it implements and the plain Go that would get that rule wrong. It
// is empty when there is no such helper, or no note for it.
func Note(name string) string {
	if d, ok := load().byName[name]; ok {
		return notes[d.name()]
	}
	return ""
}

// Emit returns a single formatted Go source file, in package pkg, holding the
// named helpers and everything they depend on. The file imports exactly what
// those helpers use and declares nothing else. Output is deterministic: the
// same names in any order produce the same bytes.
//
// It returns nil, nil when names is empty, so a program that needs no helper
// gets no file at all.
func Emit(names []string, pkg string) ([]byte, error) {
	return load().emit(names, pkg, false)
}

// EmitAnnotated is Emit with a leading comment that explains, helper by
// helper, which rule of the original program each one implements and what the
// obvious Go would have got wrong. The helpers themselves, doc comments and
// all, are identical to the ones Emit returns.
func EmitAnnotated(names []string, pkg string) ([]byte, error) {
	return load().emit(names, pkg, true)
}

func (c *catalog) emit(names []string, pkg string, annotate bool) ([]byte, error) {
	if len(names) == 0 {
		return nil, nil
	}
	if !token.IsIdentifier(pkg) || token.IsKeyword(pkg) {
		return nil, fmt.Errorf("runtime: %q is not a usable package name", pkg)
	}
	wanted, err := c.resolve(names)
	if err != nil {
		return nil, err
	}

	var out strings.Builder
	if annotate {
		out.WriteString(annotatedHeader(wanted))
	} else {
		out.WriteString(plainHeader)
	}
	out.WriteString("\npackage " + pkg + "\n")

	var imports []string
	for _, d := range wanted {
		imports = append(imports, d.uses()...)
	}
	slices.Sort(imports)
	imports = slices.Compact(imports)
	switch len(imports) {
	case 0:
	case 1:
		out.WriteString("\nimport " + strconv.Quote(imports[0]) + "\n")
	default:
		out.WriteString("\nimport (\n")
		for _, p := range imports {
			out.WriteString("\t" + strconv.Quote(p) + "\n")
		}
		out.WriteString(")\n")
	}

	for _, d := range wanted {
		out.WriteString("\n" + d.source + "\n")
		for _, m := range d.methods {
			out.WriteString("\n" + m.source + "\n")
		}
	}

	formatted, err := format.Source([]byte(out.String()))
	if err != nil {
		return nil, fmt.Errorf("runtime: emitted helpers do not parse: %w", err)
	}
	return formatted, nil
}

// resolve returns the named declarations and everything they depend on,
// sorted by name. A dependency cycle is legal in Go and resolves fine here,
// because a declaration already collected is never visited again.
func (c *catalog) resolve(names []string) ([]*decl, error) {
	roots := make([]*decl, 0, len(names))
	for _, name := range names {
		d, ok := c.byName[name]
		if !ok {
			return nil, fmt.Errorf("runtime: no helper named %q", name)
		}
		roots = append(roots, d)
	}

	var wanted []*decl
	collected := make(map[*decl]bool, len(roots))
	var collect func(*decl)
	collect = func(d *decl) {
		if collected[d] {
			return
		}
		collected[d] = true
		wanted = append(wanted, d)
		for _, need := range d.needs() {
			if dep, ok := c.byName[need]; ok {
				collect(dep)
			}
		}
	}
	for _, d := range roots {
		collect(d)
	}

	slices.SortFunc(wanted, func(a, b *decl) int { return strings.Compare(a.name(), b.name()) })
	return wanted, nil
}

// plainHeader introduces the helpers in the program a reader runs. It says
// what the file is and stays out of the way.
const plainHeader = "// Small helpers used by this program.\n"

// annotatedHeader introduces the same helpers in the annotated program, where
// the point is to teach: it names the rule each helper implements and the
// plain Go that would break it.
func annotatedHeader(wanted []*decl) string {
	var out strings.Builder
	out.WriteString(`// Small helpers used by this program.
//
// Perl and Go disagree about a handful of everyday operations, and each
// helper below settles one of those disagreements the way the original
// program did. The note with each one names the shorter Go that would have
// been wrong, so that a reader can tell a deliberate helper from a detour.
`)
	for _, d := range wanted {
		note := notes[d.name()]
		if note == "" {
			continue
		}
		out.WriteString("//\n// " + d.name() + "\n")
		for _, line := range wrap(note, 68) {
			out.WriteString("//     " + line + "\n")
		}
	}
	return out.String()
}

// wrap breaks text into lines of at most width characters, splitting only at
// spaces. A word longer than width gets a line of its own.
func wrap(text string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
