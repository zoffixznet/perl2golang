package lower

import "perl2go/internal/ir"

// Small constructors for the IR shapes this package builds constantly. They
// exist so that the type an expression carries is never forgotten: an IR node
// with no type makes the emitter guess, and a guessing emitter produces Go
// that does not compile.

func composite(t *ir.Type, keys, elems []ir.Expr) *ir.CompositeLit {
	c := &ir.CompositeLit{LitType: t, Elems: elems, Keys: keys}
	c.T = t
	return c
}

func index(x, i ir.Expr, t *ir.Type) *ir.Index {
	n := &ir.Index{X: x, Index: i}
	n.T = t
	return n
}

func indexComma(x, i ir.Expr, t *ir.Type) *ir.IndexComma {
	n := &ir.IndexComma{X: x, Index: i}
	n.T = t
	return n
}

func selector(x ir.Expr, sel string, t *ir.Type) *ir.Selector {
	n := &ir.Selector{X: x, Sel: sel}
	n.T = t
	return n
}

func conversion(to *ir.Type, x ir.Expr) *ir.Conversion {
	n := &ir.Conversion{To: to, X: x}
	n.T = to
	return n
}

func paren(x ir.Expr) *ir.Paren {
	n := &ir.Paren{X: x}
	if x != nil {
		n.T = x.Type()
	}
	return n
}

func slicing(x, lo, hi ir.Expr, t *ir.Type) *ir.SliceExpr {
	n := &ir.SliceExpr{X: x, Low: lo, High: hi}
	n.T = t
	return n
}

func funcLit(params []ir.Param, results []*ir.Type, body *ir.Block) *ir.FuncLit {
	n := &ir.FuncLit{Params: params, Results: results, Body: body}
	ptypes := make([]*ir.Type, len(params))
	for i, p := range params {
		ptypes[i] = p.Type
	}
	n.T = ir.FuncOf(ptypes, results)
	return n
}

// lenOf builds len(x).
func lenOf(x ir.Expr) *ir.Call {
	return ir.CallOf(ir.NewIdent("len", nil), ir.TInt, x)
}

// appendTo builds append(dst, vals...).
func appendTo(dst ir.Expr, vals ...ir.Expr) *ir.Call {
	t := ir.SliceOf(ir.TAny)
	if dst != nil && dst.Type() != nil {
		t = dst.Type()
	}
	return ir.CallOf(ir.NewIdent("append", nil), t, append([]ir.Expr{dst}, vals...)...)
}

// assign builds a plain assignment statement.
func assign(op string, lhs, rhs []ir.Expr) *ir.Assign {
	return &ir.Assign{Op: op, LHS: lhs, RHS: rhs}
}

// exprStmt wraps an expression as a statement.
func exprStmt(x ir.Expr) *ir.ExprStmt { return &ir.ExprStmt{X: x} }
