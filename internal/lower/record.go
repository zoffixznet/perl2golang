package lower

import (
	"sort"
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file turns the hash reference Perl carries records in into a Go
// struct.
//
// `{ name => $name, secs => $secs, tags => [] }` is not a collection at all:
// its keys are written into the program, they never vary, and the values are
// different kinds of thing. Go's word for that is a struct, and the whole
// difference shows up at the first field read: `job.Secs` is an int, where
// `m["secs"]` is an `any` that has to be asserted before it can be added to
// anything.
//
// The test used here is deliberately narrow. Every key has to be written out,
// there have to be at least two of them, and no single Go type may cover all
// the values. A hash whose values are all the same type is a lookup table and
// stays a map, which is the right answer for it: a map is what you want when
// the keys are data.

// recordFor returns the struct type a hash literal of this shape becomes, or
// nil when the literal is better left a map.
//
// Two literals with the same set of keys share one type, which is what makes
// a constructor function and the records it builds agree.
func (l *Lowerer) recordFor(h *ast.AnonHash, hint string) *Class {
	keys, ok := recordKeys(h)
	if !ok {
		return nil
	}
	shape := strings.Join(keys, "\x00")
	if c, ok := l.records[shape]; ok {
		return c
	}
	if !mixedValues(h) {
		return nil
	}
	c := l.declareRecord(recordName(hint, keys), keys)
	if l.records == nil {
		l.records = map[string]*Class{}
	}
	l.records[shape] = c
	return c
}

// recordHint is the best name in scope for a record being built here: the
// variable it is about to be stored in, or the sub that returns it.
func (l *Lowerer) recordHint() string {
	if len(l.hints) > 0 {
		return l.hints[len(l.hints)-1]
	}
	if l.curSub != nil {
		return l.curSub.Name
	}
	return ""
}

// withHint runs f with a name suggestion in scope, for the literals built
// inside it.
func (l *Lowerer) withHint(name string, f func()) {
	l.hints = append(l.hints, name)
	f()
	l.hints = l.hints[:len(l.hints)-1]
}

// recordHash decides whether a `my %h = (...)` initialiser is a record. The
// list of pairs is the same shape a hash literal has, so it is looked at as
// one.
func (l *Lowerer) recordHash(rhs ast.Expr, hint string) (*Class, *ast.AnonHash, bool) {
	flat, ok := hashPairs(rhs)
	if !ok {
		return nil, nil, false
	}
	h := &ast.AnonHash{Elems: flat}
	h.SetSpan(rhs.Pos(), rhs.End())
	c := l.recordFor(h, hint)
	if c == nil {
		return nil, nil, false
	}
	return c, h, true
}

// recordKeys reads the key list of a hash literal, reporting false unless
// every key is written out in the source and there are at least two.
func recordKeys(h *ast.AnonHash) ([]string, bool) {
	var flat []ast.Expr
	for _, e := range h.Elems {
		flat = append(flat, flatten(e)...)
	}
	if len(flat) < 4 || len(flat)%2 != 0 {
		return nil, false
	}
	keys := make([]string, 0, len(flat)/2)
	seen := map[string]bool{}
	for i := 0; i+1 < len(flat); i += 2 {
		k, ok := staticString(flat[i])
		if !ok || k == "" || seen[k] {
			return nil, false
		}
		seen[k] = true
		keys = append(keys, k)
	}
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	return sorted, true
}

// mixedValues reports whether the values in a hash literal are things no one
// Go type covers, which is what separates a record from a lookup table.
//
// The test is on how the values are written and not on what they turn out to
// be, because it has to give the same answer every time it is asked and the
// types are still moving while inference runs. A table written out entirely
// as text, entirely as numbers or entirely as subs is a table: its keys are
// data and a map is what holds it. Anything else is a record.
func mixedValues(h *ast.AnonHash) bool {
	var flat []ast.Expr
	for _, e := range h.Elems {
		flat = append(flat, flatten(e)...)
	}
	kind := ""
	for i := 1; i < len(flat); i += 2 {
		k := literalKind(flat[i])
		if k == "" {
			return true
		}
		if kind == "" {
			kind = k
		} else if kind != k {
			return true
		}
	}
	return false
}

// literalKind names the sort of literal an expression is written as, and is
// empty for anything that is not one.
func literalKind(e ast.Expr) string {
	switch n := e.(type) {
	case *ast.StrLit:
		return "text"
	case *ast.NumberLit:
		return "number"
	case *ast.AnonSub:
		return "sub"
	case *ast.AnonArray:
		return "list"
	case *ast.AnonHash:
		return "hash"
	case *ast.InterpLit:
		if _, ok := staticString(n); ok {
			return "text"
		}
	}
	return ""
}

// declareRecord registers a struct type with these field names, so that the
// rest of the lowering can treat it exactly as it treats a blessed class.
func (l *Lowerer) declareRecord(name string, keys []string) *Class {
	c := &Class{
		Perl:    name,
		Go:      l.names.take(name),
		IsType:  true,
		Record:  true,
		fieldBy: map[string]*ClassField{},
		subBy:   map[string]*Sub{},
	}
	c.Value = ir.NamedType(c.Go, "")
	c.Ptr = ir.PointerTo(c.Value)
	c.recv = strings.ToLower(c.Go[:1])
	l.byGoType[c.Go] = c
	// The key is the generated name, which is unique, and not the readable
	// one: two records can want the same readable name and only one of them
	// may own the entry the emitter walks.
	key := "record\x00" + c.Go
	l.classes[key] = c
	l.classOrd = append(l.classOrd, key)
	for _, k := range keys {
		l.declareField(c, k, nil)
	}
	return c
}

// recordName picks the type's name from where the literal was written: the
// sub that builds it, the variable it is stored in, or the keys themselves.
func recordName(hint string, keys []string) string {
	hint = strings.TrimPrefix(hint, "make_")
	hint = strings.TrimPrefix(hint, "new_")
	hint = strings.TrimPrefix(hint, "build_")
	if name := exportedName(singular(hint)); name != "" {
		return name
	}
	return exportedName(singular(keys[0])) + "Record"
}

// singular trims one trailing s from a plural name, which is what a variable
// holding a list of records is usually called.
func singular(name string) string {
	if len(name) > 3 && strings.HasSuffix(name, "s") && !strings.HasSuffix(name, "ss") {
		return name[:len(name)-1]
	}
	return name
}

// recordLit builds the struct literal for a record, and explains the choice.
func (l *Lowerer) recordLit(c *Class, h *ast.AnonHash) ir.Expr {
	if l.recordUsed == nil {
		l.recordUsed = map[*Class]bool{}
	}
	l.recordUsed[c] = true
	out := l.structLit(c, h)
	l.note(out, "The keys of this hash are written into the program and never vary, "+
		"and no one Go type covers all its values, so it is a record rather than a "+
		"collection. A struct gives every field its own type, costs no lookup, and "+
		"turns a mistyped field name into a compile error.",
		"structs-and-embedding", "collections-hold-one-type")
	if l.pass == 2 {
		l.inform(h, "P2G3070", "a hash used as a record",
			"Perl carries records in hash references, so the field names live in the "+
				"data and every read of one costs a lookup and a conversion. Go writes "+
				"them down once as a struct, which is why `job.Secs` is an int here where "+
				"`$job->{secs}` was whatever happened to be in the hash.",
			"structs-and-embedding", "collections-hold-one-type")
	}
	return out
}

// fieldSlice lowers `@$rec{qw(a b c)}`, which reads several fields of one
// record at once.
//
// Go has no syntax for that and no need of one: the fields are named, so the
// result is a list literal of the values. Their types differ, which is why it
// comes out as a list of `any` and why the Perl was able to write it as one
// slice in the first place.
func (l *Lowerer) fieldSlice(n *ast.Slice, container ir.Expr) (ir.Expr, bool) {
	c := l.classOf(typeOrAny(container))
	if c == nil {
		return nil, false
	}
	var indexes []ast.Expr
	for _, ie := range n.Idx {
		indexes = append(indexes, flattenWords(ie)...)
	}
	var elems []ir.Expr
	for _, one := range indexes {
		key, ok := staticString(one)
		if !ok {
			return nil, false
		}
		f := c.field(key)
		if f == nil && l.pass == 1 {
			f = l.declareField(c, key, one)
		}
		if f == nil {
			return nil, false
		}
		elems = append(elems, l.assignable(selector(container, f.Go, f.Type), ir.TAny, one))
	}
	if len(elems) == 0 {
		return nil, false
	}
	out := composite(ir.SliceOf(ir.TAny), nil, elems)
	l.note(out, "A slice over a hash reads several keys at once. These are struct "+
		"fields, so each name is a field selector, and the list holds `any` because "+
		"the fields do not all have the same type.",
		"structs-and-embedding", "collections-hold-one-type")
	return out, true
}

// recordFields answers `keys %$rec` and `values %$rec` for a record.
//
// A struct's field names are fixed when the program is compiled, so the list
// is written out here rather than looked up. What that costs is that adding a
// field means adding it in two places, which is the trade the language makes
// everywhere else too.
func (l *Lowerer) recordFields(n ast.Node, c *Class, recv ir.Expr, wantValues bool) ir.Expr {
	names := make([]string, 0, len(c.Fields))
	for _, f := range c.Fields {
		names = append(names, f.Perl)
	}
	sort.Strings(names)

	elems := make([]ir.Expr, 0, len(names))
	for _, name := range names {
		if !wantValues {
			elems = append(elems, ir.Str(quote(name)))
			continue
		}
		f := c.field(name)
		elems = append(elems, l.assignable(selector(recv, f.Go, f.Type), ir.TAny, n))
	}
	elem := ir.TString
	if wantValues {
		elem = ir.TAny
	}
	out := composite(ir.SliceOf(elem), nil, elems)
	word := "keys"
	if wantValues {
		word = "values"
	}
	l.approximate(n, "P2G3071", word+" of a record",
		"the field list is written out rather than looked up",
		"This value is a struct, and a struct has no keys to ask for: its field "+
			"names are fixed when the program is compiled, so the list is written out "+
			"here.",
		"A field added to the record has to be added to this list too. Where the set "+
			"of names really varies, that part of the data wants to be a map.",
		"structs-and-embedding", "compile-time-mindset")
	l.note(out, "A struct is not a collection, so there is nothing to enumerate at "+
		"run time. Listing the names is what a Go program does, and the compiler "+
		"checks that each one exists.",
		"structs-and-embedding")
	return out
}

// recordFieldByName declares, once per record type, the method that answers a
// field name the program works out while it runs.
//
// Go has no way to reach a field by a computed name short of reflection, and
// a switch is what a Go program writes instead: it is faster, it is checked
// when the program is compiled, and it says out loud which names are allowed.
func (l *Lowerer) recordFieldByName(c *Class) string {
	if l.pass != 2 {
		return "fieldOf" + c.Go
	}
	if l.recordLookups == nil {
		l.recordLookups = map[*Class]string{}
	}
	if name, ok := l.recordLookups[c]; ok {
		return name
	}
	name := l.names.take("fieldOf" + c.Go)
	l.recordLookups[c] = name

	var b strings.Builder
	b.WriteString("func " + name + "(r *" + c.Go + ", field string) any {\n\tswitch field {\n")
	for _, f := range c.Fields {
		b.WriteString("\tcase " + quote(f.Perl) + ":\n\t\treturn r." + f.Go + "\n")
	}
	b.WriteString("\t}\n\treturn nil\n}")
	d := &ir.RawDecl{
		Source: b.String(),
		Doc: []string{name + " returns the named field of " + article(c.Go) + " " + c.Go +
			", for the places a field name is only known while the program runs."},
	}
	ir.Annotate(d, "A struct field cannot be reached by a name computed at run time, "+
		"and the reflect package is the only thing that can. A switch is what Go "+
		"programs write instead: it is checked when the program is compiled and it "+
		"says out loud which names are allowed.",
		"structs-and-embedding", "compile-time-mindset")
	l.recordDecls = append(l.recordDecls, d)
	return name
}
