package ai

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// The model never chooses what to rename. This file does, deterministically,
// from the syntax of the file the converter produced.
//
// That split is what keeps the failure mode small. A model asked "improve this
// file" can do anything; a model handed a list of five names and asked what to
// call them can only answer badly, and a bad answer is caught by the checks
// that follow. Everything below is about producing that list.

// maxRenameTargets caps how many names one file offers. A file with fifty weak
// names is a file the converter should be fixed for, and a prompt that long
// stops being answered well.
const maxRenameTargets = 24

// maxUsageLines is how many lines of context each target carries. The model
// infers a name from how the variable is used, so the use sites are the whole
// input; more than a handful is noise.
const maxUsageLines = 6

// RenameTarget is one local name the converter produced that reads as
// generated, together with the lines that show what it holds.
type RenameTarget struct {
	// Name is the current identifier.
	Name string
	// Kind says how it was declared, for the prompt: "variable", "loop
	// variable", "parameter".
	Kind string
	// TypeHint is the declared or obvious Go type, empty when there is none.
	TypeHint string
	// Usage are the source lines the identifier appears on, trimmed.
	Usage []string
	// Func is the enclosing function, for the prompt.
	Func string
}

// ShapeTarget is one struct type whose name the converter had to invent.
type ShapeTarget struct {
	// Name is the current type name.
	Name string
	// Fields are the current field names, in declaration order.
	Fields []string
	// FieldTypes are the fields' Go types, aligned with Fields.
	FieldTypes []string
	// Usage are the source lines the type or its fields appear on.
	Usage []string
}

// CommentTarget is one top-level declaration with no doc comment.
type CommentTarget struct {
	// Name is the declared identifier.
	Name string
	// Kind is "function", "method", "type", "variable" or "constant".
	Kind string
	// Decl is the declaration line, which is what the comment must describe.
	Decl string
	// Exported reports whether the identifier is part of the package's API,
	// which is where a missing doc comment actually costs something.
	Exported bool
	// Body are the first few lines of the declaration's body, for grounding.
	Body []string
}

// targets is everything one file offers a model.
type targets struct {
	renames  []RenameTarget
	shapes   []ShapeTarget
	comments []CommentTarget

	// file is the parsed source, kept so the caller can apply decisions
	// without parsing twice.
	file *ast.File
	fset *token.FileSet
	// taken is every identifier that appears anywhere in the file. A new name
	// has to be absent from it.
	taken map[string]bool
	// declCount counts declarations per name, which is what makes a
	// whole-file rename provably safe.
	declCount map[string]int
	// imports is the set of identifiers that name an imported package.
	imports map[string]bool
	// fields is the set of names declared as struct fields.
	fields map[string]bool
}

// syntheticSuffix matches the numbered names a generator produces when it needs
// a fresh identifier: item4, pattern22, content2. A person writing Go does not
// number their variables, so the digit is a reliable tell.
var syntheticSuffix = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*[0-9]+$`)

// validGoName is the shape a replacement name has to have: lower camel case,
// no underscores, no digits, which is what the surrounding Go already looks
// like.
var validGoName = regexp.MustCompile(`^[a-z][a-zA-Z]*$`)

// validTypeName is the same rule for an exported type or field name.
var validTypeName = regexp.MustCompile(`^[A-Z][a-zA-Z]*$`)

// neverRename are the names that are already the right name. err and ok are
// Go's own conventions, and renaming either makes the code worse.
var neverRename = map[string]bool{
	"_": true, "err": true, "ok": true, "ctx": true, "nil": true,
}

// vacuumNames are names that say nothing about what they hold. They are weak
// however long they are.
var vacuumNames = map[string]bool{
	"tmp": true, "temp": true, "val": true, "value": true, "data": true,
	"arr": true, "res": true, "result": true, "obj": true, "item": true,
	"elem": true, "buf": true, "str": true, "num": true, "ret": true,
	"out": true, "aux": true, "thing": true, "stuff": true, "info": true,
	"list": true, "sorted": true, "matched": true, "found": true, "byKey": true,
	"entry": true, "record": true, "field": true, "part": true,
}

// predeclared are the identifiers Go defines for every file. A new name that
// shadows one of these compiles and then misleads, so they are refused.
var predeclared = map[string]bool{
	"any": true, "bool": true, "byte": true, "comparable": true, "complex64": true,
	"complex128": true, "error": true, "float32": true, "float64": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"rune": true, "string": true, "uint": true, "uint8": true, "uint16": true,
	"uint32": true, "uint64": true, "uintptr": true,
	"true": true, "false": true, "iota": true, "nil": true,
	"append": true, "cap": true, "clear": true, "close": true, "complex": true,
	"copy": true, "delete": true, "imag": true, "len": true, "make": true,
	"max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true,
}

// goKeywords cannot be identifiers at all.
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// weakName reports whether a local name reads as machine-chosen.
func weakName(name string) bool {
	switch {
	case neverRename[name]:
		return false
	case vacuumNames[name]:
		return true
	case syntheticSuffix.MatchString(name):
		return true
	case len(name) <= 2:
		return true
	}
	return false
}

// weakTypeName reports whether a type name was invented rather than derived
// from anything meaningful.
func weakTypeName(name string) bool {
	if syntheticSuffix.MatchString(name) {
		return true
	}
	lower := strings.ToLower(name)
	for _, stem := range []string{"record", "shape", "entry", "item", "struct", "row", "obj", "value"} {
		if lower == stem {
			return true
		}
	}
	return false
}

// scanTargets parses one generated Go file and works out what a model may be
// asked about. It returns an error only when the file does not parse, which
// means the converter produced something broken and there is nothing to
// improve.
func scanTargets(path, src string, jobs JobSet) (*targets, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, parseErrorFrom(err)
	}
	t := &targets{
		file:      file,
		fset:      fset,
		taken:     map[string]bool{},
		declCount: map[string]int{},
		imports:   map[string]bool{},
		fields:    map[string]bool{},
	}
	lines := strings.Split(src, "\n")

	for _, imp := range file.Imports {
		name := strings.Trim(imp.Path.Value, `"`)
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		t.imports[name] = true
	}

	// Two passes. The first counts every identifier and every declaration,
	// because the safety argument for a rename is "this name is declared once
	// and the replacement appears nowhere". The second picks the targets.
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.Ident:
			t.taken[node.Name] = true
		case *ast.FuncDecl:
			t.declCount[node.Name.Name]++
		case *ast.TypeSpec:
			t.declCount[node.Name.Name]++
		case *ast.ValueSpec:
			for _, id := range node.Names {
				t.declCount[id.Name]++
			}
		case *ast.AssignStmt:
			if node.Tok != token.DEFINE {
				return true
			}
			for _, lhs := range node.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					t.declCount[id.Name]++
				}
			}
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{node.Key, node.Value} {
				if id, ok := e.(*ast.Ident); ok && node.Tok == token.DEFINE {
					t.declCount[id.Name]++
				}
			}
		case *ast.Field:
			for _, id := range node.Names {
				t.declCount[id.Name]++
			}
		case *ast.LabeledStmt:
			t.declCount[node.Label.Name]++
		}
		return true
	})

	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, f := range st.Fields.List {
					for _, id := range f.Names {
						t.fields[id.Name] = true
					}
				}
			}
		}
	}

	if jobs.Has(JobRename) {
		t.renames = t.scanRenames(lines)
	}
	if jobs.Has(JobShapeNaming) {
		t.shapes = t.scanShapes(lines)
	}
	if jobs.Has(JobDocComments) {
		t.comments = t.scanComments(lines)
	}
	return t, nil
}

// empty reports whether there is nothing worth asking about.
func (t *targets) empty() bool {
	return len(t.renames) == 0 && len(t.shapes) == 0 && len(t.comments) == 0
}

// scanRenames collects the weak local names, with the lines that show how each
// is used.
func (t *targets) scanRenames(lines []string) []RenameTarget {
	seen := map[string]*RenameTarget{}
	var order []string

	add := func(id *ast.Ident, kind, typeHint, fn string) {
		name := id.Name
		if !weakName(name) || t.fields[name] || t.declCount[name] != 1 {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = &RenameTarget{Name: name, Kind: kind, TypeHint: typeHint, Func: fn}
		order = append(order, name)
	}

	for _, decl := range t.file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		name := fn.Name.Name
		// Loop indices are skipped: `for i := 0; i < n; i++` and `for i :=
		// range xs` name i by convention, and a longer name there reads worse,
		// not better.
		indices := loopIndices(fn.Body)

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				if node.Tok != token.DEFINE {
					return true
				}
				for _, lhs := range node.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && !indices[id] {
						add(id, "variable", "", name)
					}
				}
			case *ast.DeclStmt:
				gd, ok := node.Decl.(*ast.GenDecl)
				if !ok {
					return true
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					hint := ""
					if vs.Type != nil {
						hint = render(t.fset, vs.Type)
					}
					for _, id := range vs.Names {
						add(id, "variable", hint, name)
					}
				}
			case *ast.RangeStmt:
				if node.Tok != token.DEFINE {
					return true
				}
				if id, ok := node.Value.(*ast.Ident); ok {
					add(id, "loop variable", "", name)
				}
			}
			return true
		})
	}

	out := make([]RenameTarget, 0, len(order))
	for _, name := range order {
		tgt := seen[name]
		tgt.Usage = usageLines(lines, name)
		if len(tgt.Usage) == 0 {
			continue
		}
		out = append(out, *tgt)
	}
	if len(out) > maxRenameTargets {
		out = out[:maxRenameTargets]
	}
	return out
}

// loopIndices returns the identifiers that are a loop's index, which are the
// one class of short name Go wants left alone.
func loopIndices(body *ast.BlockStmt) map[*ast.Ident]bool {
	out := map[*ast.Ident]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt:
			if assign, ok := node.Init.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
				for _, lhs := range assign.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						out[id] = true
					}
				}
			}
		case *ast.RangeStmt:
			if id, ok := node.Key.(*ast.Ident); ok {
				out[id] = true
			}
		}
		return true
	})
	return out
}

// scanShapes collects struct types whose names the converter had to invent.
func (t *targets) scanShapes(lines []string) []ShapeTarget {
	var out []ShapeTarget
	for _, decl := range t.file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			shape := ShapeTarget{Name: ts.Name.Name}
			for _, f := range st.Fields.List {
				for _, id := range f.Names {
					shape.Fields = append(shape.Fields, id.Name)
					shape.FieldTypes = append(shape.FieldTypes, render(t.fset, f.Type))
				}
			}
			if len(shape.Fields) == 0 {
				continue
			}
			// A type is worth asking about when its own name is invented, or
			// when any of its field names is.
			interesting := weakTypeName(shape.Name)
			for _, f := range shape.Fields {
				if syntheticSuffix.MatchString(f) {
					interesting = true
				}
			}
			if !interesting {
				continue
			}
			shape.Usage = usageLines(lines, shape.Name)
			out = append(out, shape)
		}
	}
	return out
}

// scanComments collects the top-level declarations with no doc comment,
// exported ones first, because those are the ones godoc shows.
func (t *targets) scanComments(lines []string) []CommentTarget {
	var out []CommentTarget
	for _, decl := range t.file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Doc != nil || d.Name.Name == "main" || d.Name.Name == "init" {
				continue
			}
			kind := "function"
			if d.Recv != nil {
				kind = "method"
			}
			out = append(out, CommentTarget{
				Name:     d.Name.Name,
				Kind:     kind,
				Decl:     declLine(t.fset, lines, d.Pos()),
				Exported: d.Name.IsExported(),
				Body:     bodyLines(t.fset, lines, d),
			})
		case *ast.GenDecl:
			if d.Doc != nil || d.Tok == token.IMPORT {
				continue
			}
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Doc != nil || s.Comment != nil {
						continue
					}
					out = append(out, CommentTarget{
						Name: s.Name.Name, Kind: "type",
						Decl:     declLine(t.fset, lines, d.Pos()),
						Exported: s.Name.IsExported(),
					})
				case *ast.ValueSpec:
					if s.Doc != nil || s.Comment != nil || len(s.Names) != 1 {
						continue
					}
					kind := "variable"
					if d.Tok == token.CONST {
						kind = "constant"
					}
					out = append(out, CommentTarget{
						Name: s.Names[0].Name, Kind: kind,
						Decl:     declLine(t.fset, lines, d.Pos()),
						Exported: s.Names[0].IsExported(),
					})
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Exported && !out[j].Exported })
	return out
}

// declLine returns the source line a declaration starts on.
func declLine(fset *token.FileSet, lines []string, pos token.Pos) string {
	n := fset.Position(pos).Line
	if n < 1 || n > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[n-1])
}

// bodyLines returns the first few lines of a function body, which is what a
// doc comment has to be true about.
func bodyLines(fset *token.FileSet, lines []string, fn *ast.FuncDecl) []string {
	if fn.Body == nil {
		return nil
	}
	from := fset.Position(fn.Body.Lbrace).Line
	to := min(fset.Position(fn.Body.Rbrace).Line, from+8)
	var out []string
	for i := from; i < to && i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// wordBoundary finds whole-word occurrences of an identifier, so that a search
// for `w` does not match `words`.
func wordBoundary(name string) *regexp.Regexp {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
}

// usageLines returns the lines an identifier appears on, deduplicated and
// capped. They are the evidence the model names the variable from.
func usageLines(lines []string, name string) []string {
	re := wordBoundary(name)
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if !re.MatchString(trimmed) {
			continue
		}
		if len(trimmed) > 160 {
			trimmed = trimmed[:157] + "..."
		}
		if !slices.Contains(out, trimmed) {
			out = append(out, trimmed)
		}
		if len(out) == maxUsageLines {
			break
		}
	}
	return out
}

// checkNewName is the whole of the naming contract, applied to every name the
// model returns before anything is rewritten.
//
// The rename is sound when the old name is declared exactly once in the file
// and the new name appears in it nowhere at all: with a single declaration
// there is no shadowing to get wrong, and with no prior occurrence there is no
// capture to introduce. Everything else here is style, which matters because
// the point of the job is that the result reads like Go.
func (t *targets) checkNewName(old, replacement string, exported bool) error {
	switch {
	case replacement == old:
		return &RejectedError{Gate: "naming", Reason: fmt.Sprintf("%q was offered as its own replacement", old)}
	case goKeywords[replacement]:
		return &RejectedError{Gate: "naming", Reason: fmt.Sprintf("%q is a Go keyword", replacement)}
	case predeclared[replacement]:
		return &RejectedError{Gate: "naming", Reason: fmt.Sprintf("%q shadows a predeclared identifier", replacement)}
	case t.imports[replacement]:
		return &RejectedError{Gate: "naming", Reason: fmt.Sprintf("%q is the name of an imported package", replacement)}
	case len(replacement) > 32:
		return &RejectedError{Gate: "naming", Reason: fmt.Sprintf("%q is too long to read as a name", replacement)}
	case exported && !validTypeName.MatchString(replacement):
		return &RejectedError{Gate: "naming", Reason: fmt.Sprintf("%q is not an exported Go name in the surrounding style", replacement)}
	case !exported && !validGoName.MatchString(replacement):
		return &RejectedError{Gate: "naming", Reason: fmt.Sprintf("%q is not a lower camel case Go name", replacement)}
	case t.taken[replacement]:
		return &RejectedError{Gate: "naming", Reason: fmt.Sprintf("%q is already used in this file", replacement)}
	case t.declCount[old] != 1:
		return &RejectedError{Gate: "naming", Reason: fmt.Sprintf("%q is declared %d times, so renaming it is not provably safe", old, t.declCount[old])}
	}
	return nil
}

// claim records a name as used, so two decisions in one batch cannot both take
// it.
func (t *targets) claim(name string) { t.taken[name] = true }
