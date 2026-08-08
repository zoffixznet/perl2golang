package lower

import (
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// hoistSubs registers every subroutine before anything is lowered.
//
// Perl resolves a call to a sub declared later in the file, and so does Go for
// package-level functions, so the whole file is scanned first. It also means
// the type of a call is known before the call site is reached.
//
// Subs are keyed by their fully qualified Perl name, because a file with two
// packages in it can hold two subs called `new` and they are different
// functions.
func (l *Lowerer) hoistSubs() {
	for _, u := range l.units {
		for _, sd := range namedSubs(u.st) {
			key := qualify(u.pkg, sd.Name)
			if _, dup := l.subs[key]; dup {
				continue
			}
			s := &Sub{Name: sd.Name, Pkg: u.pkg, Decl: sd, Line: posLine(sd), File: u.file}
			s.Doc = leadComments(sd)
			l.subs[key] = s
			l.subOrd = append(l.subOrd, key)
			if c, ok := l.classes[u.pkg]; ok && c.IsType {
				s.Class = c
				c.Subs = append(c.Subs, s)
				c.subBy[sd.Name] = s
				continue
			}
			s.Go = l.plainName(u.pkg, sd.Name)
		}
	}
	// Naming the members of a class needs the whole class in hand: a method
	// and a field share one namespace in Go, and the constructor's name is
	// built from the type's.
	for _, name := range l.classOrd {
		if c := l.classes[name]; c.IsType {
			l.nameMembers(c)
		}
	}
	// A class with no constructor of its own gets one that fills in the
	// parent's, which needs every parent to have been named first.
	for _, name := range l.classOrd {
		if c := l.classes[name]; c.IsType {
			l.inheritCtor(c)
		}
	}
}

// namedSubs finds the named sub declarations in one statement, including any
// nested in blocks, branches and loops. A named sub is package-global in Perl
// however deeply its declaration is buried; only its body is affected by
// where it sits, `use integer` in the enclosing block being the usual reason
// to bury one.
func namedSubs(st ast.Stmt) []*ast.SubDecl {
	var out []*ast.SubDecl
	var walk func(list []ast.Stmt)
	walk = func(list []ast.Stmt) {
		for _, s := range list {
			switch n := s.(type) {
			case *ast.SubDecl:
				out = append(out, n)
			case *ast.Block:
				walk(n.Body)
			case *ast.If:
				walk(n.Then)
				for _, ei := range n.ElseIfs {
					walk(ei.Then)
				}
				walk(n.Else)
			case *ast.While:
				walk(n.Body)
			case *ast.ForC:
				walk(n.Body)
			case *ast.Foreach:
				walk(n.Body)
			}
		}
	}
	walk([]ast.Stmt{st})
	return out
}

// qualify returns the fully qualified Perl name of a sub.
func qualify(pkg, name string) string {
	if strings.Contains(name, "::") {
		return name
	}
	if pkg == "" {
		pkg = "main"
	}
	return pkg + "::" + name
}

// plainName picks the Go identifier for a sub that is not a method. A sub in a
// second package keeps its own name where nothing else has claimed it, and is
// qualified with the package only when it would otherwise collide.
func (l *Lowerer) plainName(pkg, name string) string {
	base := goName(name)
	if pkg != "main" && l.names.has(base) {
		base = goName(lastPart(pkg) + "_" + name)
	}
	return l.names.take(base)
}

// lastPart returns the final component of a :: separated package name.
func lastPart(pkg string) string {
	if i := strings.LastIndex(pkg, "::"); i >= 0 {
		return pkg[i+2:]
	}
	return pkg
}

// findSub resolves a call by name from inside the package being lowered, the
// way Perl does: an unqualified name means this package, falling back to main
// for a script that declares its helpers before its classes.
func (l *Lowerer) findSub(name string) (*Sub, bool) {
	if strings.Contains(name, "::") {
		s, ok := l.subs[name]
		return s, ok
	}
	if s, ok := l.subs[qualify(l.curPkg, name)]; ok {
		return s, true
	}
	if s, ok := l.subs["main::"+name]; ok {
		return s, true
	}
	// A module exports names into whoever used it, and every package in the
	// conversion lands in one Go package, so a name only one of them declares
	// resolves to that one. Two packages declaring it is a real ambiguity and
	// is left alone.
	var found *Sub
	for _, key := range l.subOrd {
		s := l.subs[key]
		if s.Name != name || s.Class != nil {
			continue
		}
		if found != nil {
			return nil, false
		}
		found = s
	}
	return found, found != nil
}

// lowerSubDecl builds the Go function for one subroutine.
func (l *Lowerer) lowerSubDecl(sd *ast.SubDecl) {
	s := l.subs[qualify(l.curPkg, sd.Name)]
	if s == nil {
		return
	}

	savedScope, savedSub, savedClass, savedFile := l.scope, l.curSub, l.curClass, l.curFile
	// A sub body is a lexical scope like any other, so `use integer` inside
	// one stops at the closing brace and does not reach the code after it.
	// The separator variables are the same shape: `local $/` inside a sub
	// governs the reads in that sub, not whatever the file lowers next.
	savedInteger := l.integerPragma
	savedSeps := l.seps
	l.scope = newScope(nil)
	l.scope.fn = s
	l.curSub = s
	l.curClass = s.Class
	l.setFile(s.File)
	defer func() {
		l.scope, l.curSub, l.curClass = savedScope, savedSub, savedClass
		l.integerPragma = savedInteger
		l.seps = savedSeps
		l.setFile(savedFile)
	}()

	if s.Class != nil {
		l.lowerMethodDecl(s, sd)
		return
	}

	if s.Comparator {
		l.lowerComparatorSub(s, sd)
		return
	}

	body := valueTail(sd.Body)
	params, rest := l.recoverParams(s, body)

	fn := &ir.FuncDecl{Name: s.Go, Params: params}
	if l.pass == 2 {
		fn.Results = s.Results
		fn.Doc = l.subDoc(s)
	}
	fn.Body = l.markUnused(&ir.Block{Stmts: l.stmts(rest)})
	l.addImplicitReturn(s, fn, rest)
	l.ensureReturn(s, fn.Body)
	l.setProv(fn, sd)
	l.explainSub(fn, s, sd)
	s.irDecl = fn
}

// lowerComparatorSub builds the Go function for a sub used as `sort byname
// @list`.
//
// Perl hands the comparator its two values in the package globals $a and $b,
// which is why such a sub takes no arguments and cannot be called directly. Go
// passes them as parameters, so the whole signature changes: two values in, an
// int out.
func (l *Lowerer) lowerComparatorSub(s *Sub, sd *ast.SubDecl) {
	elem := s.CmpElem
	if elem == nil {
		elem = ir.TAny
	}
	ba := l.declareNamed("cmpA@"+s.Name, '$', "a", KindParam, sd)
	bb := l.declareNamed("cmpB@"+s.Name, '$', "b", KindParam, sd)
	ba.Perl, bb.Perl = "$a", "$b"
	ba.Type, bb.Type = elem, elem
	l.scope.define(varKey('$', "a"), ba)
	l.scope.define(varKey('$', "b"), bb)

	s.Results = []*ir.Type{ir.TInt}
	fn := &ir.FuncDecl{
		Name:   s.Go,
		Params: []ir.Param{{Name: ba.Go, Type: elem}, {Name: bb.Go, Type: elem}},
	}
	if l.pass == 2 {
		fn.Results = s.Results
		fn.Doc = []string{s.Go + " orders two values, returning a negative number, zero, " +
			"or a positive one."}
	}
	stmts, value := l.blockValue(sd.Body)
	if value != nil {
		if typeOrAny(value).Kind != ir.Int {
			value = l.toInt(value, nil)
		}
		stmts = append(stmts, &ir.Return{Results: []ir.Expr{value}})
	}
	fn.Body = l.markUnused(&ir.Block{Stmts: stmts})
	l.setProv(fn, sd)
	l.note(fn, "Perl passes a comparator its two values in the package globals $a "+
		"and $b, so the sub takes no arguments and only works when sort calls it. Go "+
		"passes them as parameters, which makes it an ordinary function that anything "+
		"can call and the compiler can check.",
		"sort-slice")
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
	l.reportArgAliasing(body)
	rest := body
	var bindings []*Binding

	// A closure sharing a slot with others has to carry the signature they
	// all agree on. When the group has settled on a fixed parameter list,
	// the member names its own parameters below and the missing positions
	// are padded; until then, and whenever a call passes a whole list or a
	// body reads @_ raw, the shared signature is a variadic slice, typed by
	// what the call sites pass through it.
	group := s.group()
	fixedGroup := group != nil && group.decided && group.fixed
	if fixedGroup && s.VarArgs != nil {
		// An earlier round shaped this member as a variadic slice before the
		// group had settled on a fixed list. The slice parameter it declared
		// then would now be a second tail after the named parameters, so it
		// is withdrawn along with its binding.
		delete(l.decls, keyNode{"args@" + s.Name})
		s.VarArgs = nil
		s.Variadic = false
	}
	if l.uniformFn && !fixedGroup {
		var at ast.Node
		if s.Decl != nil {
			at = s.Decl
		}
		s.Variadic = true
		if s.VarArgs == nil {
			s.VarArgs = l.declareNamed("args@"+s.Name, '@', "args", KindParam, at)
		}
		elem := ir.TAny
		s.Results = []*ir.Type{ir.TAny}
		if group != nil && group.decided {
			if group.elem != nil && !isUnresolved(group.elem) {
				elem = group.elem
			}
			switch {
			case !group.returns:
				s.Results = nil
			case len(group.unified) > 0:
				s.Results = append([]*ir.Type(nil), group.unified...)
			case group.result != nil && !isUnresolved(group.result):
				s.Results = []*ir.Type{group.result}
			}
		}
		s.VarArgs.Type = ir.SliceOf(elem)
		return []ir.Param{{Name: s.VarArgs.Go, Type: elem, Variadic: true}}, rest
	}

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

	if fixedGroup {
		// A member whose body reads `$_[k]` by number is positional: each
		// position it reads becomes a parameter of its own, so the fixed
		// signature can carry it and the reads resolve to names.
		if arity, ok := s.naturalShape(); ok {
			var at ast.Node
			if s.Decl != nil {
				at = s.Decl
			}
			for i := len(bindings); i < arity; i++ {
				bindings = append(bindings,
					l.declareNamed("pos"+itoa(i)+"@"+s.Name, '$', "arg", KindParam, at))
			}
		}
	} else if len(bindings) == 0 && s.VarArgs == nil && usesArgs(body) {
		// An anonymous sub has no declaration node to point at, so the
		// position comes out as unknown rather than as a nil dereference.
		var at ast.Node
		if s.Decl != nil {
			at = s.Decl
		}
		s.Variadic = true
		s.VarArgs = l.declareNamed("args@"+s.Name, '@', "args", KindParam, at)
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
	// A member of a fixed-signature group takes the group's full width,
	// whatever its own body reads. The positions it never looks at are
	// blank, which is Go's way of writing "this argument arrives and
	// nothing here wants it".
	if fixedGroup && s.VarArgs == nil {
		for i := len(params); i < group.arity; i++ {
			params = append(params, ir.Param{Name: "_", Type: group.settledParam(i)})
		}
	}
	return params, rest
}

// addImplicitReturn makes Perl's "the last expression is the return value"
// explicit, which Go requires.
func (l *Lowerer) addImplicitReturn(s *Sub, fn *ir.FuncDecl, body []ast.Stmt) {
	l.implicitReturn(s, fn.Body, body)
}

// implicitReturn is addImplicitReturn over a block, so that a function literal
// gets the same treatment a declared function does.
func (l *Lowerer) implicitReturn(s *Sub, block *ir.Block, body []ast.Stmt) {
	if block == nil || len(block.Stmts) == 0 {
		return
	}
	last := block.Stmts[len(block.Stmts)-1]
	if es, ok := last.(*ir.ExprStmt); ok {
		// A sub whose last statement throws or exits produces no value at
		// all, and returning the result of a call that never returns is not
		// Go.
		if terminatingCall(es.X) {
			return
		}
		// Nor does a statement that produces nothing, or one whose Go form
		// hands back several values, such as a print.
		if t := es.X.Type(); t == nil || t.Kind == ir.Void || t.Kind == ir.Invalid {
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
		block.Stmts[len(block.Stmts)-1] = ret
		return
	}
	if _, isRet := last.(*ir.Return); isRet {
		return
	}
	// A sub ending in an assignment yields a value too: `$memo{$t} //= ...`
	// hands back what $memo{$t} holds once the statement has run. The
	// assignment lowers to statements rather than to one expression, an if
	// around a store for //=, a plain store otherwise, so the value is read
	// back out of the place after the last of them.
	target := assignedPlace(body)
	if target == nil {
		return
	}
	x := l.expr(target)
	if x == nil {
		return
	}
	if t := x.Type(); t == nil || t.Kind == ir.Void || t.Kind == ir.Invalid {
		return
	}
	if l.pass == 1 {
		s.ResultEvidence = append(s.ResultEvidence, []*ir.Type{typeOrAny(x)})
		return
	}
	if len(s.Results) == 0 {
		return
	}
	ret := &ir.Return{Results: []ir.Expr{l.assignable(x, s.Results[0], nil)}}
	l.note(ret, "A Perl sub with no return statement yields its last expression, "+
		"and an assignment is an expression there: its value is what was stored. "+
		"The Go form stores first and then reads the place back, which says the "+
		"same thing in statements.")
	block.Stmts = append(block.Stmts, ret)
}

// assignedPlace reports the scalar place a body's final statement assigns to,
// when there is one to read a return value back out of.
func assignedPlace(body []ast.Stmt) ast.Expr {
	if len(body) == 0 {
		return nil
	}
	es, ok := body[len(body)-1].(*ast.ExprStmt)
	if !ok {
		return nil
	}
	as, ok := es.X.(*ast.Assign)
	if !ok {
		return nil
	}
	lhs := as.LHS
	if my, isMy := lhs.(*ast.My); isMy && len(my.Vars) == 1 {
		lhs = my.Vars[0]
	}
	switch n := lhs.(type) {
	case *ast.Var:
		if n.Sigil == '$' {
			return n
		}
	case *ast.Index, *ast.HashIndex:
		return lhs
	case *ast.Deref:
		if n.Sigil == '$' {
			return lhs
		}
	}
	return nil
}

// ensureReturn appends a return of zero values when a function that promises
// results can fall off its end.
//
// Perl functions end wherever the code stops and yield whatever was last
// evaluated. Go requires the compiler to be able to see that the end is
// unreachable, and it is stricter about that than a reader would be, so the
// return is written even where nothing can reach it.
func (l *Lowerer) ensureReturn(s *Sub, block *ir.Block) {
	if l.pass != 2 || block == nil || len(s.Results) == 0 {
		return
	}
	if n := len(block.Stmts); n > 0 {
		switch last := block.Stmts[n-1].(type) {
		case *ir.Return:
			return
		case *ir.ExprStmt:
			// Go's rule for a terminating statement names panic and not
			// os.Exit, so a function ending in an exit still needs a return
			// written even though it can never be reached.
			if c, ok := last.X.(*ir.Call); ok {
				if id, isID := c.Fun.(*ir.Ident); isID && id.Name == "panic" {
					return
				}
			}
		}
	}
	results := make([]ir.Expr, len(s.Results))
	for i, t := range s.Results {
		results[i] = zeroOf(t)
	}
	ret := &ir.Return{Results: results}
	l.note(ret, "Go will not compile a function that promises a result and can "+
		"reach its closing brace without returning one. Perl simply yields whatever "+
		"the last statement produced, so the zero values are written here to say "+
		"what that turns into.",
		"static-types-and-zero-values")
	block.Stmts = append(block.Stmts, ret)
}

// terminatingCall reports whether an expression is a call that never comes
// back: panic, or os.Exit.
func terminatingCall(x ir.Expr) bool {
	c, ok := x.(*ir.Call)
	if !ok {
		return false
	}
	switch fn := c.Fun.(type) {
	case *ir.Ident:
		return fn.Name == "panic"
	case *ir.Selector:
		id, ok := fn.X.(*ir.Ident)
		return ok && id.Name == "os" && fn.Sel == "Exit"
	}
	return false
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
			// $_[0] reads the argument list as surely as @_ does, and it is
			// how a one-line sub gets at its arguments without naming them.
			if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' && v.Name == "_" {
				found = true
			}
			walkE(n.Base)
			walkE(n.Idx)
		case *ast.Slice:
			// @_[0, 1] slices the argument list the same way.
			walkE(n.Base)
			for _, i := range n.Idx {
				walkE(i)
			}
		case *ast.RefGen:
			// \@_ carries the whole argument list out as a reference.
			walkE(n.X)
		case *ast.Deref:
			walkE(n.X)
		case *ast.HashIndex:
			// $h{ $_[0] } reads the argument list from inside a key.
			walkE(n.Base)
			walkE(n.Key)
		case *ast.Match:
			walkE(n.Bound)
			walkE(n.PatternExpr)
		case *ast.Subst:
			walkE(n.Bound)
			walkE(n.Repl)
		case *ast.Trans:
			walkE(n.Bound)
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
		case *ast.FuncCallRef:
			walkE(n.Ref)
			for _, a := range n.Args {
				walkE(a)
			}
		case *ast.MethodCall:
			walkE(n.Invocant)
			walkE(n.Dynamic)
			for _, a := range n.Args {
				walkE(a)
			}
		case *ast.AnonArray:
			for _, el := range n.Elems {
				walkE(el)
			}
		case *ast.AnonHash:
			for _, el := range n.Elems {
				walkE(el)
			}
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
	all := make([]*Sub, 0, len(l.subOrd)+len(l.anonOrd))
	for _, name := range l.subOrd {
		all = append(all, l.subs[name])
	}
	all = append(all, l.anonOrd...)
	defer func() {
		// A function literal's type was built before its results were known,
		// so it is refreshed here, in place: the containers that hold it kept
		// a reference to this very type and are settled after this runs.
		for _, s := range all {
			fillResults(s)
			if s.LitType == nil {
				continue
			}
			s.LitType.Results = s.Results
			for i, p := range s.Params {
				if i < len(s.LitType.Params) {
					s.LitType.Params[i] = p.Type
				}
			}
		}
	}()
	for _, s := range all {
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
			// A position some return leaves undef needs room for the
			// absence, the same rule a container with undef in it follows.
			if s.NilResults[i] && isScalarKind(results[i]) {
				results[i] = nullable(results[i])
			}
		}
		s.Results = results
	}
	// Members of a shared slot answer with one type between them, decided
	// here so the literal types refreshed on the way out carry it.
	l.unifyGroupResults()
}

// multiResultCall handles a call to a sub that returns several values.
//
// Go allows a multi-valued call only where all of its results are consumed at
// once, so anywhere Perl would have taken the list apart, the call is hoisted
// into its own statement first. In list context the results become the list;
// in scalar context Perl yields the last value of the returned list, and so
// does this.
func (l *Lowerer) multiResultCall(c *ast.Call, wantList bool) (ir.Expr, bool) {
	s, ok := l.findSub(c.Name)
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
		last := len(names) - 1
		if s.TailSpill && typeOrAny(names[last]).Kind == ir.Slice {
			// The final result carries a whole list, so it contributes its
			// elements to this one rather than itself.
			var seen []*ir.Type
			for i, name := range names {
				if i == last {
					l.spliced[name] = true
					seen = append(seen, elemOf(typeOrAny(name)))
					continue
				}
				seen = append(seen, typeOrAny(name))
			}
			t := joinAll(sansGroups(seen))
			if t == nil {
				t = ir.TAny
			}
			// listValue rewrites the parts it is given to fit the element
			// type, and names is also the left side of the := above, so it
			// gets a copy.
			parts := make([]ir.Expr, len(names))
			copy(parts, names)
			return l.listValue(parts, t), true
		}
		t := joinAll(sansGroups(s.Results))
		if t == nil {
			t = ir.TAny
		}
		// The results are a list here, and a Go slice holds one type, so each
		// of them is converted into the one type that covers all of them.
		elems := make([]ir.Expr, len(names))
		for i, name := range names {
			elems[i] = l.assignable(name, t, c)
		}
		return composite(ir.SliceOf(t), nil, elems), true
	}
	if s.TailSpill && typeOrAny(names[len(names)-1]).Kind == ir.Slice {
		// One value wanted from a call whose return ends in an array: Perl
		// evaluates the return's last expression in the caller's context, and
		// an array asked for one value answers with its element count.
		out := lenOf(names[len(names)-1])
		l.note(out, "This sub's return ends in an array, and a Perl sub called "+
			"where one value is wanted evaluates its return in that context: the "+
			"array answers with its element count. Go has no context, so the len "+
			"is taken here, at the call.",
			"context-is-gone", "multiple-return-values")
		return out, true
	}
	return names[len(names)-1], true
}

// callSub lowers a call to a user-declared subroutine.
func (l *Lowerer) callSub(s *Sub, n *ast.Call) ir.Expr {
	s.CallSites++
	args, _ := l.listParts(l.callArgs(n))

	// Fixed parameters take their values in order; anything left over goes to
	// the variadic tail.
	var out, tail []ir.Expr
	tailHasList := false
	for i, a := range args {
		if i < len(s.Params) {
			p := s.Params[i]
			l.observe(p, typeOrAny(a))
			out = append(out, l.assignable(a, p.Type, nil))
			continue
		}
		if s.VarArgs != nil {
			// Perl flattens an array into the argument list, so what the tail
			// receives is the array's elements, not the array.
			if at := typeOrAny(a); at.Kind == ir.Slice {
				l.observeElem(s.VarArgs, elemOf(at))
				tail = append(tail, l.assignable(a, ir.SliceOf(elemOf(s.VarArgs.Type)), nil))
				tailHasList = true
				continue
			}
			l.observeElem(s.VarArgs, typeOrAny(a))
			tail = append(tail, l.assignable(a, elemOf(s.VarArgs.Type), nil))
			continue
		}
		// The sub named its parameters and takes no variadic tail, so Perl
		// would have left this argument sitting unread in @_.
		if l.pass == 2 && i == len(s.Params) {
			l.approximate(n, "P2G2130", "call with extra arguments",
				"the extra arguments are dropped",
				"This call passes more values than the sub unpacks. In Perl the extras "+
					"sit in @_ where nothing reads them; a Go call has to match the "+
					"signature exactly.",
				"If the extras were meant to be used, add parameters for them. If they "+
					"were not, remove them from the call.",
				"variadic-and-no-defaults")
		}
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

	// The variadic tail. Perl flattens an array into the argument list, so a
	// call that mixes single values with arrays hands the sub one flat list.
	// Go spreads exactly one slice and will not mix that with other arguments,
	// so the flattening is written out: the list is built first and spread as
	// a whole.
	ellipsis := false
	switch {
	case len(tail) == 0:
	case len(tail) == 1 && tailHasList:
		out = append(out, tail[0])
		ellipsis = true
	case !tailHasList:
		out = append(out, tail...)
	default:
		out = append(out, l.flattenTail(tail, elemOf(s.VarArgs.Type), n))
		ellipsis = true
	}

	// A class method reached by its plain name is being called on the class
	// it was written in.
	if s.ClassParam != nil && s.Class != nil {
		out = append([]ir.Expr{ir.Str(quote(s.Class.Perl))}, out...)
	}

	ret := ir.TVoid
	if len(s.Results) > 0 {
		ret = s.Results[0]
	}
	c := ir.CallOf(ir.NewIdent(s.Go, nil), ret, out...)
	c.Ellipsis = ellipsis
	if len(s.Results) == 0 {
		// Nothing in the file reads this sub's value, so the Go function
		// returns nothing and the call cannot stand in an expression. It runs
		// as its own statement and the expression it was part of gets Perl's
		// answer for a sub that returns nothing.
		if l.pass == 2 {
			l.approximate(n, "P2G2125", "call to a sub that returns nothing",
				"the call runs on its own line and yields nothing",
				"Nothing in the file uses what this sub returns, so the Go function "+
					"returns no value at all and a call to it cannot appear inside a larger "+
					"expression.",
				"If the sub is meant to produce a value, return it explicitly and the "+
					"signature will follow.",
				"multiple-return-values")
		}
		l.emit(exprStmt(c))
		return ir.Nil(ir.TAny)
	}

	if ellipsis {
		l.note(c, "Perl flattens an array into the argument list automatically. Go "+
			"needs the ... suffix to say that a slice is being spread rather than "+
			"passed as one value.",
			"variadic-and-no-defaults")
	}
	return c
}

// flattenTail builds the one slice a variadic call can spread out of a mixture
// of single values and arrays.
//
// Perl does this in the call itself: every array in the argument list is
// flattened into @_ and the sub sees one list. Go spreads a single slice and
// nothing else, so the same list is built on the line above and spread whole.
func (l *Lowerer) flattenTail(parts []ir.Expr, elem *ir.Type, at ast.Node) ir.Expr {
	t := ir.SliceOf(elem)
	name := l.tmp("argList")
	target := ir.NewIdent(name, t)

	// Leading single values start the slice, so the common shape, one scalar
	// followed by an array, is two lines rather than three.
	var lead []ir.Expr
	i := 0
	for ; i < len(parts) && typeOrAny(parts[i]).Kind != ir.Slice; i++ {
		lead = append(lead, l.assignable(parts[i], elem, nil))
	}
	decl := assign(":=", []ir.Expr{target}, []ir.Expr{composite(t, nil, lead)})
	l.setProv(decl, at)
	l.note(decl, "Perl flattens every array in an argument list into one list before "+
		"the sub sees it. Go spreads one slice and nothing else, so the list is "+
		"built here and passed as a whole.",
		"variadic-and-no-defaults", "slices-not-arrays")
	l.emit(decl)

	for ; i < len(parts); i++ {
		p := parts[i]
		var st ir.Stmt
		if typeOrAny(p).Kind == ir.Slice {
			add := appendTo(target, l.assignable(p, t, nil))
			add.Ellipsis = true
			st = assign("=", []ir.Expr{target}, []ir.Expr{add})
		} else {
			st = assign("=", []ir.Expr{target},
				[]ir.Expr{appendTo(target, l.assignable(p, elem, nil))})
		}
		l.setProv(st, at)
		l.emit(st)
	}
	return target
}

// callArgs returns a call's arguments, with a leading block turned into the
// function value it stands for.
//
// A sub declared with an `&` prototype can be called as `apply { ... } @list`,
// where the block is the first argument written without the word `sub`. Go has
// no such syntax and no prototypes: the block is a function literal like any
// other, and it goes in as the first argument.
func (l *Lowerer) callArgs(n *ast.Call) []ast.Expr {
	if len(n.Block) == 0 {
		return n.Args
	}
	blk, ok := l.blockSubs[n]
	if !ok {
		blk = &ast.AnonSub{Body: n.Block}
		if l.blockSubs == nil {
			l.blockSubs = map[*ast.Call]*ast.AnonSub{}
		}
		l.blockSubs[n] = blk
	}
	if l.pass == 2 {
		l.approximate(n, "P2G2135", "a block passed to a sub",
			"the block became the first argument",
			"A `&` prototype lets a sub be called with a bare block, which perl reads "+
				"as a code reference in the first argument. Go has no prototypes and no "+
				"such call syntax.",
			"Nothing to change at the call site: the block is passed as a function "+
				"literal, which is what it always was.",
			"variadic-and-no-defaults")
	}
	return append([]ast.Expr{blk}, n.Args...)
}

// valueTail rewrites a sub body whose last statement is a bare value into an
// explicit return.
//
// A Perl sub yields whatever it evaluated last, so `sub { $_[0] * 2 }` returns
// twice its argument without saying so. Go has no such rule, and a bare
// expression is not even a legal statement there, so the value would otherwise
// be dropped on the floor along with the whole point of the sub.
func valueTail(body []ast.Stmt) []ast.Stmt {
	if len(body) == 0 {
		return body
	}
	es, ok := body[len(body)-1].(*ast.ExprStmt)
	if !ok || !yieldsValue(es.X) {
		return body
	}
	ret := &ast.Return{Exprs: []ast.Expr{es.X}}
	ret.StmtComments = es.StmtComments
	out := make([]ast.Stmt, len(body))
	copy(out, body)
	out[len(out)-1] = ret
	return out
}

// yieldsValue reports whether an expression is one that exists only for its
// value, so that reaching it at the end of a sub means returning it.
func yieldsValue(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.BinOp:
		// A comma expression is a sequence and yields nothing worth keeping.
		// `or` and `and` are control flow when one side is a call, which is
		// the `open(...) or die` shape, and an expression when both sides
		// produce values, which is how a comparator chains its keys.
		switch n.Op {
		case ",":
			return false
		case "or", "and", "||", "&&", "//":
			return yieldsValue(n.L) && yieldsValue(n.R)
		}
		return true
	case *ast.NumberLit, *ast.StrLit, *ast.InterpLit, *ast.QwLit, *ast.Ternary,
		*ast.Index, *ast.HashIndex, *ast.Slice, *ast.Deref, *ast.RefGen,
		*ast.AnonArray, *ast.AnonHash, *ast.MethodCall:
		return true
	case *ast.Var:
		return true
	case *ast.UnOp:
		return n.Op != "++" && n.Op != "--"
	case *ast.Call:
		// A call is the sub's value too, which matters most for the
		// one-line constructor `sub new { bless {}, shift }`: `bless` becomes
		// a composite literal rather than a Go call, and a composite literal
		// is no more a statement in Go than a bare value is, so without this
		// the whole body would vanish and the constructor would return
		// nothing at all. The builtins that exist only for their effect are
		// the exceptions; their value is one nobody reads.
		return !effectOnlyCall[n.Name]
	}
	return false
}

// effectOnlyCall lists the builtins a sub can end with without meaning to
// return anything. Perl gives every one of them a value, but it is a value
// written for the interpreter rather than for the reader: `push` hands back
// the new length and `print` hands back 1, and a Go function returning either
// would be answering a question nobody asked.
var effectOnlyCall = map[string]bool{
	"print": true, "say": true, "printf": true, "push": true, "unshift": true,
	"chomp": true, "chop": true, "die": true, "warn": true, "exit": true,
	"close": true, "open": true, "opendir": true, "closedir": true,
	"return": true, "local": true, "next": true, "last": true, "redo": true,
	"goto": true,
}
