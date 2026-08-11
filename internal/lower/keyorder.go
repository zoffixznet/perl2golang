package lower

import "perl2golang/internal/perl/ast"

// Hash iteration order is random in both languages and random in different
// ways, which is the part that catches people.
//
// Perl decides an order once per process: two passes over a hash nobody has
// touched come out in the same order, and a script that prints the keys and
// then prints something keyed the same way relies on that without ever saying
// so. Go randomises on every iteration, so the same two passes disagree with
// each other, and a program that agreed with itself stops agreeing with
// itself.
//
// Where a file iterates one hash more than once and never changes it, the
// order is taken once into a variable and the passes share it. That is what a
// Go developer would write for the same reason, it costs one line, and it
// leaves the order random between runs, which is true of the original too.

// planKeyOrder returns the `keys` and `values` call sites that may share one
// captured order: those reading a hash the file iterates more than once and
// never modifies.
//
// The test is deliberately whole-file and deliberately crude. A hash written
// to anywhere, referenced anywhere, or read through a nested key anywhere
// (which creates the levels above it) is left alone entirely, because the
// cost of being wrong here is an order that silently stops matching the
// program's own second pass.
func planKeyOrder(list []ast.Stmt) map[*ast.Call]string {
	sites := map[string][]*ast.Call{}
	mutated := map[string]bool{}
	mark := func(e ast.Expr) {
		if name, ok := hashRootName(e); ok {
			mutated[name] = true
		}
	}
	walkExprs(list, func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.Call:
			switch n.Name {
			case "keys", "values":
				if len(n.Args) == 1 {
					if v, ok := n.Args[0].(*ast.Var); ok && v.Sigil == '%' {
						sites["%"+v.Name] = append(sites["%"+v.Name], n)
					}
				}
			case "delete", "undef", "each", "local":
				for _, a := range n.Args {
					mark(a)
				}
			}
		case *ast.Assign:
			// The declaration that fills the hash comes before every use of
			// it, so it is not a change any later pass could disagree with.
			if _, decl := n.LHS.(*ast.My); !decl {
				mark(n.LHS)
			}
		case *ast.RefGen:
			mark(n.X)
		case *ast.UnOp:
			if n.Op == "++" || n.Op == "--" {
				mark(n.X)
			}
		case *ast.HashIndex:
			// Reading `$h{a}{b}` creates the level above it, so a nested read
			// is a write as far as the key order is concerned.
			switch n.Base.(type) {
			case *ast.HashIndex, *ast.Index:
				mark(n.Base)
			}
		}
	})
	out := map[*ast.Call]string{}
	for name, calls := range sites {
		if len(calls) < 2 || mutated[name] {
			continue
		}
		for _, c := range calls {
			out[c] = name
		}
	}
	return out
}

// hashRootName names the hash an expression writes into, when it writes into
// one at all: `%h`, `$h{k}`, `@h{...}` and a list of any of them all name %h.
func hashRootName(e ast.Expr) (string, bool) {
	switch n := e.(type) {
	case *ast.Var:
		if n.Sigil == '%' {
			return "%" + n.Name, true
		}
	case *ast.HashIndex:
		if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' {
			return "%" + v.Name, true
		}
		return hashRootName(n.Base)
	case *ast.Slice:
		if v, ok := n.Base.(*ast.Var); ok && n.Hash {
			return "%" + v.Name, true
		}
	case *ast.List:
		for _, el := range n.Elems {
			if name, ok := hashRootName(el); ok {
				return name, true
			}
		}
	}
	return "", false
}
