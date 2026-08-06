package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file converts DESTROY into explicit destruction.
//
// Perl runs DESTROY at a precise instant: the moment the last reference to
// the object goes away, at a closing brace, at an undef, or during the
// unwinding of a die. Go's collector gives no such promise, and a finalizer
// would be both unordered and unreliable, so the generated code does what a
// Go developer does by hand: the destructor becomes an ordinary method, and
// the converter calls it at the point where the object's life provably ends.
//
// Only a provable life is converted. The object must be born in a `my`
// scalar from a constructor call and never used for anything except method
// calls and an undef; anything else, storing it, returning it, passing it,
// even interpolating it, means its lifetime is the reference count's
// business, which this code does not model. Those sites keep the object and
// lose the automatic call, and the report says so at each one.

// Where the destructor call goes for one qualifying declaration.
const (
	// destroyNone: the object escapes, so no call is emitted and the site
	// is reported instead.
	destroyNone = iota
	// destroyDefer: the declaration sits in a function-shaped body, a sub
	// or an eval block, so a defer runs the destructor on every way out,
	// unwinding included.
	destroyDefer
	// destroyBlockEnd: the declaration sits in a plain block, an if branch
	// or a loop body, so the call is the block's last statement, which is
	// the closing brace Perl fired it at.
	destroyBlockEnd
	// destroyAtUndef: the block undefs the variable, which is Perl's way of
	// choosing the instant, so the call happens there and nowhere else.
	destroyAtUndef
)

// destroyPlan is the decision for one `my $x = Class->new(...)` whose class
// has a destructor.
type destroyPlan struct {
	v     *ast.Var
	class *Class
	mode  int
}

// destroyFor finds the destructor a class answers to, its own or an
// inherited one.
func destroyFor(c *Class) *Sub {
	for _, a := range c.ancestors() {
		if a.Destroy != nil {
			return a.Destroy
		}
	}
	return nil
}

// hasDestructor reports whether any class in the file has a DESTROY, which
// is what decides whether the scan below runs at all.
func (l *Lowerer) hasDestructor() bool {
	for _, name := range l.classOrd {
		if c := l.classes[name]; c != nil && c.IsType && c.Destroy != nil {
			return true
		}
	}
	return false
}

// collectDestroys walks the program before the emitting pass and decides,
// for every object born into a `my` scalar from a class with a destructor,
// where its destructor call belongs.
func (l *Lowerer) collectDestroys() {
	l.destroyPlans = map[ast.Stmt]*destroyPlan{}
	l.destroyBound = map[*Binding]*destroyPlan{}
	if !l.hasDestructor() {
		return
	}
	var top []ast.Stmt
	for _, u := range l.units {
		if _, isSub := u.st.(*ast.SubDecl); isSub {
			continue
		}
		top = append(top, u.st)
	}
	l.scanDestroyList(top, true)
	for _, u := range l.units {
		if sd, ok := u.st.(*ast.SubDecl); ok {
			l.scanDestroyList(sd.Body, true)
		}
	}
}

// scanDestroyList looks for qualifying declarations in one statement list
// and recurses into the blocks below it. funcBody says whether leaving this
// list is leaving a Go function, which is what makes defer the right tool.
func (l *Lowerer) scanDestroyList(list []ast.Stmt, funcBody bool) {
	for i, st := range list {
		if pl := l.destroyCandidate(st); pl != nil {
			rest := list[i+1:]
			switch {
			case destroyEscapes(pl.v.Name, rest):
				pl.mode = destroyNone
			case hasUndefOf(pl.v.Name, rest):
				pl.mode = destroyAtUndef
			case funcBody:
				pl.mode = destroyDefer
			default:
				pl.mode = destroyBlockEnd
			}
			l.destroyPlans[st] = pl
		}
		l.scanDestroyStmt(st)
	}
}

// scanDestroyStmt recurses into the statement's nested blocks.
func (l *Lowerer) scanDestroyStmt(st ast.Stmt) {
	switch n := st.(type) {
	case *ast.Block:
		l.scanDestroyList(n.Body, false)
	case *ast.If:
		l.scanDestroyList(n.Then, false)
		for _, ei := range n.ElseIfs {
			l.scanDestroyList(ei.Then, false)
		}
		l.scanDestroyList(n.Else, false)
	case *ast.While:
		l.scanDestroyList(n.Body, false)
	case *ast.ForC:
		l.scanDestroyList(n.Body, false)
	case *ast.Foreach:
		l.scanDestroyList(n.Body, false)
	case *ast.ExprStmt:
		l.scanDestroyExpr(n.X)
	}
}

// scanDestroyExpr finds the function-shaped blocks inside an expression: an
// eval block and an anonymous sub both leave through a Go function's exit.
func (l *Lowerer) scanDestroyExpr(e ast.Expr) {
	switch n := e.(type) {
	case nil:
	case *ast.Call:
		if n.Name == "eval" && n.Block != nil {
			l.scanDestroyList(n.Block, true)
		}
		for _, a := range n.Args {
			l.scanDestroyExpr(a)
		}
	case *ast.AnonSub:
		l.scanDestroyList(n.Body, true)
	case *ast.Assign:
		l.scanDestroyExpr(n.LHS)
		l.scanDestroyExpr(n.RHS)
	case *ast.BinOp:
		l.scanDestroyExpr(n.L)
		l.scanDestroyExpr(n.R)
	case *ast.UnOp:
		l.scanDestroyExpr(n.X)
	case *ast.Ternary:
		l.scanDestroyExpr(n.Cond)
		l.scanDestroyExpr(n.A)
		l.scanDestroyExpr(n.B)
	case *ast.List:
		for _, el := range n.Elems {
			l.scanDestroyExpr(el)
		}
	case *ast.MethodCall:
		l.scanDestroyExpr(n.Invocant)
		for _, a := range n.Args {
			l.scanDestroyExpr(a)
		}
	}
}

// destroyCandidate recognises `my $x = Class->new(...)` where the class has
// a destructor of its own or by inheritance.
func (l *Lowerer) destroyCandidate(st ast.Stmt) *destroyPlan {
	es, ok := st.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	as, ok := es.X.(*ast.Assign)
	if !ok || as.Op != "=" {
		return nil
	}
	my, ok := as.LHS.(*ast.My)
	if !ok || len(my.Vars) != 1 {
		return nil
	}
	v, ok := my.Vars[0].(*ast.Var)
	if !ok || v.Sigil != '$' {
		return nil
	}
	mc, ok := as.RHS.(*ast.MethodCall)
	if !ok || mc.Method == "" {
		return nil
	}
	c := l.classByBareword(mc.Invocant)
	if c == nil || destroyFor(c) == nil {
		return nil
	}
	s := c.method(mc.Method)
	if s == nil || s.Kind != SubCtor {
		return nil
	}
	return &destroyPlan{v: v, class: c}
}

// classByBareword resolves a class named literally before an arrow, without
// touching any scope: the scan runs outside the lowering.
func (l *Lowerer) classByBareword(e ast.Expr) *Class {
	name := ""
	switch n := e.(type) {
	case *ast.FileHandle:
		name = n.Name
	case *ast.Call:
		if len(n.Args) == 0 && !n.Paren {
			name = n.Name
		}
	case *ast.StrLit:
		name = n.Value
	}
	if name == "" {
		return nil
	}
	if c, ok := l.classes[name]; ok && c.IsType {
		return c
	}
	return nil
}

// destroyEscapes reports whether the variable is used, anywhere below the
// declaration, for anything except a method call on it or an undef of it.
// Any other use may extend the object's life beyond the block, and a life
// the converter cannot see the end of gets no automatic call.
func destroyEscapes(name string, rest []ast.Stmt) bool {
	escaped := false
	var walkE func(e ast.Expr)
	var walkS func(list []ast.Stmt)
	use := func(e ast.Expr) {
		if v, ok := e.(*ast.Var); ok && v.Sigil == '$' && v.Name == name {
			escaped = true
		}
	}
	walkE = func(e ast.Expr) {
		if escaped {
			return
		}
		switch n := e.(type) {
		case nil:
		case *ast.Var:
			use(n)
		case *ast.MethodCall:
			// The invocant position is the sanctioned use; a method call
			// neither stores the object nor hands it on.
			if v, ok := n.Invocant.(*ast.Var); !ok || v.Sigil != '$' || v.Name != name {
				walkE(n.Invocant)
			}
			walkE(n.Dynamic)
			for _, a := range n.Args {
				walkE(a)
			}
		case *ast.Call:
			if n.Name == "undef" && len(n.Args) == 1 {
				if v, ok := n.Args[0].(*ast.Var); ok && v.Sigil == '$' && v.Name == name {
					return
				}
			}
			for _, a := range n.Args {
				walkE(a)
			}
			walkS(n.Block)
		case *ast.Assign:
			walkE(n.LHS)
			walkE(n.RHS)
		case *ast.BinOp:
			walkE(n.L)
			walkE(n.R)
		case *ast.UnOp:
			walkE(n.X)
		case *ast.Ternary:
			walkE(n.Cond)
			walkE(n.A)
			walkE(n.B)
		case *ast.List:
			for _, el := range n.Elems {
				walkE(el)
			}
		case *ast.My:
			for _, v := range n.Vars {
				walkE(v)
			}
		case *ast.Index:
			walkE(n.Base)
			walkE(n.Idx)
		case *ast.HashIndex:
			walkE(n.Base)
			walkE(n.Key)
		case *ast.Deref:
			walkE(n.X)
		case *ast.RefGen:
			walkE(n.X)
		case *ast.AnonArray:
			for _, el := range n.Elems {
				walkE(el)
			}
		case *ast.AnonHash:
			for _, el := range n.Elems {
				walkE(el)
			}
		case *ast.AnonSub:
			walkS(n.Body)
		case *ast.FuncCallRef:
			walkE(n.Ref)
			for _, a := range n.Args {
				walkE(a)
			}
		case *ast.InterpLit:
			for _, p := range n.Parts {
				walkE(p)
			}
		case *ast.Match:
			walkE(n.Bound)
		case *ast.Subst:
			walkE(n.Bound)
			walkE(n.Repl)
		case *ast.Trans:
			walkE(n.Bound)
		case *ast.Readline:
			walkE(n.Var)
		case *ast.FileTest:
			walkE(n.Arg)
		}
	}
	walkS = func(list []ast.Stmt) {
		for _, st := range list {
			if escaped {
				return
			}
			switch n := st.(type) {
			case *ast.ExprStmt:
				walkE(n.X)
			case *ast.Block:
				walkS(n.Body)
			case *ast.If:
				walkE(n.Cond)
				walkS(n.Then)
				for _, ei := range n.ElseIfs {
					walkE(ei.Cond)
					walkS(ei.Then)
				}
				walkS(n.Else)
			case *ast.While:
				walkE(n.Cond)
				walkS(n.Body)
			case *ast.ForC:
				walkE(n.Init)
				walkE(n.Cond)
				walkE(n.Post)
				walkS(n.Body)
			case *ast.Foreach:
				for _, e := range n.List {
					walkE(e)
				}
				walkS(n.Body)
			case *ast.Return:
				for _, e := range n.Exprs {
					walkE(e)
				}
			}
		}
	}
	walkS(rest)
	return escaped
}

// hasUndefOf reports whether the statements contain `undef $name` at any
// depth, which is Perl's way of choosing the destruction instant by hand.
func hasUndefOf(name string, rest []ast.Stmt) bool {
	found := false
	var walkS func(list []ast.Stmt)
	var walkE func(e ast.Expr)
	walkE = func(e ast.Expr) {
		if found {
			return
		}
		if n, ok := e.(*ast.Call); ok {
			if n.Name == "undef" && len(n.Args) == 1 {
				if v, ok := n.Args[0].(*ast.Var); ok && v.Sigil == '$' && v.Name == name {
					found = true
					return
				}
			}
			for _, a := range n.Args {
				walkE(a)
			}
			walkS(n.Block)
		}
	}
	walkS = func(list []ast.Stmt) {
		for _, st := range list {
			if found {
				return
			}
			switch n := st.(type) {
			case *ast.ExprStmt:
				walkE(n.X)
			case *ast.Block:
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
			case *ast.Foreach:
				walkS(n.Body)
			}
		}
	}
	walkS(rest)
	return found
}

// destroyCall builds the call that runs one object's destructor.
func (l *Lowerer) destroyCall(b *Binding, pl *destroyPlan) ir.Expr {
	s := destroyFor(pl.class)
	ret := ir.TVoid
	if len(s.Results) > 0 {
		ret = s.Results[0]
	}
	return ir.CallOf(selector(l.identFor(b), s.Go, nil), ret)
}

// applyDestroyPlan emits what one qualifying declaration's plan asks for,
// returning any statements to place directly after the declaration.
func (l *Lowerer) applyDestroyPlan(st ast.Stmt, body []ir.Stmt) []ir.Stmt {
	pl, ok := l.destroyPlans[st]
	if !ok || l.pass != 2 {
		return body
	}
	b, found := l.scope.lookup(varKey('$', pl.v.Name))
	if !found {
		return body
	}
	switch pl.mode {
	case destroyNone:
		l.approximate(st, "P2G7030", "an object with a destructor",
			"the destructor is not called automatically for this object",
			"Perl would run DESTROY when this object's reference count reaches "+
				"zero. The object is used beyond plain method calls here, so where "+
				"its life ends is the reference count's business, and the generated "+
				"code does not model one.",
			"Call "+destroyFor(pl.class).Go+" yourself at the point where the "+
				"object is finished with, or wrap the uses so the object stays "+
				"inside one block.",
			"defer-timing", "pointers-vs-references")
	case destroyDefer:
		l.destroyBound[b] = pl
		d := &ir.Defer{Call: l.destroyCall(b, pl)}
		l.setProv(d, st)
		l.note(d, "Perl runs the destructor the instant the last reference goes "+
			"away, which for this object is on the way out of this function, "+
			"however it leaves: a fall through the bottom, a return, or a die "+
			"unwinding past it. defer is Go's word for exactly that promise, and "+
			"writing it next to the construction is the idiom, so acquisition and "+
			"release read as one thought.",
			"defer-timing", "panic-and-recover")
		body = append(body, d)
	case destroyBlockEnd:
		l.destroyBound[b] = pl
		l.scope.destroys = append(l.scope.destroys, scopeDestroy{b: b, pl: pl})
	case destroyAtUndef:
		l.destroyBound[b] = pl
	}
	return body
}

// flushDestroys emits the destructor calls a block's scope collected, last
// declared first, which is the order Perl releases a block's lexicals in.
func (l *Lowerer) flushDestroys(sc *scope) []ir.Stmt {
	var out []ir.Stmt
	for i := len(sc.destroys) - 1; i >= 0; i-- {
		d := sc.destroys[i]
		st := exprStmt(l.destroyCall(d.b, d.pl))
		l.note(st, "The closing brace below is where Perl ran this object's "+
			"destructor: the variable holding it goes out of scope and the "+
			"reference count hits zero. Go frees memory when it chooses and "+
			"promises nothing about when, so the call is written out at the same "+
			"brace, where a reader can see it.",
			"defer-timing", "statements-vs-expressions")
		out = append(out, st)
	}
	sc.destroys = nil
	return out
}

// scopeDestroy is one pending end-of-block destructor call.
type scopeDestroy struct {
	b  *Binding
	pl *destroyPlan
}
