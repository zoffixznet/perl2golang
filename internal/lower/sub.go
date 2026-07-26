package lower

import (
	"strings"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// hoistSubs registers every subroutine before anything is lowered.
//
// Perl resolves a call to a sub declared later in the file, and so does Go for
// package-level functions, so the whole file is scanned first. It also means
// the type of a call is known before the call site is reached.
func (l *Lowerer) hoistSubs(list []ast.Stmt) {
	for _, st := range list {
		sd, ok := st.(*ast.SubDecl)
		if !ok {
			continue
		}
		if _, dup := l.subs[sd.Name]; dup {
			continue
		}
		s := &Sub{Name: sd.Name, Decl: sd, Line: posLine(sd)}
		s.Go = l.names.take(goName(sd.Name))
		s.Doc = leadComments(sd)
		l.subs[sd.Name] = s
		l.subOrd = append(l.subOrd, sd.Name)
	}
}

// lowerSubDecl builds the Go function for one subroutine.
func (l *Lowerer) lowerSubDecl(sd *ast.SubDecl) {
	s := l.subs[sd.Name]
	if s == nil {
		return
	}

	savedScope, savedSub := l.scope, l.curSub
	l.scope = newScope(nil)
	l.scope.fn = s
	l.curSub = s
	defer func() { l.scope, l.curSub = savedScope, savedSub }()

	body := sd.Body
	params, rest := l.recoverParams(s, body)

	fn := &ir.FuncDecl{Name: s.Go, Params: params}
	if l.pass == 2 {
		fn.Results = s.Results
		fn.Doc = l.subDoc(s)
	}
	fn.Body = &ir.Block{Stmts: l.stmts(rest)}
	l.addImplicitReturn(s, fn)
	l.setProv(fn, sd)
	l.explainSub(fn, s, sd)
	s.irDecl = fn
}

// subDoc builds the Go doc comment for a generated function, preferring the
// developer's own comment above the sub.
func (l *Lowerer) subDoc(s *Sub) []string {
	if len(s.Doc) > 0 {
		out := make([]string, 0, len(s.Doc)+1)
		// A Go doc comment starts with the name of the thing it documents. When
		// the developer's first line begins with an article the two fit
		// together; otherwise their words are left exactly as written, with the
		// naming line added above them.
		first := s.Doc[0]
		if rest, ok := cutArticle(first); ok {
			out = append(out, s.Go+" is "+rest)
			out = append(out, s.Doc[1:]...)
			return out
		}
		out = append(out, s.Go+" is defined as follows.")
		out = append(out, s.Doc...)
		return out
	}
	return []string{s.Go + " performs one step of the program's work."}
}

// cutArticle reports whether a sentence starts with an article, and returns it
// with the article kept so that "A helper" reads as "name is a helper".
func cutArticle(s string) (string, bool) {
	for _, article := range []string{"A ", "An ", "The "} {
		if strings.HasPrefix(s, article) {
			return strings.ToLower(s[:1]) + s[1:], true
		}
	}
	return s, false
}

// explainSub attaches the lesson a function declaration deserves.
func (l *Lowerer) explainSub(fn *ir.FuncDecl, s *Sub, sd *ast.SubDecl) {
	if l.pass != 2 {
		return
	}
	switch {
	case s.Variadic:
		l.note(fn, "Perl subs take a flat list in @_ and pull their arguments out of "+
			"it by hand. This one does not unpack into a fixed set of names, so it "+
			"becomes a variadic Go function: args is a slice inside the body, and "+
			"callers still pass values one by one.",
			"variadic-and-no-defaults")
	case len(s.Params) > 0:
		l.note(fn, "The `my (...) = @_` at the top of the sub is Perl's way of naming "+
			"its arguments. Go declares them in the signature with types, so the "+
			"compiler checks the number and the kinds at every call site rather than "+
			"leaving a short call to produce undef.",
			"variadic-and-no-defaults", "static-types-and-zero-values")
	}
	if len(s.Results) > 1 {
		l.note(fn, "Perl returns a list and the caller decides what to do with it. Go "+
			"declares exactly how many values come back, and the caller must accept "+
			"all of them or discard the extras with _.",
			"multiple-return-values")
	}
	if len(s.Results) == 0 {
		l.note(fn, "Nothing in this sub returns a value that a caller uses, so the Go "+
			"function returns nothing. Perl would have returned the last expression "+
			"evaluated whether or not anyone wanted it.")
	}
}

// recoverParams turns the `my (...) = @_` idiom into a Go parameter list and
// returns the statements that remain.
func (l *Lowerer) recoverParams(s *Sub, body []ast.Stmt) ([]ir.Param, []ast.Stmt) {
	if l.pass == 2 && s.irDecl != nil {
		// Re-entering with the shape already settled: rebuild the scope from
		// the recorded bindings.
	}
	rest := body
	var bindings []*Binding

	for len(rest) > 0 {
		es, ok := rest[0].(*ast.ExprStmt)
		if !ok {
			break
		}
		as, ok := es.X.(*ast.Assign)
		if !ok || as.Op != "=" {
			break
		}
		my, ok := as.LHS.(*ast.My)
		if !ok || my.Keyword != "my" {
			break
		}

		// my ($a, $b) = @_;
		if isArgsVar(as.RHS) {
			vars := declaredVars(my)
			allScalar := true
			for _, v := range vars {
				if v.Sigil != '$' {
					allScalar = false
				}
			}
			if allScalar {
				for _, v := range vars {
					bindings = append(bindings, l.declare(v, KindParam))
				}
				rest = rest[1:]
				continue
			}
			if len(vars) == 1 && vars[0].Sigil == '@' {
				b := l.declare(vars[0], KindParam)
				s.Variadic = true
				s.VarArgs = b
				rest = rest[1:]
				break
			}
			break
		}

		// my $x = shift;
		if isShiftArgs(as.RHS) {
			vars := declaredVars(my)
			if len(vars) == 1 && vars[0].Sigil == '$' {
				bindings = append(bindings, l.declare(vars[0], KindParam))
				rest = rest[1:]
				continue
			}
		}
		break
	}

	if len(bindings) == 0 && s.VarArgs == nil && usesArgs(body) {
		s.Variadic = true
		s.VarArgs = l.declareNamed("args@"+s.Name, '@', "args", KindParam, s.Decl)
	}

	s.Params = bindings
	var params []ir.Param
	for _, b := range bindings {
		params = append(params, ir.Param{Name: b.Go, Type: b.Type})
	}
	if s.VarArgs != nil {
		elem := elemOf(s.VarArgs.Type)
		params = append(params, ir.Param{Name: s.VarArgs.Go, Type: elem, Variadic: true})
	}
	return params, rest
}

// addImplicitReturn makes Perl's "the last expression is the return value"
// explicit, which Go requires.
func (l *Lowerer) addImplicitReturn(s *Sub, fn *ir.FuncDecl) {
	if len(fn.Body.Stmts) == 0 {
		return
	}
	last := fn.Body.Stmts[len(fn.Body.Stmts)-1]
	es, ok := last.(*ir.ExprStmt)
	if !ok {
		return
	}
	if l.pass == 1 {
		s.ResultEvidence = append(s.ResultEvidence, []*ir.Type{typeOrAny(es.X)})
		return
	}
	if len(s.Results) == 0 {
		return
	}
	ret := &ir.Return{Results: []ir.Expr{l.assignable(es.X, s.Results[0], nil)}}
	ret.Meta = es.Meta
	l.note(ret, "A Perl sub with no return statement yields the value of the last "+
		"expression it evaluated. Go requires the return to be written, which is "+
		"one fewer thing to remember when reading the code.")
	fn.Body.Stmts[len(fn.Body.Stmts)-1] = ret
}

// isArgsVar reports whether an expression is @_.
func isArgsVar(e ast.Expr) bool {
	v, ok := e.(*ast.Var)
	return ok && v.Sigil == '@' && v.Name == "_"
}

// isShiftArgs reports whether an expression is a bare `shift`, which inside a
// sub means `shift @_`.
func isShiftArgs(e ast.Expr) bool {
	c, ok := e.(*ast.Call)
	if !ok || c.Name != "shift" {
		return false
	}
	if len(c.Args) == 0 {
		return true
	}
	return len(c.Args) == 1 && isArgsVar(c.Args[0])
}

// usesArgs reports whether a sub body mentions @_ anywhere.
func usesArgs(body []ast.Stmt) bool {
	found := false
	var walkE func(ast.Expr)
	var walkS func([]ast.Stmt)
	walkE = func(e ast.Expr) {
		switch n := e.(type) {
		case nil:
			return
		case *ast.Var:
			if n.Sigil == '@' && n.Name == "_" {
				found = true
			}
		case *ast.Index:
			walkE(n.Base)
			walkE(n.Idx)
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
		case *ast.Call:
			if n.Name == "shift" && len(n.Args) == 0 {
				found = true
			}
			for _, a := range n.Args {
				walkE(a)
			}
			walkS(n.Block)
		case *ast.My:
			for _, v := range n.Vars {
				walkE(v)
			}
		case *ast.InterpLit:
			for _, p := range n.Parts {
				walkE(p)
			}
		}
	}
	walkS = func(list []ast.Stmt) {
		for _, st := range list {
			switch n := st.(type) {
			case *ast.ExprStmt:
				walkE(n.X)
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
			case *ast.Block:
				walkS(n.Body)
			case *ast.Return:
				for _, e := range n.Exprs {
					walkE(e)
				}
			}
		}
	}
	walkS(body)
	return found
}

// settleSubs decides each sub's Go signature from the returns seen on pass 1.
func (l *Lowerer) settleSubs() {
	for _, name := range l.subOrd {
		s := l.subs[name]
		shapes := s.ResultEvidence
		if len(shapes) == 0 {
			s.Results = nil
			continue
		}
		width := 0
		for _, sh := range shapes {
			if len(sh) > width {
				width = len(sh)
			}
		}
		if width == 0 {
			s.Results = nil
			continue
		}
		results := make([]*ir.Type, width)
		for _, sh := range shapes {
			for i, t := range sh {
				results[i] = join(results[i], t)
			}
		}
		for i := range results {
			if results[i] == nil {
				results[i] = ir.TAny
			}
		}
		s.Results = results
	}
}

// multiResultCall handles a call to a sub that returns several values.
//
// Go allows a multi-valued call only where all of its results are consumed at
// once, so anywhere Perl would have taken the list apart, the call is hoisted
// into its own statement first. In list context the results become the list;
// in scalar context Perl yields the last value of the returned list, and so
// does this.
func (l *Lowerer) multiResultCall(c *ast.Call, wantList bool) (ir.Expr, bool) {
	s, ok := l.subs[c.Name]
	if !ok || len(s.Results) < 2 || isBuiltinName(c.Name) {
		return nil, false
	}
	value := l.callSub(s, c)

	names := make([]ir.Expr, len(s.Results))
	for i := range s.Results {
		names[i] = ir.NewIdent(l.tmp("r"), s.Results[i])
	}
	st := assign(":=", names, []ir.Expr{value})
	l.setProv(st, c)
	l.note(st, "A Go function that returns several values can only be called where "+
		"all of them are taken at once, so the call gets its own statement and the "+
		"results are named. Perl would have flattened them into the surrounding "+
		"expression.",
		"multiple-return-values")
	l.emit(st)

	if wantList {
		t := joinAll(s.Results)
		return composite(ir.SliceOf(t), nil, names), true
	}
	return names[len(names)-1], true
}

// callSub lowers a call to a user-declared subroutine.
func (l *Lowerer) callSub(s *Sub, n *ast.Call) ir.Expr {
	s.CallSites++
	args, _ := l.listParts(n.Args)

	// Fixed parameters take their values in order; anything left over goes to
	// the variadic tail.
	var out []ir.Expr
	for i, a := range args {
		if i < len(s.Params) {
			p := s.Params[i]
			l.observe(p, typeOrAny(a))
			out = append(out, l.assignable(a, p.Type, nil))
			continue
		}
		if s.VarArgs != nil {
			l.observeElem(s.VarArgs, typeOrAny(a))
			out = append(out, l.assignable(a, elemOf(s.VarArgs.Type), nil))
			continue
		}
		out = append(out, a)
	}
	// A call that passes fewer arguments than the sub unpacks would leave
	// undef in Perl; Go will not compile it, so the gap is filled explicitly.
	for i := len(out); i < len(s.Params); i++ {
		out = append(out, zeroOf(s.Params[i].Type))
		if l.pass == 2 {
			l.approximate(n, "P2G2130", "call with missing arguments",
				"a missing argument becomes a zero value",
				"This call passes fewer arguments than the sub unpacks, so Perl would "+
					"have left the rest undef. Go requires every parameter, so the missing "+
					"ones are passed as their type's zero value.",
				"Give the parameter an explicit default inside the function, or split "+
					"the function in two, which is what Go code does instead of optional "+
					"arguments.",
				"variadic-and-no-defaults")
		}
	}

	ret := ir.TVoid
	if len(s.Results) > 0 {
		ret = s.Results[0]
	}
	c := ir.CallOf(ir.NewIdent(s.Go, nil), ret, out...)

	// Passing a slice where the sub takes a variadic tail needs the ... form.
	if s.VarArgs != nil && len(args) == len(s.Params)+1 {
		if last := args[len(args)-1]; typeOrAny(last).Kind == ir.Slice {
			c.Ellipsis = true
			l.note(c, "Perl flattens an array into the argument list automatically. Go "+
				"needs the ... suffix to say that a slice is being spread rather than "+
				"passed as one value.",
				"variadic-and-no-defaults")
		}
	}
	return c
}
