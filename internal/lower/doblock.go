package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file lowers `do BLOCK`, Perl's block used as an expression.
//
// Perl has no separate expression and statement worlds: a block is a term, and
// its value is whatever its last evaluated statement produced. Go draws the
// line hard, so there is nothing to translate the construct into directly.
//
// What a Go developer writes instead is the block's work as ordinary statements
// followed by the value, and that is what comes out here: the statements are
// lifted in front of the statement the block appeared in, and the block's final
// expression takes its place. `my $text = do { open ...; local $/; <$fh> }`
// becomes three lines of Go with no wrapper around them, which is both what the
// code means and what a Go developer would have written for it.
//
// The alternative, a function literal called on the spot, is legal Go and is
// what the construct literally is, but it reads as machinery rather than as
// code. It is used only where the block genuinely has to stay behind a
// condition.

// doBlock lowers `do { ... }` in expression position.
func (l *Lowerer) doBlock(n *ast.Call) ir.Expr {
	if len(n.Block) == 0 {
		out := ir.Nil(ir.TAny)
		l.setProv(out, n)
		return out
	}

	saved := l.scope
	l.scope = newScope(saved)
	stmts, value := l.blockValue(n.Block)
	if value == nil {
		// The block ends in something that is not an expression. A trailing
		// conditional is the common case and has a real Go shape, so it is
		// worth recognising; anything else has no value to carry out.
		if v, extra, ok := l.conditionalValue(n.Block); ok {
			stmts, value = extra, v
		}
	}
	l.scope = saved

	for _, st := range stmts {
		l.emit(st)
	}
	if value == nil {
		return l.todoExpr(n, "P2G3540", "do block",
			"the value of this block is not carried out of it",
			"A do block's value is whatever its last evaluated statement produced. "+
				"The last statement here is not one with a value, so there is nothing "+
				"for the surrounding expression to receive.",
			"Assign the value to a variable inside the block and read the variable "+
				"afterwards; the statements themselves converted and are above this.")
	}
	if len(stmts) > 0 {
		l.note(value, "Perl blocks are expressions: this one's value is whatever its "+
			"last statement produced. Go separates statements from expressions, so the "+
			"work happens on the lines above and the final value stands here. That is "+
			"what the block was doing, written out.",
			"statements-vs-expressions")
		l.concept("statements-vs-expressions")
	}
	return value
}

// doBlockStmts lowers `do { ... }` where its value is thrown away, which is
// what `EXPR or do { ... }` does with it.
func (l *Lowerer) doBlockStmts(n *ast.Call) []ir.Stmt {
	saved := l.scope
	l.scope = newScope(saved)
	out := l.stmts(n.Block)
	l.scope = saved
	return out
}

// conditionalValue recognises a block whose last statement is an if/elsif/else
// with an expression at the end of every branch, which is how Perl writes a
// conditional value.
//
// Go's answer is a variable declared once and assigned in each branch. The
// declaration says what the value is; the branches say what it is in each case.
func (l *Lowerer) conditionalValue(block []ast.Stmt) (ir.Expr, []ir.Stmt, bool) {
	if len(block) == 0 {
		return nil, nil, false
	}
	n, ok := block[len(block)-1].(*ast.If)
	if !ok || len(n.Else) == 0 {
		// Without an else the value is undef on the path that skips every
		// branch, and Perl's undef has no single Go spelling here.
		return nil, nil, false
	}
	branches := [][]ast.Stmt{n.Then}
	for _, ei := range n.ElseIfs {
		branches = append(branches, ei.Then)
	}
	branches = append(branches, n.Else)
	for _, b := range branches {
		if len(b) == 0 {
			return nil, nil, false
		}
		if _, isExpr := b[len(b)-1].(*ast.ExprStmt); !isExpr {
			return nil, nil, false
		}
	}

	lead := l.stmts(block[:len(block)-1])

	// Lower every branch first so the declared type covers all of them.
	type arm struct {
		cond  ast.Expr
		unles bool
		stmts []ir.Stmt
		value ir.Expr
	}
	arms := make([]arm, 0, len(branches))
	for i, b := range branches {
		sts, v := l.blockValue(b)
		if v == nil {
			return nil, nil, false
		}
		a := arm{stmts: sts, value: v}
		switch {
		case i == 0:
			a.cond, a.unles = n.Cond, n.Unless
		case i <= len(n.ElseIfs):
			a.cond = n.ElseIfs[i-1].Cond
		}
		arms = append(arms, a)
	}
	types := make([]*ir.Type, 0, len(arms))
	for _, a := range arms {
		types = append(types, typeOrAny(a.value))
	}
	t := joinAll(types)
	if t == nil || t.Kind == ir.Invalid {
		t = ir.TAny
	}

	name := l.tmp("value")
	decl := &ir.DeclStmt{Names: []string{name}, Type: t}
	l.setProv(decl, n)
	l.note(decl, "Perl hands back the value of whichever branch ran, because a block "+
		"is an expression there. Go declares the variable first and each branch "+
		"assigns to it, so the type is stated once and every path has to produce one.",
		"statements-vs-expressions", "static-types-and-zero-values")
	l.concept("statements-vs-expressions")
	target := ir.NewIdent(name, t)

	body := func(a arm) *ir.Block {
		return &ir.Block{Stmts: append(a.stmts,
			assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(a.value, t, nil)}))}
	}
	var tail ir.Stmt = body(arms[len(arms)-1])
	for i := len(arms) - 2; i >= 1; i-- {
		tail = &ir.If{Cond: l.cond(arms[i].cond), Then: body(arms[i]), Else: tail}
	}
	cond := l.cond(arms[0].cond)
	if arms[0].unles {
		cond = negated(cond)
	}
	out := &ir.If{Cond: cond, Then: body(arms[0]), Else: tail}
	l.setProv(out, n)

	return ir.NewIdent(name, t), append(append(lead, decl), out), true
}
