package lower

import (
	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// assignStmts lowers an assignment into Go statements.
//
// Perl's assignment is an expression that works in list or scalar context and
// declares as a side effect. Go's is a statement with a fixed arity and an
// explicit declaring form. Reconciling those is most of what this file does.
func (l *Lowerer) assignStmts(n *ast.Assign) []ir.Stmt {
	if n.Op != "=" {
		return l.compoundAssign(n)
	}

	switch lhs := n.LHS.(type) {
	case *ast.My:
		return l.declareAssign(lhs, n)
	case *ast.List:
		return l.listAssign(flatten(lhs), n.RHS, n, false)
	case *ast.Var:
		if lhs.Sigil == '#' {
			return l.truncateArray(lhs, n)
		}
		return l.assignToVar(lhs, n)
	case *ast.Index:
		return l.assignToIndex(lhs, n)
	case *ast.HashIndex:
		if sts, ok := l.assignToEnv(lhs, n); ok {
			return sts
		}
		return l.assignToHash(lhs, n)
	case *ast.Slice:
		return l.assignToSlice(lhs, n)
	case *ast.Deref:
		x := l.expr(lhs)
		rhs := l.assignable(l.expr(n.RHS), typeOrAny(x), n.RHS)
		return []ir.Stmt{assign("=", []ir.Expr{x}, []ir.Expr{rhs})}
	case *ast.Call:
		if lhs.Name == "local" {
			return l.localStmts(flatten(argList(lhs)), n.RHS, n)
		}
		// `pos($s) = N` moves the cursor a global match starts from, which is
		// an assignment to the variable holding that position.
		if lhs.Name == "pos" {
			if x, ok := l.posTarget(lhs); ok {
				rhs := l.toInt(l.expr(n.RHS), n.RHS)
				st := assign("=", []ir.Expr{x}, []ir.Expr{rhs})
				l.setProv(st, n)
				return []ir.Stmt{st}
			}
		}
	}

	return []ir.Stmt{l.todoStmt(n, "P2G2540", "assignment target",
		"this assignment target is not implemented",
		"The converter does not recognise the shape of the left side of this "+
			"assignment.",
		"Translate the assignment by hand.")}
}

// assignToSlice lowers `@a[i, j] = LIST` and `@h{k1, k2} = LIST`.
//
// Go has no syntax for a scattered slice, but it does have multiple assignment,
// which evaluates every right-hand value before storing any of them. That is
// exactly Perl's rule, and it is what makes `@_[0, 1] = @_[1, 0]` a swap rather
// than two copies of one element.
func (l *Lowerer) assignToSlice(lhs *ast.Slice, n *ast.Assign) []ir.Stmt {
	targets := l.sliceElements(lhs)
	if len(targets) == 0 {
		return []ir.Stmt{l.todoStmt(n, "P2G2530", "slice assignment",
			"assigning to a slice of an array or hash is not implemented",
			"Perl can assign to several elements at once through a slice. The shape "+
				"of this one is not something the converter could take apart.",
			"Write one assignment per element, or a loop over the index and value "+
				"pairs.")}
	}

	var values []ir.Expr
	for _, e := range flatten(n.RHS) {
		// A slice on the right is its elements, not one list value, so that
		// the two sides line up element for element.
		if s, ok := e.(*ast.Slice); ok {
			values = append(values, l.sliceElements(s)...)
			continue
		}
		parts, _ := l.listParts([]ast.Expr{e})
		values = append(values, parts...)
	}
	if len(values) != len(targets) {
		// Perl pads a short list with undef and drops a long one. Writing that
		// out would take a temporary slice and a length check, and a mismatch
		// is nearly always a mistake rather than an intention.
		return []ir.Stmt{l.todoStmt(n, "P2G2530", "slice assignment",
			"the two sides of this slice assignment are different lengths",
			"Perl fills the extra targets with undef when the right side is shorter "+
				"and ignores the extra values when it is longer. Go's multiple "+
				"assignment requires the two sides to match.",
			"Write the assignments out individually, deciding what a missing value "+
				"should be.",
			"nil-vs-undef")}
	}

	for i := range values {
		values[i] = l.assignable(values[i], typeOrAny(targets[i]), nil)
	}
	st := assign("=", targets, values)
	l.setProv(st, n)
	l.note(st, "Go assigns to several places in one statement, and every value on "+
		"the right is worked out before any of them is stored. That is what makes a "+
		"swap written this way a swap rather than two copies of the same element.",
		"multiple-return-values")
	return []ir.Stmt{st}
}

// sliceElements turns @a[i, j] or @h{k1, k2} into the individual element
// expressions behind it.
func (l *Lowerer) sliceElements(n *ast.Slice) []ir.Expr {
	var container ir.Expr
	if v, ok := n.Base.(*ast.Var); ok {
		sig := '@'
		if n.Hash {
			sig = '%'
		}
		container = l.identFor(l.lookup(sig, v.Name, v))
	} else {
		container = l.expr(n.Base)
	}
	if container == nil {
		return nil
	}
	elem := elemOf(typeOrAny(container))

	var out []ir.Expr
	for _, ie := range n.Idx {
		for _, one := range flatten(ie) {
			if n.Hash {
				out = append(out, index(container, l.toStr(l.expr(one), one), elem))
				continue
			}
			if text, neg := negativeLiteral(one); neg {
				out = append(out, index(container,
					ir.Bin("-", lenOf(container), ir.IntLit(text), ir.TInt), elem))
				continue
			}
			out = append(out, index(container, l.toInt(l.expr(one), one), elem))
		}
	}
	return out
}

// countOf recognises `my $n = () = EXPR`, the idiom for counting what a list
// expression produced.
//
// It works in Perl because a list assignment in scalar context yields the
// number of values on its right. Nothing about that is obvious, which is why
// it has a name; in Go it is len of the slice.
func (l *Lowerer) countOf(n *ast.Assign) (ir.Expr, bool) {
	list, ok := n.LHS.(*ast.List)
	if !ok || len(list.Elems) != 0 || n.Op != "=" {
		return nil, false
	}
	src := l.list(n.RHS)
	name := l.tmp("produced")
	t := typeOrAny(src)
	if t.Kind != ir.Slice {
		t = ir.SliceOf(t)
		src = composite(t, nil, []ir.Expr{src})
	}
	decl := assign(":=", []ir.Expr{ir.NewIdent(name, t)}, []ir.Expr{src})
	l.setProv(decl, n)
	l.note(decl, "`my $n = () = LIST` counts the list: a list assignment in scalar "+
		"context yields how many values were on its right. Go has len, which says the "+
		"same thing without the trick.")
	l.emit(decl)
	out := lenOf(ir.NewIdent(name, t))
	return out, true
}

// declareAssign lowers `my ... = ...`.
func (l *Lowerer) declareAssign(my *ast.My, n *ast.Assign) []ir.Stmt {
	if my.Keyword == "local" {
		return l.localStmts(localTargets(my), n.RHS, n)
	}

	vars := declaredVars(my)
	if len(vars) == 1 && !my.Paren {
		return l.declareSingle(vars[0], n)
	}
	return l.listAssign(varExprs(vars), n.RHS, n, true)
}

// declaredVars pulls the *ast.Var nodes out of a `my` declaration.
func declaredVars(my *ast.My) []*ast.Var {
	var out []*ast.Var
	for _, v := range my.Vars {
		for _, one := range flatten(v) {
			if vv, ok := one.(*ast.Var); ok {
				out = append(out, vv)
			}
		}
	}
	return out
}

func varExprs(vs []*ast.Var) []ast.Expr {
	out := make([]ast.Expr, len(vs))
	for i, v := range vs {
		out[i] = v
	}
	return out
}

// declareSingle lowers `my $x = ...`, `my @a = ...`, `my %h = ...`.
func (l *Lowerer) declareSingle(v *ast.Var, n *ast.Assign) []ir.Stmt {
	b := l.declare(v, KindLocal)
	b.Writes++

	// The hash reference a constructor is about to bless is the object
	// itself, so it is built as the struct rather than as a map.
	if c := l.ctorSelf(v); c != nil {
		if h, ok := n.RHS.(*ast.AnonHash); ok {
			b.Type = c.Ptr
			l.observe(b, c.Ptr)
			st := assign(":=", []ir.Expr{ir.NewIdent(b.Go, b.Type)}, []ir.Expr{l.structLit(c, h)})
			l.setProv(st, n)
			return []ir.Stmt{st}
		}
	}

	var value ir.Expr
	switch v.Sigil {
	case '@':
		value = l.list(n.RHS)
		l.observe(b, typeOrAny(value))
	case '%':
		// A hash an option block fills in is a struct, so its initialiser is
		// a struct literal rather than a map one.
		if c := l.classOf(b.Type); c != nil {
			if h, ok := hashPairs(n.RHS); ok {
				st := assign(":=", []ir.Expr{ir.NewIdent(b.Go, b.Type)},
					[]ir.Expr{l.optionLit(c, h)})
				l.setProv(st, n)
				l.explainDeclaration(st, b, v)
				return []ir.Stmt{st}
			}
		}
		value = l.hashInit(n.RHS, elemOf(b.Type))
		l.observe(b, typeOrAny(value))
	default:
		value = l.scalar(n.RHS)
		l.observe(b, typeOrAny(value))
	}
	notePattern(b, n.RHS)

	coerced := l.assignable(value, b.Type, n.RHS)

	// The short declaration form takes its type from the initialiser, so it
	// only says what the binding wants when the two agree. A scalar that
	// inference left dynamic needs the type written out, or the variable ends
	// up narrower than its later uses. A container whose element type is only
	// dynamic because nothing pinned it down is better off taking the more
	// specific type the initialiser brought.
	var st ir.Stmt
	switch {
	case b.Kind == KindGlobal:
		// The declaration was hoisted to package level because a sub reads
		// the same variable, so the line that declared it now assigns to it.
		st = assign("=", []ir.Expr{ir.NewIdent(b.Go, b.Type)}, []ir.Expr{coerced})
		l.note(st, "This was a file-scope `my` that the subs below also read, so in Perl "+
			"they all close over one variable. Go has no file-scope lexical: the variable "+
			"is declared at package level and this line sets its starting value.",
			"packages-and-exported-names", "closures-and-loop-capture")
		return []ir.Stmt{st}
	case b.Type == nil || typeOrAny(coerced).Equal(b.Type):
		st = assign(":=", []ir.Expr{ir.NewIdent(b.Go, b.Type)}, []ir.Expr{coerced})
	case b.Type.Kind == ir.Any:
		st = &ir.DeclStmt{Names: []string{b.Go}, Type: b.Type, Values: []ir.Expr{coerced}}
	default:
		b.Type = typeOrAny(coerced)
		st = assign(":=", []ir.Expr{ir.NewIdent(b.Go, b.Type)}, []ir.Expr{coerced})
	}
	l.setProv(st, n)
	l.explainDeclaration(st, b, v)
	return append([]ir.Stmt{st}, l.discardIfUnused(b)...)
}

// discardIfUnused keeps a declaration compiling when nothing ever reads it.
//
// Go rejects an unused local outright. Perl does not, so a script often
// declares something for documentation or for a later edit. The blank
// assignment is Go's own way of saying "on purpose", and it is a better answer
// than dropping the line, which would hide something the developer wrote.
func (l *Lowerer) discardIfUnused(b *Binding) []ir.Stmt {
	if b == nil || b.Used > 0 || b.Kind == KindGlobal || b.Go == "" {
		return nil
	}
	st := assign("=", []ir.Expr{ir.NewIdent("_", nil)}, []ir.Expr{ir.NewIdent(b.Go, b.Type)})
	l.note(st, "Go will not compile a local variable that is never read, on the "+
		"grounds that an unread variable is usually a mistake. Nothing in this "+
		"program reads "+b.Go+", and assigning it to the blank identifier is how Go "+
		"says that is deliberate.",
		"var-vs-short-declaration")
	return []ir.Stmt{st}
}

// explainDeclaration attaches the lesson a first declaration deserves. Only
// the first few carry the full explanation, because a note on every line stops
// being read.
func (l *Lowerer) explainDeclaration(st ir.Annotated, b *Binding, v *ast.Var) {
	if l.pass != 2 {
		return
	}
	switch {
	case b.Dynamic:
		l.note(st, "Type inference could not settle on one Go type for "+b.Perl+": "+
			b.Reason+". The variable is declared as `any`, which compiles and is "+
			"honest, but every use of it needs a type assertion or a helper. Narrowing "+
			"it by hand is the single biggest readability win available in this file.",
			"type-assertions-and-switches", "static-types-and-zero-values")
	case v.Sigil == '@':
		l.note(st, "A Perl array becomes a Go slice of one element type. The := form "+
			"declares the variable and infers its type from the right-hand side, so "+
			"the type is written nowhere and still checked everywhere.",
			"slices-not-arrays", "var-vs-short-declaration")
	case v.Sigil == '%':
		l.note(st, "A Perl hash becomes a Go map with a declared key type and value "+
			"type. Keys here are strings, as they always are in Perl. Writing to a nil "+
			"map panics, so a map must be made before use; a literal like this one "+
			"makes it.",
			"nil-slices-vs-nil-maps")
	default:
		l.note(st, "my declares a lexically scoped variable, and so does Go's :=. The "+
			"difference is that the Go variable has a type from this moment on, chosen "+
			"here as "+typeWords(b.Type)+", and nothing else can ever be stored in it.",
			"var-vs-short-declaration", "static-types-and-zero-values")
	}
}

// hashInit builds the value for a hash assignment.
func (l *Lowerer) hashInit(rhs ast.Expr, want *ir.Type) ir.Expr {
	if rhs == nil {
		return composite(ir.MapOf(want), nil, nil)
	}
	if v, ok := rhs.(*ast.Var); ok && v.Sigil == '%' {
		return l.expr(v)
	}
	if c, ok := rhs.(*ast.Call); ok {
		if x, done := l.mapToHash(c, want); done {
			return x
		}
	}
	flat := flatten(rhs)
	if len(flat) == 0 {
		return composite(ir.MapOf(want), nil, nil)
	}
	if len(flat)%2 == 0 {
		keys, vals, t := l.pairs(flat)
		return composite(ir.MapOf(t), keys, vals)
	}
	// A single list whose length is not known until the program runs still
	// pairs up: Perl walks it two at a time, and so does the loop.
	if len(flat) == 1 && flattensInList(flat[0]) {
		if src := l.list(flat[0]); typeOrAny(src).Kind == ir.Slice {
			return l.hashFromPairs(src, rhs)
		}
	}
	l.approximate(rhs, "P2G2050", "hash from an odd-length list",
		"a hash was built from a list of unknown length",
		"Perl builds a hash from a flat list of alternating keys and values. The "+
			"list here does not have an even number of elements the converter can see, "+
			"so the pairs cannot be matched up statically.",
		"Build the map with an explicit loop over the list, two elements at a time.")
	return composite(ir.MapOf(want), nil, nil)
}

// assignToEnv lowers `$ENV{NAME} = VALUE`.
//
// Perl presents the environment as a hash, so setting a variable looks like an
// ordinary assignment. Go has no such view: os.Setenv is a call, and it returns
// an error, because setting an environment variable can fail.
func (l *Lowerer) assignToEnv(lhs *ast.HashIndex, n *ast.Assign) ([]ir.Stmt, bool) {
	v, ok := lhs.Base.(*ast.Var)
	if !ok || lhs.Arrow || v.Sigil != '$' || v.Name != "ENV" {
		return nil, false
	}
	key := l.toStr(l.expr(lhs.Key), lhs.Key)
	value := l.toStr(l.scalar(n.RHS), n.RHS)
	st := exprStmt(call("os", "os", "Setenv", ir.TError, key, value))
	l.setProv(st, n)
	l.note(st, "Perl's %ENV is a hash view of the environment, so setting a variable "+
		"reads as an assignment. Go has os.Setenv, which is a call and returns an "+
		"error: the environment is a system resource rather than a map.",
		"errors-are-values")
	l.approximate(n, "P2G6070", "assigning to %ENV",
		"the error os.Setenv returns is not checked",
		"Setting an environment variable can fail, and os.Setenv says so with an "+
			"error. The assignment it came from had nowhere to put one.",
		"Check the returned error where the setting matters, or use t.Setenv in a "+
			"test, which restores the old value afterwards.",
		"errors-are-values")
	return []ir.Stmt{st}, true
}

// hashFromPairs builds a map out of a flat list of alternating keys and values,
// which is how Perl builds one from anything longer than a literal.
func (l *Lowerer) hashFromPairs(src ir.Expr, at ast.Node) ir.Expr {
	elem := elemOf(typeOrAny(src))
	mapT := ir.MapOf(elem)

	list := l.tmp("flat")
	name := l.tmp("byKey")
	idx := l.tmp("i")
	listT := typeOrAny(src)

	listDecl := assign(":=", []ir.Expr{ir.NewIdent(list, listT)}, []ir.Expr{src})
	decl := assign(":=", []ir.Expr{ir.NewIdent(name, mapT)}, []ir.Expr{composite(mapT, nil, nil)})
	key := l.toStr(index(ir.NewIdent(list, listT), ir.NewIdent(idx, ir.TInt), elem), nil)
	value := index(ir.NewIdent(list, listT),
		ir.Bin("+", ir.NewIdent(idx, ir.TInt), ir.IntLit("1"), ir.TInt), elem)
	loop := &ir.For{
		Init: assign(":=", []ir.Expr{ir.NewIdent(idx, ir.TInt)}, []ir.Expr{ir.IntLit("0")}),
		Cond: ir.Bin("<", ir.Bin("+", ir.NewIdent(idx, ir.TInt), ir.IntLit("1"), ir.TInt),
			lenOf(ir.NewIdent(list, listT)), ir.TBool),
		Post: assign("+=", []ir.Expr{ir.NewIdent(idx, ir.TInt)}, []ir.Expr{ir.IntLit("2")}),
		Body: &ir.Block{Stmts: []ir.Stmt{
			assign("=", []ir.Expr{index(ir.NewIdent(name, mapT), key, elem)}, []ir.Expr{value}),
		}},
	}
	l.setProv(decl, at)
	l.note(decl, "Perl builds a hash from a flat list of alternating keys and values, "+
		"which is why a list of odd length is a warning rather than an error. Go "+
		"walks the list two at a time and puts the pairs in, which is the same rule "+
		"written where it can be seen.",
		"nil-slices-vs-nil-maps")
	l.emit(listDecl)
	l.emit(decl)
	l.emit(loop)
	return ir.NewIdent(name, mapT)
}

// listAssign lowers `my ($a, $b) = ...` and `($a, $b) = ...`.
func (l *Lowerer) listAssign(targets []ast.Expr, rhs ast.Expr, n *ast.Assign, declare bool) []ir.Stmt {
	sources := flatten(rhs)

	// The clean case: as many values as targets, each one its own expression.
	// This is what Go's multiple assignment was made for.
	//
	// A source that is itself a list disqualifies it however the counts line
	// up. `my ($first) = grep { ... } @rows` has one target and one source and
	// is not a one-to-one assignment: the right-hand side is in list context,
	// and $first takes the list's first element. Pairing them here would put
	// the source in scalar context and assign the count instead.
	if len(sources) == len(targets) && allScalarTargets(targets) && !l.anyProducesList(sources) {
		var lhsExprs, rhsExprs []ir.Expr
		var binds []*Binding
		var values []ir.Expr
		dynamic := false
		for i, t := range targets {
			v := t.(*ast.Var)
			value := l.scalar(sources[i])
			b := l.bindingFor(v, declare)
			b.Writes++
			l.observe(b, typeOrAny(value))
			if b.Type != nil && b.Type.Kind == ir.Any && typeOrAny(value).Kind != ir.Any {
				dynamic = true
			}
			binds = append(binds, b)
			values = append(values, value)
			lhsExprs = append(lhsExprs, ir.NewIdent(b.Go, b.Type))
			rhsExprs = append(rhsExprs, l.assignable(value, b.Type, sources[i]))
		}
		if declare && dynamic {
			// The short form takes each variable's type from its initialiser,
			// which would declare one of these narrower than the rest of the
			// file needs it. Declaring them separately keeps the type the
			// inference settled on.
			var out []ir.Stmt
			for i, b := range binds {
				st := l.bindDecl(true, b, values[i])
				l.setProv(st, n)
				out = append(out, st)
			}
			for _, b := range binds {
				out = append(out, l.discardIfUnused(b)...)
			}
			return out
		}
		op := "="
		if declare {
			op = ":="
		}
		st := assign(op, lhsExprs, rhsExprs)
		l.setProv(st, n)
		l.note(st, "Go assigns several variables in one statement, evaluating every "+
			"right-hand side before storing any of them. That is what makes a, b = b, a "+
			"a working swap.",
			"multiple-return-values")
		out := []ir.Stmt{st}
		if declare {
			for _, t := range targets {
				out = append(out, l.discardIfUnused(l.bindingFor(t.(*ast.Var), false))...)
			}
		}
		return out
	}

	// A call that returns exactly as many values as there are targets.
	if len(sources) == 1 {
		if c, ok := sources[0].(*ast.Call); ok {
			if s, known := l.findSub(c.Name); known && len(s.Results) == len(targets) && allScalarTargets(targets) {
				value := l.callExpr(c)
				var lhsExprs []ir.Expr
				for i, t := range targets {
					v := t.(*ast.Var)
					b := l.bindingFor(v, declare)
					b.Writes++
					l.observe(b, s.Results[i])
					lhsExprs = append(lhsExprs, ir.NewIdent(b.Go, b.Type))
				}
				op := "="
				if declare {
					op = ":="
				}
				st := assign(op, lhsExprs, []ir.Expr{value})
				l.setProv(st, n)
				l.note(st, "A Perl sub returns a list and the caller takes it apart. A "+
					"Go function declares how many values it returns, and the compiler "+
					"checks that the caller takes exactly that many.",
					"multiple-return-values")
				return []ir.Stmt{st}
			}
		}
	}

	// Anything else: the right side is a list whose length is only known at
	// run time, so the targets are filled by index.
	return l.listAssignByIndex(targets, rhs, n, declare)
}

// anyProducesList reports whether any of these expressions yields a list
// rather than a single value when it is evaluated in list context.
func (l *Lowerer) anyProducesList(es []ast.Expr) bool {
	for _, e := range es {
		if l.producesList(e) {
			return true
		}
	}
	return false
}

// producesList reports whether one expression yields a list in list context.
func (l *Lowerer) producesList(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.Var:
		return n.Sigil == '@' || n.Sigil == '%'
	case *ast.Deref:
		return n.Sigil == '@' || n.Sigil == '%'
	case *ast.Slice, *ast.List:
		return true
	case *ast.BinOp:
		return n.Op == ".."
	case *ast.Call:
		if isListBuiltin(n.Name) {
			return true
		}
		if s, known := l.findSub(n.Name); known {
			return len(s.Results) > 1
		}
	}
	return false
}

// listAssignByIndex fills targets from a slice, which is what Perl does when
// the right side is an array.
func (l *Lowerer) listAssignByIndex(targets []ast.Expr, rhs ast.Expr, n *ast.Assign, declare bool) []ir.Stmt {
	src := l.list(rhs)
	elem := elemOf(typeOrAny(src))
	tmp := l.tmp("values")
	var out []ir.Stmt
	hold := assign(":=", []ir.Expr{ir.NewIdent(tmp, typeOrAny(src))}, []ir.Expr{src})
	l.setProv(hold, n)
	l.note(hold, "The right-hand side is a list whose length is not known until the "+
		"program runs, so it is held in a slice and the variables are filled from it. "+
		"Perl leaves a variable undef when the list runs out; the helper returns the "+
		"element type's zero value instead.",
		"slices-not-arrays", "nil-vs-undef")
	out = append(out, hold)

	i := 0
	for _, t := range targets {
		v, ok := t.(*ast.Var)
		if !ok {
			continue
		}
		b := l.bindingFor(v, declare)
		b.Writes++
		if v.Sigil == '@' {
			l.observe(b, ir.SliceOf(elem))
			rest := slicing(ir.NewIdent(tmp, typeOrAny(src)), ir.IntLit(itoa(i)), nil, ir.SliceOf(elem))
			var st ir.Stmt
			if declare && b.Type != nil && !b.Type.Equal(ir.SliceOf(elem)) {
				// The rest of this list is not what the rest of the file puts
				// in the array, and Go has no conversion between two slice
				// types. Declaring the array with the type the whole file
				// agreed on keeps every later use of it compiling; what the
				// list had to offer goes in one element at a time.
				st = &ir.DeclStmt{Names: []string{b.Go}, Type: b.Type}
				loop := &ir.Range{
					Key:    ir.NewIdent("_", ir.TInt),
					Value:  ir.NewIdent(l.tmp("rest"), elem),
					X:      rest,
					Define: true,
				}
				loop.Body = &ir.Block{Stmts: []ir.Stmt{
					assign("=", []ir.Expr{ir.NewIdent(b.Go, b.Type)},
						[]ir.Expr{appendTo(ir.NewIdent(b.Go, b.Type),
							l.assignable(loop.Value, elemOf(b.Type), nil))}),
				}}
				l.note(st, "An array on the left of a list assignment swallows everything "+
					"that is left. The rest of the file puts a different kind of value in "+
					"this array than this list holds, and Go converts between two slice "+
					"types one element at a time.")
				out = append(out, st, loop)
				i++
				continue
			}
			st = assign(declOp(declare), []ir.Expr{ir.NewIdent(b.Go, b.Type)}, []ir.Expr{rest})
			l.note(st, "An array on the left of a list assignment swallows everything "+
				"that is left, which Go writes as a slice expression from that index on.")
			out = append(out, st)
			i++
			continue
		}
		l.observe(b, elem)
		val := l.helperCall(hAt, elem, ir.NewIdent(tmp, typeOrAny(src)), ir.IntLit(itoa(i)))
		out = append(out, l.bindDecl(declare, b, val))
		if declare {
			out = append(out, l.discardIfUnused(b)...)
		}
		i++
	}
	return out
}

// bindDecl writes one binding's declaration or assignment and keeps the Go
// variable's type and the binding's agreed.
//
// A short declaration takes its type from its initialiser. Where the binding
// stayed dynamic and the initialiser did not, that leaves the Go variable
// narrower than the binding thinks it is, and every later use asserts a type
// the variable does not have, which does not compile.
func (l *Lowerer) bindDecl(declare bool, b *Binding, value ir.Expr) ir.Stmt {
	coerced := l.assignable(value, b.Type, nil)
	if declare && b.Type != nil && b.Type.Kind == ir.Any && typeOrAny(coerced).Kind != ir.Any {
		return &ir.DeclStmt{Names: []string{b.Go}, Type: b.Type, Values: []ir.Expr{coerced}}
	}
	return assign(declOp(declare), []ir.Expr{ir.NewIdent(b.Go, b.Type)}, []ir.Expr{coerced})
}

func declOp(declare bool) string {
	if declare {
		return ":="
	}
	return "="
}

// bindingFor returns the binding for a target, declaring it when the
// assignment declares.
func (l *Lowerer) bindingFor(v *ast.Var, declare bool) *Binding {
	if declare {
		return l.declare(v, KindLocal)
	}
	return l.lookup(v.Sigil, v.Name, v)
}

func allScalarTargets(targets []ast.Expr) bool {
	for _, t := range targets {
		v, ok := t.(*ast.Var)
		if !ok || v.Sigil != '$' {
			return false
		}
	}
	return len(targets) > 0
}

// assignToVar lowers `$x = ...`, `@a = ...`, `%h = ...`.
func (l *Lowerer) assignToVar(v *ast.Var, n *ast.Assign) []ir.Stmt {
	// Assigning to a glob replaces an entry in the symbol table, so every
	// call already written against that name starts running the new code.
	// Nothing in Go can be reached through a name at run time.
	if v.Sigil == '*' {
		return []ir.Stmt{l.todoStmt(n, "P2G8020", "*"+v.Name+" = ...",
			"replacing a subroutine at run time has no Go equivalent",
			"Assigning to `*"+v.Name+"` puts a new subroutine in the symbol table under "+
				"that name, and every call site already written against it, including the "+
				"ones in objects that already exist, starts running the new one from the "+
				"next call onwards. Go resolves a call when it compiles the program and "+
				"has no symbol table to write into.",
			"Where the point was to vary behaviour, hold the function in a variable or "+
				"a field and call through it, which makes the substitution visible at "+
				"every call site that can see it. Where the point was to patch a package "+
				"the program does not own, there is no equivalent at all.",
			"implicit-interfaces", "methods-and-receivers")}
	}
	// One of Perl's own variables is not an ordinary name and does not get an
	// ordinary binding: whatever it maps onto is the assignment target.
	if target := l.specialVar(v); target != nil {
		value := l.assignable(l.scalar(n.RHS), typeOrAny(target), n.RHS)
		st := assign("=", []ir.Expr{target}, []ir.Expr{value})
		l.setProv(st, n)
		return []ir.Stmt{st}
	}

	b := l.lookup(v.Sigil, v.Name, v)
	b.Writes++

	var value ir.Expr
	switch v.Sigil {
	case '@':
		value = l.list(n.RHS)
	case '%':
		value = l.hashInit(n.RHS, elemOf(b.Type))
	default:
		value = l.scalar(n.RHS)
	}
	if !selfReferential(v, n.RHS) {
		l.observe(b, typeOrAny(value))
	}

	st := assign("=", []ir.Expr{ir.NewIdent(b.Go, b.Type)}, []ir.Expr{l.assignable(value, b.Type, n.RHS)})
	l.setProv(st, n)
	return []ir.Stmt{st}
}

// truncateArray lowers `$#array = N`, which shortens or extends an array.
func (l *Lowerer) truncateArray(v *ast.Var, n *ast.Assign) []ir.Stmt {
	b := l.lookup('@', v.Name, v)
	b.Writes++
	length := ir.Bin("+", l.toInt(l.expr(n.RHS), n.RHS), ir.IntLit("1"), ir.TInt)
	target := ir.NewIdent(b.Go, b.Type)
	st := assign("=", []ir.Expr{target}, []ir.Expr{slicing(target, nil, length, b.Type)})
	l.setProv(st, n)
	l.approximate(n, "P2G5560", "assigning to $#array",
		"setting the last index only shortens here",
		"Assigning to $#array sets the array's length: a smaller value throws "+
			"elements away, a larger one pads with undef. Reslicing in Go shortens "+
			"correctly, but it cannot grow past the slice's capacity the way Perl "+
			"grows an array.",
		"To grow, append the zero value the required number of times instead.",
		"slices-not-arrays", "slice-aliasing-and-copy")
	return []ir.Stmt{st}
}

// assignToIndex lowers `$a[i] = v`.
func (l *Lowerer) assignToIndex(lhs *ast.Index, n *ast.Assign) []ir.Stmt {
	base, idx, elem := l.indexParts(lhs)
	if base == nil {
		return nil
	}
	value := l.assignable(l.scalar(n.RHS), elem, n.RHS)
	if b := l.arrayBindingOf(lhs); b != nil {
		b.Writes++
		l.observeElem(b, typeOrAny(value))
	}
	st := assign("=", []ir.Expr{index(base, idx, elem)}, []ir.Expr{value})
	l.setProv(st, n)
	l.approximate(n, "P2G5561", "assigning past the end of an array",
		"Go does not grow a slice on assignment",
		"Assigning to an index beyond the end of a Perl array extends it, filling "+
			"the gap with undef. Assigning past the end of a Go slice panics.",
		"Use append to add elements, and index assignment only for positions that "+
			"already exist.",
		"slices-not-arrays")
	return []ir.Stmt{st}
}

// arrayBindingOf finds the binding an array element access refers to, when it
// refers to a plain named array.
func (l *Lowerer) arrayBindingOf(n *ast.Index) *Binding {
	if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' {
		return l.lookup('@', v.Name, v)
	}
	return nil
}

// hashBindingOf finds the binding a hash element access refers to.
func (l *Lowerer) hashBindingOf(n *ast.HashIndex) *Binding {
	if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' && v.Name != "ENV" {
		return l.lookup('%', v.Name, v)
	}
	return nil
}

// assignToHash lowers `$h{k} = v`, including the nested form that Perl
// autovivifies.
func (l *Lowerer) assignToHash(lhs *ast.HashIndex, n *ast.Assign) []ir.Stmt {
	var out []ir.Stmt
	out = append(out, l.autovivify(lhs)...)

	m, key, elem, field := l.hashPartsField(lhs)
	if m == nil {
		return out
	}
	if field != nil {
		// A struct field: the place is the whole expression rather than a
		// lookup inside a container, and the value says what the field holds.
		if !selfReferential(lhs, n.RHS) {
			l.observeField(field, typeOrAny(l.scalar(n.RHS)))
		}
		st := assign("=", []ir.Expr{m}, []ir.Expr{l.assignable(l.scalar(n.RHS), field.Type, n.RHS)})
		l.setProv(st, n)
		return append(out, st)
	}
	value := l.assignable(l.scalar(n.RHS), elem, n.RHS)
	if key == nil {
		st := assign("=", []ir.Expr{m}, []ir.Expr{value})
		l.setProv(st, n)
		return append(out, st)
	}
	if b := l.hashBindingOf(lhs); b != nil {
		b.Writes++
		l.observeElem(b, typeOrAny(l.scalar(n.RHS)))
	} else if f := l.fieldOf(lhs.Base); f != nil {
		// The container is a struct field rather than a variable, and a field
		// of a map type learns what it holds the same way.
		l.observeField(f, ir.MapOf(typeOrAny(l.scalar(n.RHS))))
	}
	st := assign("=", []ir.Expr{index(m, key, elem)}, []ir.Expr{value})
	l.setProv(st, n)
	out = append(out, st)
	return out
}

// autovivify emits the map creation Perl would have done implicitly.
//
// In Perl, $tree{a}{b} = 1 creates the inner hash on the way through. In Go
// the inner map is nil and writing to a nil map panics, which is one of the
// first runtime failures a Perl developer meets.
func (l *Lowerer) autovivify(lhs *ast.HashIndex) []ir.Stmt {
	inner, ok := lhs.Base.(*ast.HashIndex)
	if !ok {
		return nil
	}
	m, key, elem := l.hashParts(inner)
	if m == nil || key == nil || elem == nil {
		return nil
	}
	// made is what goes in when the key is missing. Where inference settled on
	// a map, that is the map. Where it did not, the outer map holds `any` and
	// every read of the inner one asserts a map out of it, so putting a map
	// there is what makes those assertions true instead of a panic on nil.
	made := elem
	switch elem.Kind {
	case ir.Map:
	case ir.Any:
		made = ir.MapOf(ir.TAny)
	default:
		return nil
	}
	okName := l.tmp("ok")
	check := assign(":=", []ir.Expr{ir.NewIdent("_", nil), ir.NewIdent(okName, ir.TBool)},
		[]ir.Expr{indexComma(m, key, elem)})
	create := &ir.If{
		Cond: negated(ir.NewIdent(okName, ir.TBool)),
		Then: &ir.Block{Stmts: []ir.Stmt{
			assign("=", []ir.Expr{index(m, key, elem)}, []ir.Expr{composite(made, nil, nil)}),
		}},
	}
	l.note(check, "Perl creates the inner hash on the way through, which is called "+
		"autovivification. Go does not: the inner map is nil until something makes "+
		"it, and writing to a nil map panics. The check and the make are what Perl "+
		"was doing invisibly.",
		"nil-slices-vs-nil-maps", "comma-ok-idiom")
	return []ir.Stmt{check, create}
}

// autovivifyTarget emits the map creation Perl would have done implicitly
// before a statement writes through a nested hash element.
//
// `$h{a}{b} = 1` goes through assignToHash, which already does this. `++` and
// `+=` on the same place do not, and they are how a counting or accumulating
// hash of hashes is actually written, so without this the first line of the
// loop panics.
func (l *Lowerer) autovivifyTarget(e ast.Expr) {
	h, ok := e.(*ast.HashIndex)
	if !ok {
		return
	}
	for _, st := range l.autovivify(h) {
		l.emit(st)
	}
}

// compoundAssign lowers +=, .=, //= and the rest.
func (l *Lowerer) compoundAssign(n *ast.Assign) []ir.Stmt {
	op := n.Op[:len(n.Op)-1]
	l.autovivifyTarget(n.LHS)
	target := l.assignTarget(n.LHS)
	if target == nil {
		return []ir.Stmt{l.todoStmt(n, "P2G2540", "compound assignment",
			"this compound assignment is not implemented",
			"The converter does not recognise the left side of this assignment.",
			"Translate it by hand.")}
	}
	t := typeOrAny(target)

	switch op {
	case "||", "//":
		value := l.assignable(l.scalar(n.RHS), t, n.RHS)
		guard := &ir.If{
			Cond: negated(l.toBool(target, n.LHS)),
			Then: &ir.Block{Stmts: []ir.Stmt{assign("=", []ir.Expr{target}, []ir.Expr{value})}},
		}
		l.setProv(guard, n)
		l.note(guard, "Perl's ||= and //= assign only when the current value is false "+
			"or undefined. Go has no such operator, so the test is written out, which "+
			"also makes it obvious which of the two tests is meant.",
			"nil-vs-undef")
		return []ir.Stmt{guard}

	case "&&":
		value := l.assignable(l.scalar(n.RHS), t, n.RHS)
		guard := &ir.If{
			Cond: l.toBool(target, n.LHS),
			Then: &ir.Block{Stmts: []ir.Stmt{assign("=", []ir.Expr{target}, []ir.Expr{value})}},
		}
		l.setProv(guard, n)
		return []ir.Stmt{guard}

	case ".":
		value := l.toStr(l.scalar(n.RHS), n.RHS)
		if b := l.bindingOfTarget(n.LHS); b != nil {
			l.observe(b, ir.TString)
		}
		if t.Kind != ir.String {
			// The target's type did not resolve to text, and Go will not add a
			// string to anything else, so the concatenation is written out
			// through the rendering both sides have to go through anyway.
			st := assign("=", []ir.Expr{target},
				[]ir.Expr{ir.Bin("+", l.toStr(target, n.LHS), value, ir.TString)})
			l.setProv(st, n)
			l.note(st, "Perl's .= appends text to whatever the variable held, converting "+
				"it on the way. This one's type did not resolve to text, so the "+
				"conversion is written out.",
				"explicit-conversions-no-coercion")
			return []ir.Stmt{st}
		}
		st := assign("+=", []ir.Expr{target}, []ir.Expr{value})
		l.setProv(st, n)
		l.note(st, "Go's + concatenates strings, so .= becomes +=. For a loop that "+
			"builds a large string, strings.Builder avoids copying on every append.",
			"strings-are-bytes")
		return []ir.Stmt{st}

	case "x":
		value := l.helperCall(hStrRange, ir.TString)
		_ = value
		st := assign("=", []ir.Expr{target},
			[]ir.Expr{call("strings", "strings", "Repeat", ir.TString, l.toStr(target, n.LHS), l.toInt(l.expr(n.RHS), n.RHS))})
		l.setProv(st, n)
		return []ir.Stmt{st}

	case "**":
		st := assign("=", []ir.Expr{target}, []ir.Expr{l.power(&ast.BinOp{Op: "**", L: n.LHS, R: n.RHS})})
		l.setProv(st, n)
		return []ir.Stmt{st}

	case "%":
		st := assign("=", []ir.Expr{target}, []ir.Expr{l.modulo(&ast.BinOp{Op: "%", L: n.LHS, R: n.RHS})})
		l.setProv(st, n)
		return []ir.Stmt{st}

	case "/":
		st := assign("=", []ir.Expr{target},
			[]ir.Expr{l.assignable(l.binop(&ast.BinOp{Op: "/", L: n.LHS, R: n.RHS}), t, n.RHS)})
		l.setProv(st, n)
		return []ir.Stmt{st}

	case "+", "-", "*":
		// The right side is lowered once: lowering it twice would run any
		// setup it needed twice as well.
		value := l.scalar(n.RHS)
		if b := l.bindingOfTarget(n.LHS); b != nil {
			l.observe(b, typeOrAny(value))
		}
		switch t.Kind {
		case ir.Float:
			value = l.toFloat(value, n.RHS)
		case ir.Int:
			value = l.toInt(value, n.RHS)
		case ir.Any:
			// A dynamic target cannot be added to directly, so the arithmetic
			// happens in float64 and the result goes back into the value.
			st := assign("=", []ir.Expr{target},
				[]ir.Expr{ir.Bin(op, l.toFloat(target, n.LHS), l.toFloat(value, n.RHS), ir.TFloat)})
			l.setProv(st, n)
			l.note(st, "Arithmetic needs to know what it is adding, and this variable's "+
				"type did not resolve, so both sides go through a conversion first. "+
				"Giving the variable a concrete type at its declaration removes all of "+
				"this.",
				"explicit-conversions-no-coercion", "type-assertions-and-switches")
			return []ir.Stmt{st}
		}
		st := assign(op+"=", []ir.Expr{target}, []ir.Expr{value})
		l.setProv(st, n)
		return []ir.Stmt{st}
	}

	st := assign(op+"=", []ir.Expr{target}, []ir.Expr{l.scalar(n.RHS)})
	l.setProv(st, n)
	return []ir.Stmt{st}
}

// assignTarget lowers the left side of an assignment as an addressable Go
// expression.
func (l *Lowerer) assignTarget(e ast.Expr) ir.Expr {
	switch n := e.(type) {
	case *ast.Var:
		if n.Sigil == '$' || n.Sigil == '@' || n.Sigil == '%' {
			if x := l.specialVar(n); x != nil {
				return x
			}
			b := l.lookup(n.Sigil, n.Name, n)
			b.Writes++
			return l.identFor(b)
		}
	case *ast.Index:
		base, idx, elem := l.indexParts(n)
		if base == nil {
			return nil
		}
		return index(base, idx, elem)
	case *ast.HashIndex:
		m, key, elem, field := l.hashPartsField(n)
		if m == nil {
			return nil
		}
		if field != nil {
			// A struct field is already the place; there is nothing to index.
			return m
		}
		if key == nil {
			return nil
		}
		return index(m, key, elem)
	case *ast.Deref:
		return l.expr(n)
	}
	return nil
}

// bindingOfTarget finds the binding an assignment target names, if any.
func (l *Lowerer) bindingOfTarget(e ast.Expr) *Binding {
	switch n := e.(type) {
	case *ast.Var:
		if b, ok := l.scope.lookup(varKey(n.Sigil, n.Name)); ok {
			return b
		}
		if b, ok := l.globalSeen[varKey(n.Sigil, n.Name)]; ok {
			return b
		}
	case *ast.Index:
		return l.arrayBindingOf(n)
	case *ast.HashIndex:
		return l.hashBindingOf(n)
	}
	return nil
}

// assignExpr lowers an assignment used for its value, which Go cannot express.
// The assignment is hoisted in front of the statement and the target is
// returned in its place.
func (l *Lowerer) assignExpr(n *ast.Assign) ir.Expr {
	if x, ok := l.countOf(n); ok {
		return x
	}
	for _, st := range l.assignStmts(n) {
		l.emit(st)
	}
	if t := l.assignTarget(n.LHS); t != nil {
		return t
	}
	if my, ok := n.LHS.(*ast.My); ok {
		if vs := declaredVars(my); len(vs) == 1 {
			b := l.lookup(vs[0].Sigil, vs[0].Name, vs[0])
			return ir.NewIdent(b.Go, b.Type)
		}
	}
	return ir.Nil(ir.TAny)
}

// selfReferential reports whether an assignment reads the very place it
// writes, as `$x = $x + 1` and `$self->{n} = $self->{n} + $by` both do.
//
// Such an assignment says nothing new about what the place holds: the type of
// its right-hand side was worked out from the place's own type, so recording
// it as evidence would feed a guess back in as a fact. That matters most
// early, while the type is still unknown and the arithmetic has fallen back to
// float64 for want of anything better.
func selfReferential(lhs, rhs ast.Expr) bool {
	found := false
	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		if e == nil || found {
			return
		}
		if samePlace(lhs, e) {
			found = true
			return
		}
		switch n := e.(type) {
		case *ast.BinOp:
			walk(n.L)
			walk(n.R)
		case *ast.UnOp:
			walk(n.X)
		case *ast.Ternary:
			walk(n.Cond)
			walk(n.A)
			walk(n.B)
		case *ast.List:
			for _, el := range n.Elems {
				walk(el)
			}
		case *ast.Call:
			for _, a := range n.Args {
				walk(a)
			}
		}
	}
	walk(rhs)
	return found
}

// samePlace reports whether two expressions name the same variable or the same
// element of the same container, judged by their spelling.
func samePlace(a, b ast.Expr) bool {
	switch x := a.(type) {
	case *ast.Var:
		y, ok := b.(*ast.Var)
		return ok && x.Sigil == y.Sigil && x.Name == y.Name
	case *ast.HashIndex:
		y, ok := b.(*ast.HashIndex)
		if !ok || x.Arrow != y.Arrow || !samePlace(x.Base, y.Base) {
			return false
		}
		ka, oka := staticString(x.Key)
		kb, okb := staticString(y.Key)
		return oka && okb && ka == kb
	case *ast.Index:
		y, ok := b.(*ast.Index)
		return ok && x.Arrow == y.Arrow && samePlace(x.Base, y.Base) && samePlace(x.Idx, y.Idx)
	case *ast.NumberLit:
		y, ok := b.(*ast.NumberLit)
		return ok && x.Text == y.Text
	}
	return false
}
