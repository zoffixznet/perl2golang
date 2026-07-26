package lower

import (
	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// sortCall lowers sort.
//
// Perl's sort returns a new list and, with no comparator, compares as text:
// (10, 9, 100) sorts to (10, 100, 9). Go has no default at all, which forces
// the question Perl answers silently and usually wrongly.
func (l *Lowerer) sortCall(n *ast.Call) ir.Expr {
	if n.SortSub != "" {
		return l.todoExpr(n, "P2G5590", "sort with a named comparator",
			"sort with a named sub is not implemented",
			"`sort byname @list` calls the named sub with the two values in the "+
				"package globals $a and $b. Go passes the two values as parameters, so "+
				"the sub's signature has to change.",
			"Turn the comparator into `func(a, b T) int` returning a negative number, "+
				"zero, or a positive one, and pass it to slices.SortFunc.",
			"sort-slice")
	}

	// `sort keys %h` is common enough, and idiomatic enough in Go, to deserve
	// its own shape.
	args := flatten(argList(n))
	if len(args) == 1 && n.Block == nil {
		if c, ok := args[0].(*ast.Call); ok && c.Name == "keys" {
			m := l.argExpr(c, 0)
			if typeOrAny(m).Kind == ir.Map {
				out := call("slices", "slices", "Sorted", ir.SliceOf(ir.TString),
					call("maps", "maps", "Keys", nil, m))
				l.note(out, "slices.Sorted takes an iterator and returns a sorted slice, "+
					"which is exactly `sort keys %h`. Sorting is needed here for the same "+
					"reason it is in Perl: map iteration order is randomised on purpose so "+
					"that nothing comes to depend on it.",
					"map-iteration-order", "sort-slice")
				return out
			}
		}
	}

	src := l.list(argList(n))
	t := typeOrAny(src)
	elem := elemOf(t)

	name := l.tmp("sorted")
	clone := assign(":=", []ir.Expr{ir.NewIdent(name, t)},
		[]ir.Expr{call("slices", "slices", "Clone", t, src)})
	l.setProv(clone, n)
	l.note(clone, "sort returns a new list and leaves the original untouched. Go's "+
		"sort functions work in place, so the slice is cloned first. Without the "+
		"clone the caller's data would be reordered, because a slice shares its "+
		"backing array with every other slice made from it.",
		"slice-aliasing-and-copy", "sort-slice")
	l.emit(clone)
	target := ir.NewIdent(name, t)

	kind, ok := comparatorShape(n.Block)
	switch {
	case n.Block == nil:
		l.emit(l.defaultSort(target, elem, n))
	case ok && kind == cmpAscending:
		l.emit(exprStmt(call("slices", "slices", "Sort", ir.TVoid, target)))
	case ok && kind == cmpDescending:
		fn := l.reverseComparator(elem)
		l.emit(exprStmt(call("slices", "slices", "SortFunc", ir.TVoid, target, fn)))
	default:
		fn := l.blockComparator(n, elem)
		if fn == nil {
			return target
		}
		st := exprStmt(call("slices", "slices", "SortFunc", ir.TVoid, target, fn))
		l.note(st, "A Go comparator takes the two values as parameters and returns a "+
			"negative number, zero, or a positive one. Perl passes them in the package "+
			"globals $a and $b instead, which is why a Perl comparator cannot be an "+
			"ordinary sub call.",
			"sort-slice")
		l.emit(st)
	}
	return target
}

// defaultSort emits the sort Perl performs when no comparator is given.
func (l *Lowerer) defaultSort(target ir.Expr, elem *ir.Type, n *ast.Call) ir.Stmt {
	if isStr(elem) {
		st := exprStmt(call("slices", "slices", "Sort", ir.TVoid, target))
		l.note(st, "sort with no comparator compares as text, and these values are "+
			"text, so slices.Sort matches it exactly.",
			"sort-slice")
		return st
	}

	// The values are numbers, so Perl's textual default and the obvious
	// numeric sort disagree. Reproducing the original means comparing the
	// printed forms.
	a, b := "a", "b"
	body := &ir.Block{Stmts: []ir.Stmt{
		&ir.Return{Results: []ir.Expr{
			call("strings", "strings", "Compare", ir.TInt,
				l.toStr(ir.NewIdent(a, elem), nil), l.toStr(ir.NewIdent(b, elem), nil)),
		}},
	}}
	fn := funcLit([]ir.Param{{Name: a, Type: elem}, {Name: b, Type: elem}}, []*ir.Type{ir.TInt}, body)
	st := exprStmt(call("slices", "slices", "SortFunc", ir.TVoid, target, fn))
	l.approximate(n, "P2G5501", "sort with no comparator",
		"the default sort compares as text, not as numbers",
		"Perl's sort with no block compares its values as strings, so (10, 9, 100) "+
			"comes back as (10, 100, 9). Go has no default comparison at all, which "+
			"means the question cannot be answered by accident.",
		"If a numeric sort was meant, replace the comparator with slices.Sort, "+
			"which compares numbers as numbers. The generated code reproduces the "+
			"original textual order.",
		"sort-slice")
	return st
}

// reverseComparator builds func(a, b T) int { return cmp.Compare(b, a) }.
func (l *Lowerer) reverseComparator(elem *ir.Type) ir.Expr {
	body := &ir.Block{Stmts: []ir.Stmt{
		&ir.Return{Results: []ir.Expr{
			call("cmp", "cmp", "Compare", ir.TInt, ir.NewIdent("b", elem), ir.NewIdent("a", elem)),
		}},
	}}
	fn := funcLit([]ir.Param{{Name: "a", Type: elem}, {Name: "b", Type: elem}}, []*ir.Type{ir.TInt}, body)
	l.note(fn, "Swapping the two arguments to cmp.Compare is how a descending sort is "+
		"written, and it is the whole of `$b <=> $a`.",
		"sort-slice")
	return fn
}

// blockComparator lowers an arbitrary sort block into a Go comparator.
func (l *Lowerer) blockComparator(n *ast.Call, elem *ir.Type) ir.Expr {
	saved := l.scope
	l.scope = newScope(saved)
	defer func() { l.scope = saved }()

	ba := l.declareNamed("sortA@"+itoa(posLine(n)), '$', "a", KindParam, n)
	bb := l.declareNamed("sortB@"+itoa(posLine(n)), '$', "b", KindParam, n)
	ba.Perl, bb.Perl = "$a", "$b"
	ba.Type, bb.Type = elem, elem
	l.scope.define(varKey('$', "a"), ba)
	l.scope.define(varKey('$', "b"), bb)

	stmts := l.stmts(n.Block)
	if len(stmts) == 0 {
		return nil
	}
	last, ok := stmts[len(stmts)-1].(*ir.ExprStmt)
	if !ok {
		l.refuse(n, "P2G5591", "sort block",
			"this ordering rule is not implemented",
			"The block does not end in an expression that produces an ordering.",
			"A Go comparator must return an int: negative, zero, or positive.",
			"sort-slice")
		return nil
	}
	value := last.X
	if typeOrAny(value).Kind != ir.Int {
		value = l.toInt(value, nil)
	}
	body := &ir.Block{Stmts: append(stmts[:len(stmts)-1], &ir.Return{Results: []ir.Expr{value}})}
	return funcLit([]ir.Param{{Name: ba.Go, Type: elem}, {Name: bb.Go, Type: elem}}, []*ir.Type{ir.TInt}, body)
}

type cmpKind int

const (
	cmpOther cmpKind = iota
	cmpAscending
	cmpDescending
)

// comparatorShape recognises the two comparators that make up almost all real
// sort blocks.
func comparatorShape(block []ast.Stmt) (cmpKind, bool) {
	if len(block) != 1 {
		return cmpOther, false
	}
	es, ok := block[0].(*ast.ExprStmt)
	if !ok {
		return cmpOther, false
	}
	bin, ok := es.X.(*ast.BinOp)
	if !ok || (bin.Op != "<=>" && bin.Op != "cmp") {
		return cmpOther, false
	}
	lv, lok := bin.L.(*ast.Var)
	rv, rok := bin.R.(*ast.Var)
	if !lok || !rok || lv.Sigil != '$' || rv.Sigil != '$' {
		return cmpOther, false
	}
	switch {
	case lv.Name == "a" && rv.Name == "b":
		return cmpAscending, true
	case lv.Name == "b" && rv.Name == "a":
		return cmpDescending, true
	}
	return cmpOther, false
}

// ---------------------------------------------------------------------------
// map and grep

// mapCall lowers map into the append loop that Go code writes instead.
func (l *Lowerer) mapCall(n *ast.Call) ir.Expr {
	src, item, body, ok := l.blockLoop(n)
	if !ok {
		return ir.Nil(ir.TAny)
	}
	if len(body) == 0 {
		return src
	}
	last, isExpr := body[len(body)-1].(*ir.ExprStmt)
	if !isExpr {
		return l.todoExpr(n, "P2G5595", "map block",
			"this map block is not implemented",
			"The block does not end in an expression, so there is nothing to collect.",
			"Rewrite it as an explicit loop that appends what you want.")
	}
	outT := ir.SliceOf(typeOrAny(last.X))
	name := l.tmp("mapped")
	decl := &ir.DeclStmt{Names: []string{name}, Type: outT}
	loop := &ir.Range{
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  item,
		X:      src,
		Define: true,
		Body: &ir.Block{Stmts: append(body[:len(body)-1],
			assign("=", []ir.Expr{ir.NewIdent(name, outT)},
				[]ir.Expr{appendTo(ir.NewIdent(name, outT), last.X)}))},
	}
	l.setProv(decl, n)
	l.note(decl, "Go has no map over a slice, and the standard library deliberately "+
		"never grew one even after generics made it possible. The append loop is the "+
		"accepted idiom: three more lines, and a stack trace that points at the line "+
		"that failed.",
		"range-is-not-foreach", "small-stdlib-philosophy")
	l.emit(decl)
	l.emit(loop)
	return ir.NewIdent(name, outT)
}

// grepCall lowers grep into a filtering loop.
func (l *Lowerer) grepCall(n *ast.Call) ir.Expr {
	src, item, body, ok := l.blockLoop(n)
	if !ok {
		return ir.Nil(ir.TAny)
	}
	if len(body) == 0 {
		return src
	}
	last, isExpr := body[len(body)-1].(*ir.ExprStmt)
	if !isExpr {
		return l.todoExpr(n, "P2G5596", "grep block",
			"this grep block is not implemented",
			"The block does not end in an expression, so there is no test to apply.",
			"Rewrite it as an explicit loop with an if.")
	}
	t := typeOrAny(src)
	name := l.tmp("matched")
	decl := &ir.DeclStmt{Names: []string{name}, Type: t}
	cond := last.X
	if typeOrAny(cond).Kind != ir.Bool {
		cond = l.toBool(cond, nil)
	}
	loop := &ir.Range{
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  item,
		X:      src,
		Define: true,
		Body: &ir.Block{Stmts: append(body[:len(body)-1], &ir.If{
			Cond: cond,
			Then: &ir.Block{Stmts: []ir.Stmt{
				assign("=", []ir.Expr{ir.NewIdent(name, t)}, []ir.Expr{appendTo(ir.NewIdent(name, t), item)}),
			}},
		})},
	}
	l.setProv(decl, n)
	l.note(decl, "grep becomes a loop with an if. The result starts as a nil slice, "+
		"which is perfectly usable: appending to nil allocates, ranging over nil "+
		"iterates zero times, and len(nil) is 0.",
		"nil-slices-vs-nil-maps", "range-is-not-foreach")
	l.emit(decl)
	l.emit(loop)
	return ir.NewIdent(name, t)
}

// blockLoop is the shared setup for map and grep: it lowers the list, invents
// the loop variable, and lowers the block with $_ bound to it.
func (l *Lowerer) blockLoop(n *ast.Call) (src ir.Expr, item ir.Expr, body []ir.Stmt, ok bool) {
	args := flatten(argList(n))
	block := n.Block
	if block == nil && len(args) > 0 {
		// The expression form, `map EXPR, LIST`.
		block = []ast.Stmt{&ast.ExprStmt{X: args[0]}}
		args = args[1:]
	}
	if block == nil || len(args) == 0 {
		return nil, nil, nil, false
	}
	var listArgs []ast.Expr
	listArgs = append(listArgs, args...)
	parts, t := l.listParts(listArgs)
	if len(parts) == 1 && typeOrAny(parts[0]).Kind == ir.Slice {
		src = parts[0]
	} else {
		src = composite(ir.SliceOf(t), nil, parts)
	}
	elem := elemOf(typeOrAny(src))

	saved := l.scope
	l.scope = newScope(saved)
	defer func() { l.scope = saved }()

	name := l.tmp("item")
	item = ir.NewIdent(name, elem)
	l.topicStack = append(l.topicStack, item)
	defer func() { l.topicStack = l.topicStack[:len(l.topicStack)-1] }()

	savedPre := l.pre
	l.pre = nil
	body = l.stmts(block)
	inner := l.takePre()
	l.pre = savedPre
	if len(inner) > 0 {
		body = append(inner, body...)
	}
	return src, item, body, true
}

// mapToHash recognises `map { KEY => VALUE } LIST` used to build a hash, which
// is a very common Perl idiom and becomes a plain loop in Go.
func (l *Lowerer) mapToHash(n *ast.Call, want *ir.Type) (ir.Expr, bool) {
	if n.Name != "map" || n.Block == nil || len(n.Block) != 1 {
		return nil, false
	}
	es, ok := n.Block[0].(*ast.ExprStmt)
	if !ok {
		return nil, false
	}
	pair := flatten(es.X)
	if len(pair) != 2 {
		return nil, false
	}

	args := flatten(argList(n))
	parts, t := l.listParts(args)
	src := ir.Expr(composite(ir.SliceOf(t), nil, parts))
	if len(parts) == 1 && typeOrAny(parts[0]).Kind == ir.Slice {
		src = parts[0]
	}
	elem := elemOf(typeOrAny(src))

	saved := l.scope
	l.scope = newScope(saved)
	defer func() { l.scope = saved }()

	itemName := l.tmp("item")
	item := ir.NewIdent(itemName, elem)
	l.topicStack = append(l.topicStack, item)
	defer func() { l.topicStack = l.topicStack[:len(l.topicStack)-1] }()

	key := l.toStr(l.expr(pair[0]), pair[0])
	value := l.expr(pair[1])
	mapT := ir.MapOf(typeOrAny(value))
	if want != nil && want.Kind != ir.Any {
		mapT = ir.MapOf(join(typeOrAny(value), want))
	}

	name := l.tmp("byKey")
	decl := assign(":=", []ir.Expr{ir.NewIdent(name, mapT)}, []ir.Expr{composite(mapT, nil, nil)})
	loop := &ir.Range{
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  item,
		X:      src,
		Define: true,
		Body: &ir.Block{Stmts: []ir.Stmt{
			assign("=", []ir.Expr{index(ir.NewIdent(name, mapT), key, elemOf(mapT))},
				[]ir.Expr{l.assignable(value, elemOf(mapT), pair[1])}),
		}},
	}
	l.setProv(decl, n)
	l.note(decl, "`my %seen = map { $_ => 1 } @list` builds a lookup table through a "+
		"flat list of alternating keys and values. Go builds the map directly, which "+
		"is both clearer and cheaper, and it makes the map literal's type visible.",
		"nil-slices-vs-nil-maps")
	l.emit(decl)
	l.emit(loop)
	return ir.NewIdent(name, mapT), true
}

// ---------------------------------------------------------------------------
// split

// splitCall lowers split.
func (l *Lowerer) splitCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 0 {
		return composite(ir.SliceOf(ir.TString), nil, nil)
	}
	var subject ir.Expr
	if len(args) > 1 {
		subject = l.toStr(l.expr(args[1]), args[1])
	} else if len(l.topicStack) > 0 {
		subject = l.toStr(l.topicStack[len(l.topicStack)-1], nil)
	} else {
		subject = ir.Str(`""`)
	}

	// `split ' '` is the awk special case: leading whitespace is skipped and
	// runs of whitespace separate the fields.
	if text, ok := staticString(args[0]); ok && text == " " {
		out := call("strings", "strings", "Fields", ir.SliceOf(ir.TString), subject)
		l.note(out, "A single space as the split pattern is a special case: it means "+
			"split on runs of whitespace and drop any leading empty field. That is "+
			"exactly strings.Fields.")
		return out
	}

	// A literal separator with no pattern metacharacters is a plain string
	// split, which is what a Go developer writes and is much faster.
	if sep, ok := l.literalSeparator(args[0]); ok {
		out := call("strings", "strings", "Split", ir.SliceOf(ir.TString), subject, ir.Str(quote(sep)))
		l.inform(n, "P2G4530", "split on a fixed separator",
			"split removes trailing empty fields unless a negative limit is given, "+
				"and strings.Split keeps them. For \"a,b,,\" the original yields two "+
				"fields and strings.Split yields four. Add a trim if the input can end "+
				"with separators.")
		l.note(out, "strings.Split takes the separator as plain text, not as a "+
			"pattern, so nothing has to be escaped and no regular expression is "+
			"compiled.")
		return out
	}

	pattern, ok := l.patternOf(args[0])
	if !ok {
		return composite(ir.SliceOf(ir.TString), nil, nil)
	}
	limit := ir.Expr(ir.IntLit("-1"))
	if len(args) > 2 {
		limit = l.toInt(l.expr(args[2]), args[2])
	}
	out := l.helperCall(hSplitPattern, ir.SliceOf(ir.TString), pattern, subject, limit)
	l.note(out, "Splitting on a pattern uses the compiled regular expression. The "+
		"helper keeps the rules Go's regexp.Split does not have: trailing empty "+
		"fields are dropped, and a pattern with capture groups puts the captures into "+
		"the result.",
		"regexp-is-re2")
	return out
}

// literalSeparator reports whether a split pattern is really a fixed string.
func (l *Lowerer) literalSeparator(e ast.Expr) (string, bool) {
	var raw string
	switch n := e.(type) {
	case *ast.StrLit:
		raw = n.Value
	case *ast.Match:
		if n.Pattern == nil || n.Pattern.Mods != "" {
			return "", false
		}
		raw = n.Pattern.Raw
	case *ast.Regex:
		if n.Mods != "" {
			return "", false
		}
		raw = n.Raw
	default:
		return "", false
	}
	if raw == "" {
		return "", false
	}
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\\', '.', '+', '*', '?', '(', ')', '[', ']', '{', '}', '^', '$', '|':
			return "", false
		}
	}
	return raw, true
}
