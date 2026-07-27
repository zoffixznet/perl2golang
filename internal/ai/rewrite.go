package ai

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// This file applies decisions. Nothing here reads a model's text as code: a
// rename is a pair of identifiers that this package substitutes at byte
// offsets the parser found, and a doc comment is a line of text this package
// inserts above a declaration. The model contributes words, never structure,
// which is why the diff is bounded by construction rather than by inspecting
// it afterwards.
//
// The rewrite is deliberately textual, driven by positions taken from the
// syntax tree rather than by reprinting the tree. Reprinting would move every
// comment in the file through go/printer's comment placement, and the
// annotated program is mostly comments. Splicing at offsets leaves every byte
// that is not an identifier exactly where it was.

// Rename is one accepted local rename.
type Rename struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// FieldName is one accepted field rename inside a struct type.
type FieldName struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// TypeNaming is one accepted struct naming decision.
type TypeNaming struct {
	Name    string      `json:"name"`
	NewName string      `json:"new_name"`
	Fields  []FieldName `json:"fields"`
}

// DocComment is one accepted doc comment.
type DocComment struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

// Decisions is the validated result of one structural call, in the form the
// rewriter applies. It is plain data, so the same decisions can be applied to
// two renderings of the same program and the two cannot drift apart.
type Decisions struct {
	Renames  []Rename     `json:"renames"`
	Types    []TypeNaming `json:"types"`
	Comments []DocComment `json:"comments"`
}

// Empty reports whether there is nothing to apply.
func (d Decisions) Empty() bool {
	return len(d.Renames) == 0 && len(d.Types) == 0 && len(d.Comments) == 0
}

// Count is how many individual decisions this holds.
func (d Decisions) Count() int {
	n := len(d.Renames) + len(d.Comments)
	for _, t := range d.Types {
		if t.NewName != "" && t.NewName != t.Name {
			n++
		}
		for _, f := range t.Fields {
			if f.Name != "" && f.Name != f.Key {
				n++
			}
		}
	}
	return n
}

// edit is one byte range of the source to replace.
type edit struct {
	from, to int
	text     string
}

// Apply rewrites src with the given decisions and returns the formatted
// result.
//
// A decision that does not fit this particular file is skipped rather than
// failing the whole apply, which is what lets one set of decisions be applied
// to both the clean and the annotated rendering of one program. The result is
// only source text; the caller still has to gate it.
func Apply(path, src string, d Decisions) (string, error) {
	if d.Empty() {
		return src, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return "", parseErrorFrom(err)
	}
	base := fset.File(file.Pos()).Base()
	offset := func(p token.Pos) int { return int(p) - base }

	values := map[string]string{}
	fields := map[string]string{}
	for _, r := range d.Renames {
		if r.Old != "" && r.New != "" && r.Old != r.New {
			values[r.Old] = r.New
		}
	}
	for _, t := range d.Types {
		if t.NewName != "" && t.NewName != t.Name {
			values[t.Name] = t.NewName
		}
		for _, f := range t.Fields {
			if f.Name != "" && f.Key != "" && f.Name != f.Key {
				fields[f.Key] = f.Name
			}
		}
	}

	edits := renameEdits(file, values, fields, offset)
	edits = append(edits, commentEdits(fset, file, src, d.Comments, offset)...)
	out := applyEdits(src, edits)
	return gofmt(out)
}

// renameEdits works out every identifier occurrence to substitute.
//
// The two maps apply in different positions and are kept apart for that
// reason. A value or type rename must not touch the selector half of x.Count,
// and a field rename must touch exactly that. Merging them would let a rename
// of a local called `name` rewrite every struct field called `name` in the
// file.
func renameEdits(file *ast.File, values, fields map[string]string, offset func(token.Pos) int) []edit {
	if len(values) == 0 && len(fields) == 0 {
		return nil
	}
	imports := importNames(file)

	// Positions that are a selector's right-hand half, a struct field
	// declaration, or a composite literal key. These are the only places a
	// field name can appear, and the first of them is the only place a name
	// can belong to a package rather than to this file.
	selectors := map[*ast.Ident]bool{}
	qualified := map[*ast.Ident]bool{}
	fieldNames := map[*ast.Ident]bool{}
	keys := map[*ast.Ident]bool{}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			selectors[node.Sel] = true
			if id, ok := node.X.(*ast.Ident); ok && imports[id.Name] {
				qualified[node.Sel] = true
			}
		case *ast.StructType:
			if node.Fields == nil {
				return true
			}
			for _, f := range node.Fields.List {
				for _, id := range f.Names {
					fieldNames[id] = true
				}
			}
		case *ast.CompositeLit:
			for _, elt := range node.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if id, ok := kv.Key.(*ast.Ident); ok {
					keys[id] = true
				}
			}
		}
		return true
	})

	var edits []edit
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		// A selector on an imported package is that package's business.
		if qualified[id] {
			return true
		}
		var replacement string
		switch {
		case selectors[id] || fieldNames[id]:
			replacement = fields[id.Name]
		case keys[id]:
			// A composite literal key is a field name in a struct literal and
			// an ordinary expression in a map literal. Which one it is follows
			// from which map the name is in, and a name can never be in both
			// because a replacement has to be absent from the whole file.
			if r, ok := fields[id.Name]; ok {
				replacement = r
			} else {
				replacement = values[id.Name]
			}
		default:
			replacement = values[id.Name]
		}
		if replacement == "" {
			return true
		}
		from := offset(id.Pos())
		edits = append(edits, edit{from: from, to: from + len(id.Name), text: replacement})
		return true
	})
	return edits
}

// commentEdits turns accepted doc comments into insertions above the
// declarations they describe. A declaration that already has a doc comment is
// left alone: the converter's own comment was written from the Perl and is
// better grounded than anything added here.
func commentEdits(fset *token.FileSet, file *ast.File, src string, comments []DocComment, offset func(token.Pos) int) []edit {
	if len(comments) == 0 {
		return nil
	}
	want := map[string]string{}
	for _, c := range comments {
		if c.Name != "" && c.Comment != "" {
			want[c.Name] = c.Comment
		}
	}

	var edits []edit
	insert := func(pos token.Pos, doc *ast.CommentGroup, names ...string) {
		if doc != nil {
			return
		}
		for _, name := range names {
			text, ok := want[name]
			if !ok {
				continue
			}
			delete(want, name)
			at := lineStart(src, offset(pos))
			edits = append(edits, edit{from: at, to: at, text: commentBlock(text)})
			return
		}
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			insert(d.Pos(), d.Doc, d.Name.Name)
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			var names []string
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					names = append(names, s.Name.Name)
				case *ast.ValueSpec:
					for _, id := range s.Names {
						names = append(names, id.Name)
					}
				}
			}
			insert(d.Pos(), d.Doc, names...)
		}
	}
	return edits
}

// lineStart is the offset of the beginning of the line an offset falls on, so
// an inserted comment lands above the declaration rather than inside it.
func lineStart(src string, off int) int {
	if off <= 0 || off > len(src) {
		return max(0, min(off, len(src)))
	}
	if i := strings.LastIndexByte(src[:off], '\n'); i >= 0 {
		return i + 1
	}
	return 0
}

// commentBlock renders one doc comment, wrapped so it reads in a terminal.
func commentBlock(text string) string {
	var b strings.Builder
	for _, line := range wrapWords(text, 72) {
		b.WriteString("// ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// wrapWords breaks a sentence into lines of at most width characters.
func wrapWords(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := []string{words[0]}
	for _, w := range words[1:] {
		last := len(lines) - 1
		if len(lines[last])+1+len(w) <= width {
			lines[last] += " " + w
			continue
		}
		lines = append(lines, w)
	}
	return lines
}

// applyEdits splices every edit into src, last first so earlier offsets stay
// valid.
func applyEdits(src string, edits []edit) string {
	if len(edits) == 0 {
		return src
	}
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].from > edits[j].from })
	var b strings.Builder
	b.Grow(len(src) + 256)
	end := len(src)
	var parts []string
	for _, e := range edits {
		if e.from < 0 || e.to > end || e.from > e.to {
			continue
		}
		parts = append(parts, src[e.to:end])
		parts = append(parts, e.text)
		end = e.from
	}
	parts = append(parts, src[:end])
	for i := len(parts) - 1; i >= 0; i-- {
		b.WriteString(parts[i])
	}
	return b.String()
}

// importNames is the set of identifiers that stand for an imported package, so
// a field rename never touches strings.Fields on its way past.
func importNames(file *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, imp := range file.Imports {
		name := strings.Trim(imp.Path.Value, `"`)
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		out[name] = true
	}
	return out
}

// checkRenamed proves that two files differ only in the ways a naming pass is
// allowed to differ.
//
// It is deliberately blunt: every string and numeric literal has to survive,
// because the literals are what the program prints; the imports have to be the
// same set, because a naming pass has no business adding one; and the number
// and kind of declarations has to match, because renaming a thing is not the
// same as adding or removing one.
func checkRenamed(baseline, candidate string) error {
	base, err := factsOf(baseline)
	if err != nil {
		return &RejectedError{Gate: "baseline", Reason: "the deterministic file does not parse", Err: err}
	}
	cand, err := factsOf(candidate)
	if err != nil {
		return &RejectedError{Gate: "parse", Reason: "the result does not parse", Err: err}
	}
	if len(base.decls) != len(cand.decls) {
		return &RejectedError{Gate: "declarations", Reason: fmt.Sprintf(
			"%d declarations became %d", len(base.decls), len(cand.decls))}
	}
	// Imports are checked before literals because an import path is a literal
	// too, and "the import os is not one the converter emitted" tells a reader
	// what happened where "the literal os was added" does not.
	for path := range cand.imports {
		if !base.imports[path] {
			return &RejectedError{Gate: "imports", Reason: fmt.Sprintf("the import %q is not one the converter emitted", path)}
		}
	}
	for path := range base.imports {
		if !cand.imports[path] {
			return &RejectedError{Gate: "imports", Reason: fmt.Sprintf("the import %q was dropped", path)}
		}
	}
	if !equalStrings(base.directives, cand.directives) {
		return &RejectedError{Gate: "directives", Reason: "the compiler directives or build tags changed"}
	}
	for kind, count := range cand.banned {
		if count > base.banned[kind] {
			return &RejectedError{Gate: "control flow", Reason: fmt.Sprintf(
				"the result introduces %s, which a naming pass may not add", kind)}
		}
	}
	if cand.errChecks < base.errChecks {
		return &RejectedError{Gate: "error handling", Reason: fmt.Sprintf(
			"%d error checks in the deterministic file became %d", base.errChecks, cand.errChecks)}
	}
	if !equalStrings(base.literals, cand.literals) {
		return &RejectedError{Gate: "literals", Reason: literalDelta(base.literals, cand.literals)}
	}
	return nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
