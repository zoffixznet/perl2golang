package convert

import "perl2golang/internal/ir"

// walkIR visits a node and everything under it, calling fn for each annotated
// node in source order.
//
// It lives here rather than in the ir package because the walkthrough is the
// only thing that needs a full traversal, and a visitor in ir would have to be
// kept exhaustive by everyone who adds a node.
func walkIR(n ir.Annotated, fn func(ir.Annotated)) {
	if n == nil {
		return
	}
	fn(n)

	switch x := n.(type) {
	// Declarations.
	case *ir.FuncDecl:
		walkBlock(x.Body, fn)
	case *ir.VarDecl:
		walkExprs(x.Values, fn)
	case *ir.TypeDecl, *ir.ImportDecl, *ir.RawDecl:
		// Nothing to descend into.

	// Statements.
	case *ir.Block:
		for _, s := range x.Stmts {
			walkIR(s, fn)
		}
	case *ir.Assign:
		walkExprs(x.LHS, fn)
		walkExprs(x.RHS, fn)
	case *ir.DeclStmt:
		walkExprs(x.Values, fn)
	case *ir.ExprStmt:
		walkExpr(x.X, fn)
	case *ir.IncDec:
		walkExpr(x.X, fn)
	case *ir.If:
		walkIR(x.Init, fn)
		walkExpr(x.Cond, fn)
		walkBlock(x.Then, fn)
		walkIR(x.Else, fn)
	case *ir.For:
		walkIR(x.Init, fn)
		walkExpr(x.Cond, fn)
		walkIR(x.Post, fn)
		walkBlock(x.Body, fn)
	case *ir.Range:
		walkExpr(x.Key, fn)
		walkExpr(x.Value, fn)
		walkExpr(x.X, fn)
		walkBlock(x.Body, fn)
	case *ir.Return:
		walkExprs(x.Results, fn)
	case *ir.Labeled:
		walkIR(x.Stmt, fn)
	case *ir.Switch:
		walkIR(x.Init, fn)
		walkExpr(x.Tag, fn)
		for _, c := range x.Cases {
			walkExprs(c.Values, fn)
			walkBlock(c.Body, fn)
		}
	case *ir.Defer:
		walkExpr(x.Call, fn)
	case *ir.Go:
		walkExpr(x.Call, fn)
	case *ir.BlockStmt:
		walkBlock(x.Body, fn)
	case *ir.TodoStmt:
		walkExpr(x.Stub, fn)
	case *ir.Branch, *ir.CommentStmt, *ir.RawStmt:
		// Leaves.

	// Expressions.
	case *ir.Call:
		walkExpr(x.Fun, fn)
		walkExprs(x.Args, fn)
	case *ir.Selector:
		walkExpr(x.X, fn)
	case *ir.Index:
		walkExpr(x.X, fn)
		walkExpr(x.Index, fn)
	case *ir.IndexComma:
		walkExpr(x.X, fn)
		walkExpr(x.Index, fn)
	case *ir.SliceExpr:
		walkExpr(x.X, fn)
		walkExpr(x.Low, fn)
		walkExpr(x.High, fn)
	case *ir.Binary:
		walkExpr(x.L, fn)
		walkExpr(x.R, fn)
	case *ir.Unary:
		walkExpr(x.X, fn)
	case *ir.Paren:
		walkExpr(x.X, fn)
	case *ir.CompositeLit:
		walkExprs(x.Keys, fn)
		walkExprs(x.Elems, fn)
	case *ir.FuncLit:
		walkBlock(x.Body, fn)
	case *ir.TypeAssert:
		walkExpr(x.X, fn)
	case *ir.Conversion:
		walkExpr(x.X, fn)
	case *ir.Ident, *ir.Lit, *ir.RawExpr:
		// Leaves.
	}
}

func walkBlock(b *ir.Block, fn func(ir.Annotated)) {
	if b == nil {
		return
	}
	walkIR(b, fn)
}

func walkExpr(e ir.Expr, fn func(ir.Annotated)) {
	if e == nil {
		return
	}
	walkIR(e, fn)
}

func walkExprs(es []ir.Expr, fn func(ir.Annotated)) {
	for _, e := range es {
		walkExpr(e, fn)
	}
}
