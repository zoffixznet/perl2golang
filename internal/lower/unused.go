package lower

import "perl2golang/internal/ir"

// Go refuses to compile a function that declares a local and never reads it.
// Perl has no such rule, so a script quite reasonably declares a variable for
// documentation, for symmetry with a sibling branch, or for an edit that never
// happened. Dropping the declaration would hide something the developer wrote,
// so the blank identifier is used instead, which is Go's own way of saying the
// variable is unread on purpose.
//
// The check runs over one block at a time. A name declared in this block is
// read if it appears anywhere in it, including inside nested blocks, in any
// position other than as the target of an assignment.

// markUnused appends a blank assignment for every local this block declares
// and never reads.
func (l *Lowerer) markUnused(b *ir.Block) *ir.Block {
	if b == nil || l.pass != 2 {
		return b
	}
	// A range loop declares its variables in its own header, where the only
	// way to say "unread" is to leave them out.
	for _, st := range b.Stmts {
		r, ok := st.(*ir.Range)
		if !ok || !r.Define {
			continue
		}
		value, ok := r.Value.(*ir.Ident)
		if !ok || value.Name == "_" {
			continue
		}
		inner := map[string]int{}
		countReads(r.Body, inner)
		if inner[value.Name] > 0 {
			continue
		}
		key, keyed := r.Key.(*ir.Ident)
		if !keyed || key.Name == "_" {
			r.Key, r.Value = nil, nil
			ir.Annotate(r, "Neither the index nor the element is read in the body, so "+
				"the loop names neither. `for range x` counts the iterations and nothing "+
				"else, which Go allows and which says plainly what the loop is for.",
				"range-is-not-foreach")
			continue
		}
		r.Value = ir.NewIdent("_", nil)
	}

	declared := declaredNames(b.Stmts)
	if len(declared) == 0 {
		return b
	}
	reads := map[string]int{}
	for _, st := range b.Stmts {
		countReads(st, reads)
	}
	for _, name := range declared {
		if reads[name] > 0 {
			continue
		}
		st := assign("=", []ir.Expr{ir.NewIdent("_", nil)}, []ir.Expr{ir.NewIdent(name, nil)})
		ir.Annotate(st, "Go will not compile a local variable that is never read. "+
			name+" is declared and never read, and assigning it to the blank "+
			"identifier says that is deliberate rather than a mistake.",
			"var-vs-short-declaration")
		b.Stmts = append(b.Stmts, st)
	}
	return b
}

// declaredNames lists the locals a statement list introduces at its own level.
func declaredNames(stmts []ir.Stmt) []string {
	var out []string
	add := func(name string) {
		if name == "" || name == "_" {
			return
		}
		out = append(out, name)
	}
	for _, st := range stmts {
		switch n := st.(type) {
		case *ir.Assign:
			if n.Op != ":=" {
				continue
			}
			for _, lhs := range n.LHS {
				if id, ok := lhs.(*ir.Ident); ok {
					add(id.Name)
				}
			}
		case *ir.DeclStmt:
			for _, name := range n.Names {
				add(name)
			}
		}
	}
	return out
}

// countReads adds every identifier read in a statement to counts. An
// identifier written to and never read is not a read.
func countReads(st ir.Stmt, counts map[string]int) {
	switch n := st.(type) {
	case nil:
		return
	case *ir.Block:
		for _, s := range n.Stmts {
			countReads(s, counts)
		}
	case *ir.Assign:
		// The left side is a write when it is a bare name; anything else,
		// such as m[k] or a[i], reads the container.
		for _, lhs := range n.LHS {
			if _, plain := lhs.(*ir.Ident); !plain {
				countExprReads(lhs, counts)
			}
		}
		// A compound assignment reads its target as well as writing it.
		if n.Op != "=" && n.Op != ":=" {
			for _, lhs := range n.LHS {
				countExprReads(lhs, counts)
			}
		}
		for _, rhs := range n.RHS {
			countExprReads(rhs, counts)
		}
	case *ir.DeclStmt:
		for _, v := range n.Values {
			countExprReads(v, counts)
		}
	case *ir.ExprStmt:
		countExprReads(n.X, counts)
	case *ir.IncDec:
		countExprReads(n.X, counts)
	case *ir.If:
		countReads(n.Init, counts)
		countExprReads(n.Cond, counts)
		countReads(n.Then, counts)
		countReads(n.Else, counts)
	case *ir.For:
		countReads(n.Init, counts)
		countExprReads(n.Cond, counts)
		countReads(n.Post, counts)
		countReads(n.Body, counts)
	case *ir.Range:
		countExprReads(n.X, counts)
		countReads(n.Body, counts)
	case *ir.Return:
		for _, r := range n.Results {
			countExprReads(r, counts)
		}
	case *ir.Labeled:
		countReads(n.Stmt, counts)
	case *ir.Switch:
		countReads(n.Init, counts)
		countExprReads(n.Tag, counts)
		for _, c := range n.Cases {
			for _, v := range c.Values {
				countExprReads(v, counts)
			}
			countReads(c.Body, counts)
		}
	case *ir.Defer:
		countExprReads(n.Call, counts)
	case *ir.Go:
		countExprReads(n.Call, counts)
	case *ir.BlockStmt:
		countReads(n.Body, counts)
	}
}

// countExprReads adds every identifier an expression reads to counts.
func countExprReads(e ir.Expr, counts map[string]int) {
	switch n := e.(type) {
	case nil:
		return
	case *ir.Ident:
		counts[n.Name]++
	case *ir.Call:
		countExprReads(n.Fun, counts)
		for _, a := range n.Args {
			countExprReads(a, counts)
		}
	case *ir.Selector:
		countExprReads(n.X, counts)
	case *ir.Index:
		countExprReads(n.X, counts)
		countExprReads(n.Index, counts)
	case *ir.IndexComma:
		countExprReads(n.X, counts)
		countExprReads(n.Index, counts)
	case *ir.SliceExpr:
		countExprReads(n.X, counts)
		countExprReads(n.Low, counts)
		countExprReads(n.High, counts)
	case *ir.Binary:
		countExprReads(n.L, counts)
		countExprReads(n.R, counts)
	case *ir.Unary:
		countExprReads(n.X, counts)
	case *ir.Paren:
		countExprReads(n.X, counts)
	case *ir.CompositeLit:
		for _, k := range n.Keys {
			countExprReads(k, counts)
		}
		for _, el := range n.Elems {
			countExprReads(el, counts)
		}
	case *ir.FuncLit:
		countReads(n.Body, counts)
	case *ir.TypeAssert:
		countExprReads(n.X, counts)
	case *ir.Conversion:
		countExprReads(n.X, counts)
	}
}
