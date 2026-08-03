package gogen

import "perl2golang/internal/ir"

// Annotations attached to an expression have nowhere good to go. A line
// comment would swallow the rest of the statement, so the alternative is a
// block comment in the middle of an argument list, which pushes the code off
// the right-hand edge of the screen: the annotated program had lines of 700
// characters made almost entirely of comment.
//
// So an expression's notes are hoisted to the statement that evaluates it and
// written above it as ordinary line comments, in the order the expressions are
// read. The note still sits next to the code it explains, and the code is
// still readable.
//
// A TODO is hoisted the same way, for the same reason and one more. What used
// to keep it inline was that the exact position is the point of a TODO, and
// that is still true; what changed is that the expression standing in for a
// refusal now names its own diagnostic code, so the position is marked by the
// code rather than by the comment. A statement with three refused method calls
// in it was several hundred characters of comment wrapped around a little
// code, and the code around a refusal is exactly what a reader is there for.

// hoistedNotes returns the annotations to write above n: its own first, then
// those on the expressions it evaluates. It does not descend into a nested
// block, whose statements carry their own comments where they are emitted.
func hoistedNotes(n ir.Annotated) []ir.Note {
	var out []ir.Note
	if m := metaOf(n); m != nil {
		out = append(out, m.Notes...)
	}
	for _, x := range ownExprs(n) {
		walkExprNotes(x, &out)
	}
	return out
}

// hoistedTodos returns the TODOs of the expressions n evaluates, outermost
// first. A node's own TODO is not among them: the caller writes that one
// itself, and it is already in the right place.
func hoistedTodos(n ir.Annotated) []ir.Todo {
	var out []ir.Todo
	for _, x := range ownExprs(n) {
		walkExprTodos(x, &out)
	}
	return out
}

// walkExprTodos appends the TODOs on an expression and everything inside it.
func walkExprTodos(x ir.Expr, out *[]ir.Todo) {
	if x == nil {
		return
	}
	if m := metaOf(x); m != nil && m.Todo != nil {
		*out = append(*out, *m.Todo)
	}
	for _, sub := range subExprs(x) {
		walkExprTodos(sub, out)
	}
}

// walkExprNotes appends the notes on an expression and everything inside it,
// outermost first, which is the order the expression is read in.
func walkExprNotes(x ir.Expr, out *[]ir.Note) {
	if x == nil {
		return
	}
	if m := metaOf(x); m != nil {
		*out = append(*out, m.Notes...)
	}
	for _, sub := range subExprs(x) {
		walkExprNotes(sub, out)
	}
}

// ownExprs returns the expressions a declaration or statement evaluates
// itself, excluding anything inside a block it encloses.
func ownExprs(n ir.Annotated) []ir.Expr {
	switch x := n.(type) {
	case *ir.VarDecl:
		return x.Values
	case *ir.Assign:
		return append(append([]ir.Expr{}, x.LHS...), x.RHS...)
	case *ir.DeclStmt:
		return x.Values
	case *ir.ExprStmt:
		return []ir.Expr{x.X}
	case *ir.IncDec:
		return []ir.Expr{x.X}
	case *ir.If:
		return []ir.Expr{x.Cond}
	case *ir.For:
		return []ir.Expr{x.Cond}
	case *ir.Range:
		return []ir.Expr{x.X}
	case *ir.Return:
		return x.Results
	case *ir.Switch:
		return []ir.Expr{x.Tag}
	case *ir.Defer:
		return []ir.Expr{x.Call}
	case *ir.Go:
		return []ir.Expr{x.Call}
	}
	return nil
}

// subExprs returns the expressions directly inside x. A function literal's
// body is a block and is deliberately not included.
func subExprs(x ir.Expr) []ir.Expr {
	switch e := x.(type) {
	case *ir.Call:
		return append([]ir.Expr{e.Fun}, e.Args...)
	case *ir.Selector:
		return []ir.Expr{e.X}
	case *ir.Index:
		return []ir.Expr{e.X, e.Index}
	case *ir.IndexComma:
		return []ir.Expr{e.X, e.Index}
	case *ir.SliceExpr:
		return []ir.Expr{e.X, e.Low, e.High}
	case *ir.Binary:
		return []ir.Expr{e.L, e.R}
	case *ir.Unary:
		return []ir.Expr{e.X}
	case *ir.Paren:
		return []ir.Expr{e.X}
	case *ir.CompositeLit:
		return append(append([]ir.Expr{}, e.Keys...), e.Elems...)
	case *ir.TypeAssert:
		return []ir.Expr{e.X}
	case *ir.Conversion:
		return []ir.Expr{e.X}
	}
	return nil
}
