package lower

import (
	"perl2golang/internal/perl/ast"
)

// This file finds the places a sub writes through @_.
//
// The elements of @_ alias the caller's variables: `sub bump { $_[0]++ }`
// changes the variable at the call site, and a whole idiom of in-place
// helpers is built on that. A Go parameter is a copy, so the converted write
// stays inside the function, and the program's behaviour changes silently at
// the call site unless the reader is told. Go's spelling of the same intent
// is a pointer parameter, which is also the lesson worth teaching here.

// reportArgAliasing walks a sub body and reports every write that reached
// the caller through @_ in the original and no longer does.
func (l *Lowerer) reportArgAliasing(body []ast.Stmt) {
	if l.pass != 2 {
		return
	}
	seen := map[ast.Node]bool{}
	raise := func(n ast.Node, what string) {
		if n == nil || seen[n] {
			return
		}
		seen[n] = true
		l.approximate(n, "P2G2020", "a write through @_",
			"the write stays inside this function",
			"In the original, "+what+" wrote through @_, whose elements alias the "+
				"caller's variables, so the call site saw the change. A Go parameter "+
				"is a copy: this write now stays inside the function and the caller "+
				"keeps its old value.",
			"Take a pointer parameter and write through it, `func f(x *string) { *x = ... }`, "+
				"called as `f(&v)`; or return the new value and assign it at the call "+
				"site, which is the more common Go shape.",
			"pointers-vs-references", "variadic-and-no-defaults")
	}

	isTopic := func(e ast.Expr) bool {
		v, ok := e.(*ast.Var)
		return ok && v.Sigil == '$' && v.Name == "_"
	}
	isArgsElem := func(e ast.Expr) bool {
		n, ok := e.(*ast.Index)
		if !ok || n.Arrow {
			return false
		}
		return isTopic(n.Base)
	}
	isArgsSlice := func(e ast.Expr) bool {
		n, ok := e.(*ast.Slice)
		if !ok {
			return false
		}
		return isArgsVar(n.Base)
	}
	mentionsArgs := func(list []ast.Expr) bool {
		for _, e := range list {
			if isArgsVar(e) {
				return true
			}
		}
		return false
	}
	// topicWrites reports whether a foreach body writes its $_ topic, which
	// over @_ is a write to every caller argument at once.
	var topicWrites func(list []ast.Stmt) bool
	topicWrites = func(list []ast.Stmt) bool {
		found := false
		walkExprs(list, func(e ast.Expr) {
			switch n := e.(type) {
			case *ast.Assign:
				if isTopic(n.LHS) {
					found = true
				}
			case *ast.UnOp:
				if (n.Op == "++" || n.Op == "--") && isTopic(n.X) {
					found = true
				}
			case *ast.Subst:
				if n.Bound == nil || isTopic(n.Bound) {
					found = true
				}
			case *ast.Trans:
				if n.Bound == nil || isTopic(n.Bound) {
					found = true
				}
			case *ast.Call:
				if (n.Name == "chomp" || n.Name == "chop") &&
					(argCount(n) == 0 || isTopic(flatten(argList(n))[0])) {
					found = true
				}
			}
		})
		return found
	}

	var walkS func([]ast.Stmt)
	walkS = func(list []ast.Stmt) {
		for _, st := range list {
			switch n := st.(type) {
			case *ast.Foreach:
				if n.Var == nil && mentionsArgs(n.List) && topicWrites(n.Body) {
					raise(n, "the loop over `@_`")
				}
				walkS(n.Body)
			case *ast.If:
				walkS(n.Then)
				for _, ei := range n.ElseIfs {
					walkS(ei.Then)
				}
				walkS(n.Else)
			case *ast.While:
				walkS(n.Body)
			case *ast.ForC:
				walkS(n.Body)
			case *ast.Block:
				walkS(n.Body)
			case *ast.SubDecl:
				walkS(n.Body)
			}
		}
	}
	walkS(body)

	walkExprs(body, func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.Assign:
			switch {
			case isArgsElem(n.LHS):
				raise(n, "the assignment to `"+l.snippet(n.LHS)+"`")
			case isArgsSlice(n.LHS):
				raise(n, "the assignment to `"+l.snippet(n.LHS)+"`")
			}
		case *ast.UnOp:
			if (n.Op == "++" || n.Op == "--") && isArgsElem(n.X) {
				raise(n, "`"+l.snippet(n)+"`")
			}
		case *ast.Subst:
			if isArgsElem(n.Bound) {
				raise(n, "the substitution on `"+l.snippet(n.Bound)+"`")
			}
		case *ast.Call:
			if (n.Name == "chomp" || n.Name == "chop") && argCount(n) > 0 &&
				isArgsElem(flatten(argList(n))[0]) {
				raise(n, "`"+n.Name+"` on `"+l.snippet(flatten(argList(n))[0])+"`")
			}
		case *ast.RefGen:
			if isArgsVar(n.X) {
				raise(n, "everything reached through `\\@_`")
			}
		}
	})
}
