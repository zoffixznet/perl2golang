package lower

import (
	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// foreachStmt lowers `foreach`, which is the loop Perl developers write most
// and the one whose Go translation surprises them most.
//
// Three things differ and all three matter. Go's one-variable range gives the
// index, not the element. Go's element is a copy, so assigning to it changes
// nothing, while Perl's loop variable aliases the real element. And a range
// over 1..10 in Perl builds the whole list, where Go just counts.
func (l *Lowerer) foreachStmt(n *ast.Foreach) []ir.Stmt {
	saved := l.scope
	l.scope = newScope(saved)
	defer func() { l.scope = saved }()

	// A range with numeric endpoints becomes a counting loop, which is what a
	// Go developer writes and what avoids allocating the list at all.
	if st, ok := l.countingLoop(n); ok {
		return st
	}

	src := l.listSource(n)
	if src == nil {
		return nil
	}
	setup := l.takePre()
	elem := elemOf(typeOrAny(src))

	var loopVar *ast.Var
	if n.Var != nil {
		if v, ok := n.Var.(*ast.Var); ok {
			loopVar = v
		}
	}

	// Perl's loop variable aliases the element, so a body that assigns to it
	// is modifying the array. Go's is a copy, so that has to become an
	// indexed loop or the assignment silently does nothing.
	if loopVar != nil && l.assignsTo(n.Body, loopVar) {
		return l.aliasingLoop(n, src, elem, loopVar, setup)
	}

	var b *Binding
	if loopVar != nil {
		if n.MyVar {
			b = l.declare(loopVar, KindLoop)
		} else {
			b = l.lookup(loopVar.Sigil, loopVar.Name, loopVar)
		}
	} else {
		b = l.declareNamed(topicKey(n), '$', topicName(n), KindLoop, n)
		b.Perl = "$_"
	}
	l.observe(b, elem)
	if l.pass == 2 {
		b.Type = elem
	}

	value := ir.NewIdent(b.Go, b.Type)
	if loopVar == nil {
		l.topicStack = append(l.topicStack, value)
		defer func() { l.topicStack = l.topicStack[:len(l.topicStack)-1] }()
	}

	out := &ir.Range{
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  value,
		X:      src,
		Define: true,
		Body:   l.block(n.Body),
		Label:  n.Label,
	}
	l.setProv(out, n)
	if loopVar == nil {
		l.note(out, "The loop had no variable, so Perl put each element in $_. Go has "+
			"no such implicit variable; the element is given a name, which is what makes "+
			"the body readable on its own.",
			"range-is-not-foreach")
	} else {
		l.note(out, "Go's range gives two values: the index and the element. The index "+
			"is discarded here with _, the blank identifier, because Go will not let an "+
			"unused variable compile. Writing `for x := range list` would bind the "+
			"index, not the element, which is the single most common mistake coming "+
			"from foreach.",
			"range-is-not-foreach")
	}
	if len(setup) > 0 {
		return append(setup, out)
	}
	return []ir.Stmt{out}
}

// topicKey and topicName give the implicit loop variable a stable identity
// across the two passes and a readable name in the output.
func topicKey(n *ast.Foreach) string { return "topic@" + itoa(posLine(n)) + ":" + itoa(n.Pos().Col) }

func topicName(n *ast.Foreach) string { return "item" }

// listSource lowers the thing being iterated.
func (l *Lowerer) listSource(n *ast.Foreach) ir.Expr {
	if len(n.List) == 1 {
		return l.list(n.List[0])
	}
	parts, t := l.listParts(n.List)
	if len(parts) == 1 && typeOrAny(parts[0]).Kind == ir.Slice {
		return parts[0]
	}
	return composite(ir.SliceOf(t), nil, parts)
}

// countingLoop recognises `for my $i (A .. B)` and emits a counting loop.
func (l *Lowerer) countingLoop(n *ast.Foreach) ([]ir.Stmt, bool) {
	if len(n.List) != 1 {
		return nil, false
	}
	r, ok := n.List[0].(*ast.BinOp)
	if !ok || r.Op != ".." {
		return nil, false
	}
	low := l.expr(r.L)
	high := l.expr(r.R)
	if typeOrAny(low).Kind == ir.String || typeOrAny(high).Kind == ir.String {
		return nil, false
	}
	low = l.toInt(low, r.L)
	high = l.toInt(high, r.R)

	var b *Binding
	if v, ok := n.Var.(*ast.Var); ok {
		if n.MyVar {
			b = l.declare(v, KindLoop)
		} else {
			b = l.lookup(v.Sigil, v.Name, v)
		}
	} else {
		b = l.declareNamed(topicKey(n), '$', "i", KindLoop, n)
		b.Perl = "$_"
	}
	l.observe(b, ir.TInt)
	if l.pass == 2 {
		b.Type = ir.TInt
	}
	idx := ir.NewIdent(b.Go, ir.TInt)
	if n.Var == nil {
		l.topicStack = append(l.topicStack, idx)
		defer func() { l.topicStack = l.topicStack[:len(l.topicStack)-1] }()
	}

	out := &ir.For{
		Init:  assign(":=", []ir.Expr{idx}, []ir.Expr{low}),
		Cond:  ir.Bin("<=", idx, high, ir.TBool),
		Post:  &ir.IncDec{X: idx},
		Body:  l.block(n.Body),
		Label: n.Label,
	}
	l.setProv(out, n)
	l.note(out, "Perl's range operator builds the whole list before the loop starts, "+
		"so `for (1 .. 1_000_000)` allocates a million numbers. Go counts instead and "+
		"allocates nothing. Note the <= in the condition: Perl's range includes both "+
		"ends.",
		"range-is-not-foreach")
	return []ir.Stmt{out}, true
}

// aliasingLoop emits the indexed form for a loop whose body modifies the
// element.
func (l *Lowerer) aliasingLoop(n *ast.Foreach, src ir.Expr, elem *ir.Type, loopVar *ast.Var, setup []ir.Stmt) []ir.Stmt {
	idxName := l.tmp("i")
	idx := ir.NewIdent(idxName, ir.TInt)

	// The loop variable becomes the indexed element, so every read and write
	// of it in the body reaches the real element.
	b := l.declare(loopVar, KindLoop)
	l.observe(b, elem)
	if l.pass == 2 {
		b.Type = elem
	}
	b.Go = "" // never referred to by name
	l.aliasFor(b, index(src, idx, elem))

	out := &ir.Range{
		Key:    idx,
		X:      src,
		Define: true,
		Body:   l.block(n.Body),
		Label:  n.Label,
	}
	l.setProv(out, n)
	l.note(out, "Perl's foreach variable is an alias for the element, so assigning to "+
		"it modifies the array. Go's range variable is a copy, and assigning to it "+
		"would change nothing at all. Because the body here does assign to it, the "+
		"loop ranges over the index and writes back through the slice, which is how "+
		"Go modifies elements in place.",
		"range-is-not-foreach", "slice-aliasing-and-copy")
	if len(setup) > 0 {
		return append(setup, out)
	}
	return []ir.Stmt{out}
}

// aliasFor makes a binding render as an arbitrary expression rather than as a
// plain identifier, which is how the aliasing loop rewrites its variable.
func (l *Lowerer) aliasFor(b *Binding, x ir.Expr) {
	if l.aliases == nil {
		l.aliases = map[*Binding]ir.Expr{}
	}
	l.aliases[b] = x
}

// assignsTo reports whether a statement list assigns to the given variable,
// which decides whether Perl's aliasing was load-bearing.
func (l *Lowerer) assignsTo(body []ast.Stmt, v *ast.Var) bool {
	found := false
	var walkExpr func(ast.Expr)
	var walkStmts func([]ast.Stmt)

	names := func(e ast.Expr) bool {
		vv, ok := e.(*ast.Var)
		return ok && vv.Sigil == v.Sigil && vv.Name == v.Name
	}
	walkExpr = func(e ast.Expr) {
		switch n := e.(type) {
		case nil:
			return
		case *ast.Assign:
			if names(n.LHS) {
				found = true
			}
			walkExpr(n.LHS)
			walkExpr(n.RHS)
		case *ast.UnOp:
			if (n.Op == "++" || n.Op == "--") && names(n.X) {
				found = true
			}
			walkExpr(n.X)
		case *ast.BinOp:
			walkExpr(n.L)
			walkExpr(n.R)
		case *ast.Ternary:
			walkExpr(n.Cond)
			walkExpr(n.A)
			walkExpr(n.B)
		case *ast.List:
			for _, el := range n.Elems {
				walkExpr(el)
			}
		case *ast.Call:
			for _, a := range n.Args {
				walkExpr(a)
			}
			walkStmts(n.Block)
		case *ast.Subst:
			if names(n.Bound) {
				found = true
			}
		case *ast.Trans:
			if names(n.Bound) {
				found = true
			}
		}
	}
	walkStmts = func(list []ast.Stmt) {
		for _, st := range list {
			switch n := st.(type) {
			case *ast.ExprStmt:
				walkExpr(n.X)
			case *ast.If:
				walkExpr(n.Cond)
				walkStmts(n.Then)
				for _, ei := range n.ElseIfs {
					walkExpr(ei.Cond)
					walkStmts(ei.Then)
				}
				walkStmts(n.Else)
			case *ast.While:
				walkExpr(n.Cond)
				walkStmts(n.Body)
			case *ast.ForC:
				walkStmts(n.Body)
			case *ast.Foreach:
				walkStmts(n.Body)
			case *ast.Block:
				walkStmts(n.Body)
			case *ast.Return:
				for _, e := range n.Exprs {
					walkExpr(e)
				}
			}
		}
	}
	walkStmts(body)
	return found
}
