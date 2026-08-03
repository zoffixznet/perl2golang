package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file recognises `while (my ($k, $v) = each %h)`, which is how Perl walks
// a hash a pair at a time.
//
// `each` is an iterator stored inside the hash itself, which is why abandoning
// one such loop halfway leaves the next one starting from the middle, and why
// nesting two of them over the same hash does not work. Go's `for k, v := range
// m` keeps no state anywhere: it starts at the beginning every time, and there
// is nothing to reset. That makes the loop form a straight translation and the
// standalone call something else entirely, which is why only the loop is
// recognised here.

// eachLoop lowers the pair-at-a-time hash walk. It reports whether the
// condition was that shape.
func (l *Lowerer) eachLoop(n *ast.While) ([]ir.Stmt, bool) {
	key, val, hash, ok := eachLoopShape(n.Cond)
	if !ok || n.Until {
		return nil, false
	}

	saved := l.scope
	l.scope = newScope(saved)
	defer func() { l.scope = saved }()

	src := l.expr(hash)
	t := typeOrAny(src)
	if t.Kind != ir.Map {
		return nil, false
	}

	kb := l.declare(key, KindLoop)
	kb.Type = keyOf(t)
	l.observe(kb, kb.Type)
	kb.Reads++
	var value ir.Expr
	if val != nil {
		vb := l.declare(val, KindLoop)
		vb.Type = elemOf(t)
		l.observe(vb, vb.Type)
		vb.Reads++
		value = ir.NewIdent(vb.Go, vb.Type)
	}

	out := &ir.Range{
		Key:    ir.NewIdent(kb.Go, kb.Type),
		Value:  value,
		X:      src,
		Define: true,
		Body:   l.markUnused(&ir.Block{Stmts: l.stmts(n.Body)}),
		Label:  l.label(n.Label),
	}
	l.setProv(out, n)
	l.note(out, "each walks a hash a pair at a time using an iterator kept inside "+
		"the hash, which is why abandoning one such loop halfway leaves the next one "+
		"starting from the middle. Go's range keeps no state at all: it starts at "+
		"the beginning every time, and both halves of the pair are named in the "+
		"loop header.",
		"map-iteration-order", "range-is-not-foreach")
	l.approximate(n, "P2G5570", "each",
		"the loop walks the map with range and keeps no iterator",
		"each remembers where it got to inside the hash itself. Go's range starts "+
			"afresh every time, so a loop that used to resume where an earlier one "+
			"stopped now starts at the beginning.",
		"Both languages visit the pairs in an order you cannot rely on. Sort the "+
			"keys and range over those where the order matters.",
		"map-iteration-order")
	return []ir.Stmt{out}, true
}

// eachLoopShape reads `my ($k, $v) = each %h` out of a loop condition.
func eachLoopShape(cond ast.Expr) (key, val *ast.Var, hash ast.Expr, ok bool) {
	a, isAssign := cond.(*ast.Assign)
	if !isAssign || a.Op != "=" {
		return nil, nil, nil, false
	}
	c, isCall := a.RHS.(*ast.Call)
	if !isCall || c.Name != "each" || len(c.Args) != 1 {
		return nil, nil, nil, false
	}
	var vars []*ast.Var
	switch lhs := a.LHS.(type) {
	case *ast.My:
		vars = declaredVars(lhs)
	default:
		return nil, nil, nil, false
	}
	if len(vars) == 0 || len(vars) > 2 {
		return nil, nil, nil, false
	}
	key = vars[0]
	if len(vars) == 2 {
		val = vars[1]
	}
	return key, val, c.Args[0], true
}

// keyOf returns a map's key type, which is text unless something said
// otherwise.
func keyOf(t *ir.Type) *ir.Type {
	if t != nil && t.Kind == ir.Map && t.Key != nil {
		return t.Key
	}
	return ir.TString
}
