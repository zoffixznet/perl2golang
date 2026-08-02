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
		return l.sortByName(n)
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

	kind, numeric, ok := comparatorShape(n.Block)
	// `<=>` orders numerically and `cmp` orders as text, whatever the values
	// happen to be held as. Sorting a slice of strings with Go's own ordering
	// answers the `cmp` question and gets `<=>` wrong: "142" comes before
	// "30" as text and after it as a number.
	native := (numeric && isNum(elem)) || (!numeric && isStr(elem))
	switch {
	case n.Block == nil:
		l.emit(l.defaultSort(target, elem, n))
	case ok && kind == cmpAscending:
		switch {
		case native:
			l.emit(exprStmt(call("slices", "slices", "Sort", ir.TVoid, target)))
		case numeric:
			l.emit(exprStmt(call("slices", "slices", "SortFunc", ir.TVoid, target,
				l.numericComparator(elem, false))))
		default:
			l.emit(exprStmt(call("slices", "slices", "SortFunc", ir.TVoid, target,
				l.textComparator(elem, false))))
		}
	case ok && kind == cmpDescending:
		var fn ir.Expr
		switch {
		case native:
			fn = l.reverseComparator(elem)
		case numeric:
			fn = l.numericComparator(elem, true)
		default:
			fn = l.textComparator(elem, true)
		}
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

// sortByName lowers `sort byname @list`, where the comparator is a named sub
// that reads its two values out of $a and $b.
//
// The sub itself has to change shape, because Go passes the two values as
// parameters. That is decided here, on the first pass, and the function is
// declared with the new signature when it is lowered.
func (l *Lowerer) sortByName(n *ast.Call) ir.Expr {
	src := l.list(argList(n))
	t := typeOrAny(src)
	elem := elemOf(t)

	s, ok := l.findSub(n.SortSub)
	if !ok {
		return l.todoExpr(n, "P2G5590", "sort with a named comparator",
			"the comparator sub is not declared in this file",
			"`sort "+n.SortSub+" @list` calls a sub this file does not declare, so "+
				"there is nothing to give the new signature to.",
			"Write the comparator as `func(a, b T) int` returning a negative number, "+
				"zero, or a positive one, and pass it to slices.SortFunc.",
			"sort-slice")
	}
	if l.pass == 1 {
		s.Comparator = true
		s.CmpElem = join(s.CmpElem, elem)
	}

	name := l.tmp("sorted")
	clone := assign(":=", []ir.Expr{ir.NewIdent(name, t)},
		[]ir.Expr{call("slices", "slices", "Clone", t, src)})
	l.setProv(clone, n)
	l.emit(clone)
	target := ir.NewIdent(name, t)

	st := exprStmt(call("slices", "slices", "SortFunc", ir.TVoid, target, ir.NewIdent(s.Go, nil)))
	l.note(st, "A named comparator becomes an ordinary function passed by name. In "+
		"Perl it read its two values out of the package globals $a and $b, which is "+
		"why it could not simply be called; in Go they are its parameters, so it is "+
		"a function value like any other.",
		"sort-slice", "closures-and-loop-capture")
	l.emit(st)
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
	l.approximate(n, "P2G5540", "sort with no comparator",
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

// numericComparator builds a comparator for values Go cannot order directly,
// reading each one as a number the way Perl's <=> does.
func (l *Lowerer) numericComparator(elem *ir.Type, reverse bool) ir.Expr {
	a, b := ir.Expr(ir.NewIdent("a", elem)), ir.Expr(ir.NewIdent("b", elem))
	if reverse {
		a, b = b, a
	}
	body := &ir.Block{Stmts: []ir.Stmt{
		&ir.Return{Results: []ir.Expr{
			call("cmp", "cmp", "Compare", ir.TInt, l.toFloat(a, nil), l.toFloat(b, nil)),
		}},
	}}
	fn := funcLit([]ir.Param{{Name: "a", Type: elem}, {Name: "b", Type: elem}}, []*ir.Type{ir.TInt}, body)
	l.note(fn, "These values are held in an any, which Go cannot order with <, so "+
		"the comparator reads them as numbers first. cmp.Compare is the standard "+
		"three-way comparison and is what a Go comparator returns.",
		"sort-slice")
	return fn
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

	stmts, value := l.blockValue(n.Block)
	if value == nil {
		l.refuse(n, "P2G5591", "sort block",
			"this ordering rule is not implemented",
			"The block does not end in an expression that produces an ordering.",
			"A Go comparator must return an int: negative, zero, or positive.",
			"sort-slice")
		return nil
	}
	if typeOrAny(value).Kind != ir.Int {
		value = l.toInt(value, nil)
	}
	body := &ir.Block{Stmts: append(stmts, &ir.Return{Results: []ir.Expr{value}})}
	return funcLit([]ir.Param{{Name: ba.Go, Type: elem}, {Name: bb.Go, Type: elem}}, []*ir.Type{ir.TInt}, body)
}

// blockValue lowers a Perl block that is evaluated for its value rather than
// for its effect: sort comparators, map bodies, grep tests.
//
// The distinction matters more than it looks. `$a <=> $b || $x cmp $y` read as
// a statement is a guard, `if the first test is false, run the second`, and the
// value disappears. Read as an expression it is the ordering the block exists
// to produce. Everything but the last statement is still lowered as statements,
// because that is what those are.
func (l *Lowerer) blockValue(block []ast.Stmt) ([]ir.Stmt, ir.Expr) {
	if len(block) == 0 {
		return nil, nil
	}
	lead := block
	var tail ast.Expr
	if last, isExpr := block[len(block)-1].(*ast.ExprStmt); isExpr {
		lead = block[:len(block)-1]
		tail = last.X
	}

	savedPre := l.pre
	l.pre = nil
	stmts := l.stmts(lead)
	var value ir.Expr
	if tail != nil {
		value = l.expr(tail)
	}
	stmts = append(stmts, l.takePre()...)
	l.pre = savedPre
	return stmts, value
}

type cmpKind int

const (
	cmpOther cmpKind = iota
	cmpAscending
	cmpDescending
)

// comparatorShape recognises the two comparators that make up almost all real
// sort blocks.
func comparatorShape(block []ast.Stmt) (kind cmpKind, numeric bool, ok bool) {
	if len(block) != 1 {
		return cmpOther, false, false
	}
	es, isExpr := block[0].(*ast.ExprStmt)
	if !isExpr {
		return cmpOther, false, false
	}
	bin, isBin := es.X.(*ast.BinOp)
	if !isBin || (bin.Op != "<=>" && bin.Op != "cmp") {
		return cmpOther, false, false
	}
	numeric = bin.Op == "<=>"
	lv, lok := bin.L.(*ast.Var)
	rv, rok := bin.R.(*ast.Var)
	if !lok || !rok || lv.Sigil != '$' || rv.Sigil != '$' {
		return cmpOther, numeric, false
	}
	switch {
	case lv.Name == "a" && rv.Name == "b":
		return cmpAscending, numeric, true
	case lv.Name == "b" && rv.Name == "a":
		return cmpDescending, numeric, true
	}
	return cmpOther, numeric, false
}

// textComparator builds the comparator for `cmp` over values that are not
// already strings, which orders them the way their text does.
func (l *Lowerer) textComparator(elem *ir.Type, reverse bool) ir.Expr {
	a := ir.Expr(ir.NewIdent("a", elem))
	b := ir.Expr(ir.NewIdent("b", elem))
	if reverse {
		a, b = b, a
	}
	body := &ir.Block{Stmts: []ir.Stmt{
		&ir.Return{Results: []ir.Expr{
			call("cmp", "cmp", "Compare", ir.TInt, l.toStr(a, nil), l.toStr(b, nil)),
		}},
	}}
	fn := funcLit([]ir.Param{{Name: "a", Type: elem}, {Name: "b", Type: elem}}, []*ir.Type{ir.TInt}, body)
	l.note(fn, "`cmp` orders values by their text whatever they are held as, so the "+
		"comparator renders them first. Ordering them as they stand would sort 10 "+
		"before 9 or the other way round depending on the type, which is exactly the "+
		"confusion the two operators exist to avoid.",
		"sort-slice", "explicit-conversions-no-coercion")
	return fn
}

// ---------------------------------------------------------------------------
// map and grep

// mapCall lowers map into the append loop that Go code writes instead.
func (l *Lowerer) mapCall(n *ast.Call) ir.Expr {
	src, item, body, value, ok := l.blockLoop(n)
	if !ok {
		return ir.Nil(ir.TAny)
	}
	if value == nil {
		return l.todoExpr(n, "P2G5595", "map block",
			"this map block is not implemented",
			"The block does not end in an expression, so there is nothing to collect.",
			"Rewrite it as an explicit loop that appends what you want.")
	}
	outT := ir.SliceOf(typeOrAny(value))
	name := l.tmp("mapped")
	decl := &ir.DeclStmt{Names: []string{name}, Type: outT}
	loop := &ir.Range{
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  item,
		X:      src,
		Define: true,
		Body: &ir.Block{Stmts: append(body,
			assign("=", []ir.Expr{ir.NewIdent(name, outT)},
				[]ir.Expr{appendTo(ir.NewIdent(name, outT), value)}))},
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
	src, item, body, value, ok := l.blockLoop(n)
	if !ok {
		return ir.Nil(ir.TAny)
	}
	if value == nil {
		return l.todoExpr(n, "P2G5596", "grep block",
			"this grep block is not implemented",
			"The block does not end in an expression, so there is no test to apply.",
			"Rewrite it as an explicit loop with an if.")
	}
	t := typeOrAny(src)
	name := l.tmp("matched")
	decl := &ir.DeclStmt{Names: []string{name}, Type: t}
	cond := value
	if typeOrAny(cond).Kind != ir.Bool {
		cond = l.toBool(cond, nil)
	}
	loop := &ir.Range{
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  item,
		X:      src,
		Define: true,
		Body: &ir.Block{Stmts: append(body, &ir.If{
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
func (l *Lowerer) blockLoop(n *ast.Call) (src ir.Expr, item ir.Expr, body []ir.Stmt, value ir.Expr, ok bool) {
	args := flatten(argList(n))
	block := n.Block
	if block == nil && len(args) > 0 {
		// The expression form, `map EXPR, LIST`.
		block = []ast.Stmt{&ast.ExprStmt{X: args[0]}}
		args = args[1:]
	}
	if block == nil || len(args) == 0 {
		return nil, nil, nil, nil, false
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

	// The block's value is its last expression, so that one is lowered as an
	// expression rather than as a statement: a statement layer would discard
	// exactly the thing the block exists to produce.
	lead := block
	var tail ast.Expr
	if last, isExpr := block[len(block)-1].(*ast.ExprStmt); isExpr {
		lead = block[:len(block)-1]
		tail = last.X
	}

	savedPre := l.pre
	l.pre = nil
	body = l.stmts(lead)
	if tail != nil {
		value = l.expr(tail)
	}
	inner := l.takePre()
	l.pre = savedPre
	body = append(body, inner...)
	return src, item, body, value, true
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

	pattern, ok := l.patternOf(args[0])
	if !ok {
		return l.markRefusedPattern(composite(ir.SliceOf(ir.TString), nil, nil))
	}
	// No third argument means Perl's default limit of zero, which drops
	// trailing empty fields. A negative limit is what keeps them.
	limit := ir.Expr(ir.IntLit("0"))
	if len(args) > 2 {
		limit = l.toInt(l.expr(args[2]), args[2])
	}
	out := l.helperCall(hSplitPattern, ir.SliceOf(ir.TString), pattern, subject, limit)
	if sep, plain := l.literalSeparator(args[0]); plain {
		l.note(out, "The separator here is plain text, so strings.Split(s, "+
			quote(sep)+") is the direct Go answer and is what you would normally "+
			"write. It differs in two ways that matter: split drops trailing empty "+
			"fields unless a negative limit says otherwise, and its third argument "+
			"caps the number of fields with the remainder left in the last one. The "+
			"helper keeps both rules.")
		return out
	}
	l.note(out, "Splitting on a pattern uses the compiled regular expression. The "+
		"helper keeps the rules regexp.Split does not have: trailing empty fields "+
		"are dropped, the limit leaves the remainder in the last field, and a "+
		"pattern with capture groups puts the captures into the result.",
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
