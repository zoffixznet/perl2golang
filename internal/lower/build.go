package lower

import "perl2golang/internal/ir"

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
	// A variadic tail is part of the type, and a function value whose type
	// forgets it cannot be stored anywhere the real function fits.
	if len(params) > 0 && params[len(params)-1].Variadic {
		n.T.Variadic = true
	}
	return n
}

// plusOne is x + 1, folded when x is written out as a number.
//
// The generated code is read as well as run, and `grow(xs, 8)` says what
// `grow(xs, 7+1)` only implies.
func plusOne(x ir.Expr) ir.Expr {
	if lit, ok := x.(*ir.Lit); ok && lit.Kind == ir.LitInt {
		if n, ok := strconvAtoi(lit.Value); ok {
			return ir.IntLit(itoa(n + 1))
		}
	}
	return ir.Bin("+", x, ir.IntLit("1"), ir.TInt)
}

// atLeastZero is max(x, 0), left alone when x is written out as a number that
// is already not negative.
func atLeastZero(x ir.Expr) ir.Expr {
	if lit, ok := x.(*ir.Lit); ok && lit.Kind == ir.LitInt {
		if _, ok := strconvAtoi(lit.Value); ok {
			return x
		}
	}
	return ir.CallOf(ir.NewIdent("max", nil), ir.TInt, x, ir.IntLit("0"))
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

// negated is `!x`, written the way a Go developer would write it.
//
// A negated equality test has a spelling of its own: `m == nil` rather than
// `!(m != nil)`. The two say the same thing for every type, including the
// floating-point values where flipping an ordering comparison would not, and
// the direct form is what the generated code is read for.
func negated(x ir.Expr) ir.Expr {
	if b, ok := x.(*ir.Binary); ok {
		flip := ""
		switch b.Op {
		case "==":
			flip = "!="
		case "!=":
			flip = "=="
		case "<", ">", "<=", ">=":
			// Flipping an ordering comparison is only the same statement when
			// the values are totally ordered. Floats are not: with a NaN on
			// either side both `a < b` and `a >= b` are false, so `!(a < b)`
			// and `a >= b` disagree.
			if totallyOrdered(b.L) && totallyOrdered(b.R) {
				flip = map[string]string{"<": ">=", ">": "<=", "<=": ">", ">=": "<"}[b.Op]
			}
		}
		// `!(len(x) > 0)` is `len(x) == 0`, which is the spelling every Go
		// reader expects. `<= 0` is the same test only because a length is
		// never negative, which is a fact about len rather than about ints.
		if flip == "<=" && isLenCall(b.L) && isZero(b.R) {
			flip = "=="
		}
		if flip != "" {
			out := ir.Bin(flip, b.L, b.R, ir.TBool)
			if m := ir.MetaOf(b); m != nil {
				*ir.MetaOf(out) = *m
			}
			return out
		}
	}
	if u, ok := x.(*ir.Unary); ok && u.Op == "!" {
		return u.X
	}
	return ir.Un("!", x, ir.TBool)
}

// totallyOrdered reports whether an expression's type has no unordered values,
// which is what makes flipping a comparison operator sound.
func totallyOrdered(x ir.Expr) bool {
	t := typeOrAny(x)
	return t.Kind == ir.Int || t.Kind == ir.String
}

// isLenCall reports whether an expression is a call to the len builtin, whose
// result is never negative.
func isLenCall(x ir.Expr) bool {
	c, ok := x.(*ir.Call)
	if !ok {
		return false
	}
	id, ok := c.Fun.(*ir.Ident)
	return ok && id.Name == "len"
}

// isZero reports whether an expression is the integer literal 0.
func isZero(x ir.Expr) bool {
	lit, ok := x.(*ir.Lit)
	return ok && lit.Kind == ir.LitInt && lit.Value == "0"
}
