package lower

import (
	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// This file lowers the List::Util functions that take a block.
//
// Go's standard library has almost none of them, and that is a decision rather
// than an omission: the loop is the accepted idiom, it keeps the accumulator's
// type visible, and a stack trace points at the line that failed rather than
// into a combinator. So each of these becomes the loop a Go developer would
// have written, emitted before the expression that uses its result.

// firstCall lowers `first BLOCK LIST`, which stops at the first match.
func (l *Lowerer) firstCall(n *ast.Call) ir.Expr {
	src, item, body, value, ok := l.blockLoop(n)
	if !ok || value == nil {
		return l.todoExpr(n, "P2G7541", "List::Util first",
			"this first block is not implemented",
			"The block does not end in an expression, so there is no test to apply.",
			"Write the loop directly, with an if and a break.")
	}
	elem := elemOf(typeOrAny(src))
	name := l.tmp("found")
	decl := &ir.DeclStmt{Names: []string{name}, Type: elem}
	loop := &ir.Range{
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  item,
		X:      src,
		Define: true,
		Body: &ir.Block{Stmts: append(body, &ir.If{
			Cond: l.truth(value),
			Then: &ir.Block{Stmts: []ir.Stmt{
				assign("=", []ir.Expr{ir.NewIdent(name, elem)}, []ir.Expr{item}),
				&ir.Branch{Kind: "break"},
			}},
		})},
	}
	l.setProv(decl, n)
	l.note(decl, "first is a loop with a break. Writing it out costs three lines and "+
		"makes the early exit visible, which is the whole point of the function.",
		"range-is-not-foreach")
	l.approximate(n, "P2G7541", "List::Util first",
		"no match now yields the zero value, not undef",
		"first returns undef when nothing matches, which is a value distinct from "+
			"every element. A Go variable of the element type has no such value, so it "+
			"keeps its zero value.",
		"Where the difference matters, return the index instead and use -1 for no "+
			"match, or return a second bool as the standard library does.",
		"nil-vs-undef", "comma-ok-idiom")
	l.emit(decl)
	l.emit(loop)
	return ir.NewIdent(name, elem)
}

// quantifier lowers any, all and none, which are the same loop with different
// starting values.
func (l *Lowerer) quantifier(n *ast.Call) ir.Expr {
	src, item, body, value, ok := l.blockLoop(n)
	if !ok || value == nil {
		return l.todoExpr(n, "P2G7542", "List::Util "+n.Name,
			"this "+n.Name+" block is not implemented",
			"The block does not end in an expression, so there is no test to apply.",
			"Write the loop directly, with an if and a break.")
	}

	cond := l.truth(value)
	start := ir.BoolLit(false)
	settled := ir.Expr(ir.BoolLit(true))
	if n.Name != "any" {
		// all and none both start out true and are disproved by one element.
		start = ir.BoolLit(true)
		settled = ir.BoolLit(false)
		if n.Name == "all" {
			cond = ir.Un("!", cond, ir.TBool)
		}
	}

	name := l.tmp(n.Name + "Match")
	decl := assign(":=", []ir.Expr{ir.NewIdent(name, ir.TBool)}, []ir.Expr{start})
	loop := &ir.Range{
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  item,
		X:      src,
		Define: true,
		Body: &ir.Block{Stmts: append(body, &ir.If{
			Cond: cond,
			Then: &ir.Block{Stmts: []ir.Stmt{
				assign("=", []ir.Expr{ir.NewIdent(name, ir.TBool)}, []ir.Expr{settled}),
				&ir.Branch{Kind: "break"},
			}},
		})},
	}
	l.setProv(decl, n)
	l.note(decl, "The answer starts at what an empty list would give and the loop "+
		"disproves it, breaking as soon as one element settles the question. That is "+
		"also why all of an empty list is true: nothing disproved it.",
		"range-is-not-foreach")
	l.emit(decl)
	l.emit(loop)
	return ir.NewIdent(name, ir.TBool)
}

// reduceCall lowers `reduce BLOCK LIST`, where the block sees the accumulator
// in $a and the next element in $b.
func (l *Lowerer) reduceCall(n *ast.Call) ir.Expr {
	if n.Block == nil {
		return l.todoExpr(n, "P2G7543", "List::Util reduce",
			"reduce without a block is not implemented",
			"reduce folds a list with a block that sees the accumulator in $a and the "+
				"next element in $b.",
			"Write the loop directly, with the accumulator declared before it.")
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

	acc := l.tmp("acc")
	itemName := l.tmp("item")

	ba := l.declareNamed("reduceA@"+itoa(posLine(n)), '$', "a", KindLocal, n)
	bb := l.declareNamed("reduceB@"+itoa(posLine(n)), '$', "b", KindLocal, n)
	ba.Perl, bb.Perl = "$a", "$b"
	ba.Type, bb.Type = elem, elem
	l.observe(ba, elem)
	l.observe(bb, elem)
	l.aliases = setAlias(l.aliases, bb, ir.NewIdent(itemName, elem))
	l.scope.define(varKey('$', "a"), ba)
	l.scope.define(varKey('$', "b"), bb)

	readsBefore := ba.Reads
	stmts, value := l.blockValue(n.Block)
	if value == nil {
		return l.todoExpr(n, "P2G7543", "List::Util reduce",
			"this reduce block is not implemented",
			"The block does not end in an expression, so there is nothing to fold "+
				"into the accumulator.",
			"Write the loop directly, assigning to the accumulator at the end of "+
				"each turn.")
	}

	// $a is the accumulator's value for this turn. Naming it only when the
	// block reads it keeps Go's unused-variable rule satisfied.
	var bind []ir.Stmt
	if ba.Reads > readsBefore {
		bind = []ir.Stmt{assign(":=", []ir.Expr{ir.NewIdent(ba.Go, elem)},
			[]ir.Expr{ir.NewIdent(acc, elem)})}
	}

	fold := append(bind, stmts...)
	fold = append(fold, assign("=", []ir.Expr{ir.NewIdent(acc, elem)},
		[]ir.Expr{l.assignable(value, elem, nil)}))

	idx := l.tmp("i")
	decl := &ir.DeclStmt{Names: []string{acc}, Type: elem}
	loop := &ir.Range{
		Key:    ir.NewIdent(idx, ir.TInt),
		Value:  ir.NewIdent(itemName, elem),
		X:      src,
		Define: true,
		Body: &ir.Block{Stmts: []ir.Stmt{
			&ir.If{
				Cond: ir.Bin("==", ir.NewIdent(idx, ir.TInt), ir.IntLit("0"), ir.TBool),
				Then: &ir.Block{Stmts: []ir.Stmt{
					assign("=", []ir.Expr{ir.NewIdent(acc, elem)}, []ir.Expr{ir.NewIdent(itemName, elem)}),
					&ir.Branch{Kind: "continue"},
				}},
			},
			&ir.Block{Stmts: fold},
		}},
	}
	l.setProv(decl, n)
	l.note(decl, "reduce folds the list into one value, seeding the accumulator with "+
		"the first element and then combining it with each of the rest. Perl passes "+
		"the two halves in the package globals $a and $b; here they are the "+
		"accumulator and the loop variable, which is why the fold reads as ordinary "+
		"Go.",
		"range-is-not-foreach")
	l.approximate(n, "P2G7543", "List::Util reduce",
		"an empty list now folds to the zero value",
		"reduce returns undef for an empty list, which is distinguishable from every "+
			"result it could produce. The accumulator here is an ordinary variable of "+
			"the element type, so an empty list leaves it at its zero value.",
		"Check the length before folding where the difference matters.",
		"nil-vs-undef")
	l.emit(decl)
	l.emit(loop)
	return ir.NewIdent(acc, elem)
}

// uniqCall lowers uniq and uniqnum, which keep the first of each repeated
// value.
func (l *Lowerer) uniqCall(n *ast.Call, numeric bool) ir.Expr {
	src := l.list(argList(n))
	t := typeOrAny(src)
	elem := elemOf(t)

	itemName := l.tmp("item")
	item := ir.NewIdent(itemName, elem)
	keyType := elem
	key := ir.Expr(item)
	if numeric {
		keyType = ir.TFloat
		key = l.toNum(item, nil)
	}

	seen := l.tmp("seen")
	out := l.tmp("unique")
	seenType := ir.MapOf(ir.TBool)
	seenType.Key = keyType

	seenDecl := assign(":=", []ir.Expr{ir.NewIdent(seen, seenType)}, []ir.Expr{composite(seenType, nil, nil)})
	outDecl := &ir.DeclStmt{Names: []string{out}, Type: ir.SliceOf(elem)}
	lookup := index(ir.NewIdent(seen, seenType), key, ir.TBool)
	loop := &ir.Range{
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  item,
		X:      src,
		Define: true,
		Body: &ir.Block{Stmts: []ir.Stmt{&ir.If{
			Cond: ir.Un("!", lookup, ir.TBool),
			Then: &ir.Block{Stmts: []ir.Stmt{
				assign("=", []ir.Expr{index(ir.NewIdent(seen, seenType), key, ir.TBool)}, []ir.Expr{ir.BoolLit(true)}),
				assign("=", []ir.Expr{ir.NewIdent(out, ir.SliceOf(elem))},
					[]ir.Expr{appendTo(ir.NewIdent(out, ir.SliceOf(elem)), item)}),
			}},
		}}},
	}
	l.setProv(seenDecl, n)
	l.note(seenDecl, "A set in Go is a map to bool, and reading a key that is not "+
		"there gives false rather than failing, which is exactly the test wanted "+
		"here. The result keeps the original order because it is built by appending.",
		"nil-slices-vs-nil-maps")
	if numeric {
		l.note(seenDecl, "uniqnum compares the values as numbers, so \"1.0\" and 1 "+
			"are the same value. The key is the number rather than the text, which is "+
			"what makes that happen.")
	}
	l.emit(seenDecl)
	l.emit(outDecl)
	l.emit(loop)
	return ir.NewIdent(out, ir.SliceOf(elem))
}

// pairsCall lowers pairs, which walks a flat list two elements at a time.
func (l *Lowerer) pairsCall(n *ast.Call) ir.Expr {
	src := l.list(argList(n))
	elem := elemOf(typeOrAny(src))
	pairType := ir.SliceOf(elem)
	outType := ir.SliceOf(pairType)

	name := l.tmp("pairs")
	list := l.tmp("flat")
	idx := l.tmp("i")

	listDecl := assign(":=", []ir.Expr{ir.NewIdent(list, typeOrAny(src))}, []ir.Expr{src})
	decl := &ir.DeclStmt{Names: []string{name}, Type: outType}
	first := index(ir.NewIdent(list, typeOrAny(src)), ir.NewIdent(idx, ir.TInt), elem)
	second := index(ir.NewIdent(list, typeOrAny(src)),
		ir.Bin("+", ir.NewIdent(idx, ir.TInt), ir.IntLit("1"), ir.TInt), elem)
	body := []ir.Stmt{
		assign("=", []ir.Expr{ir.NewIdent(name, outType)},
			[]ir.Expr{appendTo(ir.NewIdent(name, outType), composite(pairType, nil, []ir.Expr{first, second}))}),
	}
	loop := &ir.For{
		Init: assign(":=", []ir.Expr{ir.NewIdent(idx, ir.TInt)}, []ir.Expr{ir.IntLit("0")}),
		Cond: ir.Bin("<", ir.Bin("+", ir.NewIdent(idx, ir.TInt), ir.IntLit("1"), ir.TInt),
			lenOf(ir.NewIdent(list, typeOrAny(src))), ir.TBool),
		Post: assign("+=", []ir.Expr{ir.NewIdent(idx, ir.TInt)}, []ir.Expr{ir.IntLit("2")}),
		Body: &ir.Block{Stmts: body},
	}
	l.setProv(decl, n)
	l.note(decl, "pairs walks a flat list two at a time, which is the three-clause "+
		"for loop rather than a range: range hands over one element at a time and "+
		"cannot be told to take two.",
		"range-is-not-foreach")
	l.approximate(n, "P2G7544", "List::Util pairs",
		"an odd-length list loses its last element",
		"pairs on a list with an odd number of elements pairs the last one with "+
			"undef. There is no undef here, so the loop stops before it and the last "+
			"element is dropped.",
		"Check the length first where an odd list is possible.",
		"nil-vs-undef")
	l.emit(listDecl)
	l.emit(decl)
	l.emit(loop)
	return ir.NewIdent(name, outType)
}

// strExtreme lowers maxstr and minstr, which compare as text rather than as
// numbers.
func (l *Lowerer) strExtreme(n *ast.Call) ir.Expr {
	src := l.list(argList(n))
	elem := elemOf(typeOrAny(src))
	fn := "Max"
	if n.Name == "minstr" {
		fn = "Min"
	}
	if !isStr(elem) {
		return l.strExtremeLoop(n, src, elem)
	}
	out := call("slices", "slices", fn, elem, src)
	l.note(out, "slices.Max on a slice of strings compares them as text, which is "+
		"what "+n.Name+" does. On numbers it would compare them as numbers, which is "+
		"why Perl needs two functions and Go needs one.")
	return out
}

// strExtremeLoop is maxstr and minstr over values that are not already text:
// each one is rendered before it is compared, and the element itself is what
// comes back.
func (l *Lowerer) strExtremeLoop(n *ast.Call, src ir.Expr, elem *ir.Type) ir.Expr {
	best := l.tmp("best")
	idx := l.tmp("i")
	itemName := l.tmp("item")
	item := ir.NewIdent(itemName, elem)

	op := ">"
	if n.Name == "minstr" {
		op = "<"
	}
	better := ir.Bin("||",
		ir.Bin("==", ir.NewIdent(idx, ir.TInt), ir.IntLit("0"), ir.TBool),
		ir.Bin(op, l.toStr(item, nil), l.toStr(ir.NewIdent(best, elem), nil), ir.TBool),
		ir.TBool)

	decl := &ir.DeclStmt{Names: []string{best}, Type: elem}
	loop := &ir.Range{
		Key:    ir.NewIdent(idx, ir.TInt),
		Value:  item,
		X:      src,
		Define: true,
		Body: &ir.Block{Stmts: []ir.Stmt{&ir.If{
			Cond: better,
			Then: &ir.Block{Stmts: []ir.Stmt{
				assign("=", []ir.Expr{ir.NewIdent(best, elem)}, []ir.Expr{item}),
			}},
		}}},
	}
	l.setProv(decl, n)
	l.note(decl, "These values are not text, so each one is rendered before it is "+
		"compared. That is the difference between Perl's two families of comparison "+
		"and Go's one: in Go the operands already know what they are, so the "+
		"rendering has to be asked for.",
		"static-types-and-zero-values")
	l.emit(decl)
	l.emit(loop)
	return ir.NewIdent(best, elem)
}

// truth renders an expression as a Go bool, leaving one that already is alone.
func (l *Lowerer) truth(x ir.Expr) ir.Expr {
	if typeOrAny(x).Kind == ir.Bool {
		return x
	}
	return l.toBool(x, nil)
}

// setAlias records that a binding renders as an arbitrary expression, creating
// the map on first use.
func setAlias(m map[*Binding]ir.Expr, b *Binding, x ir.Expr) map[*Binding]ir.Expr {
	if m == nil {
		m = map[*Binding]ir.Expr{}
	}
	m[b] = x
	return m
}
