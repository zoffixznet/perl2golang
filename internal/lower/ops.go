package lower

import (
	"strconv"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// binop lowers a binary operator.
//
// Perl decides what an operator means from the operator itself, not from its
// operands: + is always numeric and . is always textual, whatever the values
// happen to be. Go decides from the operands and refuses to guess. So every
// operator here first coerces its sides into the shape the Go operator needs,
// and the coercion is exactly the lesson the annotated output carries.
func (l *Lowerer) binop(n *ast.BinOp) ir.Expr {
	switch n.Op {
	case "+", "-", "*":
		lx, rx, t := l.numPair(n.L, n.R)
		return ir.Bin(n.Op, lx, rx, t)

	case "/":
		lx := l.toFloat(l.expr(n.L), n.L)
		rx := l.toFloat(l.expr(n.R), n.R)
		out := ir.Bin("/", lx, rx, ir.TFloat)
		l.note(out, "Perl's / always produces a floating-point result: 7 / 2 is 3.5. "+
			"Go's / on two ints is integer division and would give 3, so both sides "+
			"are converted to float64 first.",
			"explicit-conversions-no-coercion")
		return out

	case "%":
		return l.modulo(n)

	case "**":
		return l.power(n)

	case ".":
		lx := l.toStr(l.expr(n.L), n.L)
		rx := l.toStr(l.expr(n.R), n.R)
		return ir.Bin("+", lx, rx, ir.TString)

	case "x":
		return l.repeatOp(n)

	case "==", "!=", "<", ">", "<=", ">=":
		lx, rx, _ := l.numPair(n.L, n.R)
		return ir.Bin(n.Op, lx, rx, ir.TBool)

	case "eq", "ne", "lt", "gt", "le", "ge":
		goOp := map[string]string{"eq": "==", "ne": "!=", "lt": "<", "gt": ">", "le": "<=", "ge": ">="}[n.Op]
		lx := l.toStr(l.expr(n.L), n.L)
		rx := l.toStr(l.expr(n.R), n.R)
		out := ir.Bin(goOp, lx, rx, ir.TBool)
		if n.Op == "eq" || n.Op == "ne" {
			l.note(out, "Perl needs two families of comparison because a scalar has no "+
				"type: eq compares as text, == compares as numbers. Go has one family, "+
				"because the operands already know what they are.",
				"static-types-and-zero-values")
		}
		return out

	case "<=>":
		lx, rx, _ := l.numPair(n.L, n.R)
		out := call("cmp", "cmp", "Compare", ir.TInt, lx, rx)
		l.note(out, "The spaceship operator returns -1, 0 or 1. Go's cmp.Compare does "+
			"the same and is what sort comparators are written with.",
			"sort-slice")
		return out

	case "cmp":
		lx := l.toStr(l.expr(n.L), n.L)
		rx := l.toStr(l.expr(n.R), n.R)
		return call("strings", "strings", "Compare", ir.TInt, lx, rx)

	case "&&", "and":
		return ir.Bin("&&", l.cond(n.L), l.cond(n.R), ir.TBool)

	case "||", "or":
		return l.orValue(n, false)

	case "//":
		return l.orValue(n, true)

	case "xor":
		return ir.Bin("!=", l.cond(n.L), l.cond(n.R), ir.TBool)

	case "&", "|", "^", "<<", ">>":
		lx := l.toInt(l.expr(n.L), n.L)
		rx := l.toInt(l.expr(n.R), n.R)
		return ir.Bin(n.Op, lx, rx, ir.TInt)

	case "..", "...":
		return l.rangeExpr(n)

	case ",", "=>":
		parts, t := l.listParts([]ast.Expr{n})
		return composite(ir.SliceOf(t), nil, parts)

	case "=~", "!~":
		// The parser normally folds these into Match/Subst nodes; reaching
		// here means the right side was not a pattern.
		return l.todoExpr(n, "P2G4590", "binding operator",
			"this pattern binding is not implemented",
			"The right side of =~ is not a pattern the converter could read statically.",
			"Compile the pattern with regexp.MustCompile and call its methods directly.",
			"mustcompile-pattern")
	}

	return l.todoExpr(n, "P2G3511", "operator "+n.Op,
		"the "+n.Op+" operator is not implemented",
		"The converter has no rule for the "+n.Op+" operator.",
		"Translate the expression by hand.")
}

// numPair lowers both sides of an arithmetic operator and agrees on a result
// type: int when both sides are integral, float64 otherwise.
func (l *Lowerer) numPair(a, b ast.Expr) (ir.Expr, ir.Expr, *ir.Type) {
	lx := l.expr(a)
	rx := l.expr(b)
	lt, rt := typeOrAny(lx), typeOrAny(rx)
	if lt.Kind == ir.Int && rt.Kind == ir.Int {
		return lx, rx, ir.TInt
	}
	return l.toFloat(lx, a), l.toFloat(rx, b), ir.TFloat
}

func typeOrAny(x ir.Expr) *ir.Type {
	if x == nil || x.Type() == nil {
		return ir.TAny
	}
	return x.Type()
}

// modulo lowers %, which is one of the genuine semantic traps.
func (l *Lowerer) modulo(n *ast.BinOp) ir.Expr {
	lx := l.toInt(l.expr(n.L), n.L)
	rx := l.toInt(l.expr(n.R), n.R)
	if mayBeNegative(n.L) || mayBeNegative(n.R) {
		out := l.helperCall(hMod, ir.TInt, lx, rx)
		l.approximate(n, "P2G5520", "% with a possibly negative operand",
			"% follows a different sign rule in Go",
			"Perl's % takes the sign of the right operand, so -7 % 3 is 2. Go's % takes "+
				"the sign of the left operand, so -7 % 3 is -1. One of the operands here "+
				"can be negative, so the two would disagree.",
			"The generated code calls a small helper that reproduces Perl's rule. If "+
				"both operands are always non-negative in practice, replace the call with "+
				"the plain % operator.")
		return out
	}
	out := ir.Bin("%", lx, rx, ir.TInt)
	l.note(out, "Perl's % takes the sign of the right operand and Go's takes the sign "+
		"of the left, so -7 % 3 is 2 in Perl and -1 in Go. Both operands here are "+
		"non-negative, where the two agree.")
	return out
}

// mayBeNegative is a deliberately conservative check: it reports true unless
// the expression is obviously non-negative.
func mayBeNegative(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.NumberLit:
		return len(n.Text) > 0 && n.Text[0] == '-'
	case *ast.UnOp:
		return n.Op == "-"
	case *ast.BinOp:
		return n.Op == "-" || mayBeNegative(n.L) || mayBeNegative(n.R)
	case *ast.Call:
		switch n.Name {
		case "length", "scalar", "abs":
			return false
		}
	}
	return true
}

// power lowers **.
func (l *Lowerer) power(n *ast.BinOp) ir.Expr {
	lx := l.expr(n.L)
	rx := l.expr(n.R)
	if typeOrAny(lx).Kind == ir.Int && typeOrAny(rx).Kind == ir.Int && !mayBeNegative(n.R) {
		out := l.helperCall(hPowInt, ir.TInt, lx, rx)
		l.note(out, "Go has no exponentiation operator. math.Pow works in float64; "+
			"raising two integers is kept integral here so the result still prints "+
			"without a decimal point.")
		return out
	}
	out := call("math", "math", "Pow", ir.TFloat, l.toFloat(lx, n.L), l.toFloat(rx, n.R))
	l.note(out, "Go has no ** operator. math.Pow is the equivalent and it works in "+
		"float64.")
	return out
}

// repeatOp lowers the x operator, which repeats a string or a list depending
// on what is to its left.
func (l *Lowerer) repeatOp(n *ast.BinOp) ir.Expr {
	count := l.toInt(l.expr(n.R), n.R)
	if isListish(n.L) {
		src := l.list(n.L)
		out := l.helperCall(hRepeatList, typeOrAny(src), src, count)
		l.note(out, "The x operator repeats a list when its left side is a list. Go "+
			"has no such operator, so a small generic helper does the appending.")
		return out
	}
	s := l.toStr(l.expr(n.L), n.L)
	out := call("strings", "strings", "Repeat", ir.TString, s, count)
	l.note(out, "strings.Repeat is Go's x operator for text.")
	return out
}

// isListish reports whether an expression is in list form, which is what
// decides between the two meanings of x.
func isListish(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.List:
		return true
	case *ast.QwLit:
		return true
	case *ast.Var:
		return n.Sigil == '@'
	}
	return false
}

// orValue lowers || and //, which return a value rather than a bool.
//
// Go's || is strictly boolean, so the Perl idiom `my $name = $arg || 'default'`
// has no operator form. Writing it out as an if is what a Go developer does,
// and it is clearer about what is being tested.
func (l *Lowerer) orValue(n *ast.BinOp, definedOr bool) ir.Expr {
	if !definedOr {
		if out, ok := l.orderingChain(n); ok {
			return out
		}
	}
	lx := l.expr(n.L)
	rx := l.expr(n.R)
	t := join(typeOrAny(lx), typeOrAny(rx))

	name := l.tmp(defaultName(n.L))
	decl := &ir.DeclStmt{Names: []string{name}, Type: t, Values: []ir.Expr{l.assignable(lx, t, n.L)}}
	l.setProv(decl, n)
	target := ir.NewIdent(name, t)
	body := &ir.Block{Stmts: []ir.Stmt{assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(rx, t, n.R)})}}
	guard := &ir.If{Cond: negated(l.toBool(target, n.L)), Then: body}
	if definedOr {
		l.note(decl, "// tests whether the left side is defined, not whether it is "+
			"true, so 0 and the empty string pass it. Go has no undefined value: a "+
			"declared variable always holds its type's zero value, which is why this "+
			"is written as an explicit test.",
			"nil-vs-undef", "static-types-and-zero-values")
	} else {
		l.note(decl, "Perl's || returns the first true operand, not a bool, which is "+
			"why it works for defaulting. Go's || is strictly boolean, so the default "+
			"is applied with an if.",
			"static-types-and-zero-values")
	}
	l.emit(decl)
	l.emit(guard)
	return ir.NewIdent(name, t)
}

// orderingChain recognises the multi-key sort comparator, `$a->{x} <=> $b->{x}
// || $a->{y} cmp $b->{y}`, and returns cmp.Or over the comparisons.
//
// The idiom leans on || returning the first true value: a comparison that says
// "equal" is 0, which is false, so the next key gets a turn. cmp.Or is the same
// rule, first non-zero wins, and it is what Go code sorting on several keys
// actually looks like. Only orderings qualify, so nothing with a side effect
// gets pulled into an argument list that Go evaluates eagerly.
func (l *Lowerer) orderingChain(n *ast.BinOp) (ir.Expr, bool) {
	leaves, ok := orderingLeaves(n)
	if !ok || len(leaves) < 2 {
		return nil, false
	}
	args := make([]ir.Expr, 0, len(leaves))
	for _, leaf := range leaves {
		x := l.expr(leaf)
		if typeOrAny(x).Kind != ir.Int {
			return nil, false
		}
		args = append(args, x)
	}
	out := call("cmp", "cmp", "Or", ir.TInt, args...)
	l.note(out, "Chaining comparisons with || works because a comparison that finds "+
		"the values equal returns 0, which is false, so the next key decides. "+
		"cmp.Or returns its first non-zero argument, which is the same rule and is "+
		"how a Go comparator sorts on more than one key.",
		"sort-slice")
	return out, true
}

// orderingLeaves flattens a || chain whose every leaf is an ordering
// comparison. Anything else in the chain disqualifies it.
func orderingLeaves(e ast.Expr) ([]ast.Expr, bool) {
	b, ok := e.(*ast.BinOp)
	if !ok {
		return nil, false
	}
	switch b.Op {
	case "<=>", "cmp":
		return []ast.Expr{b}, true
	case "||":
		left, ok := orderingLeaves(b.L)
		if !ok {
			return nil, false
		}
		right, ok := orderingLeaves(b.R)
		if !ok {
			return nil, false
		}
		return append(left, right...), true
	}
	return nil, false
}

// defaultName invents a readable identifier for a temporary based on what it
// holds.
func defaultName(e ast.Expr) string {
	switch n := e.(type) {
	case *ast.Var:
		return goName(n.Name)
	case *ast.Call:
		return goName(n.Name)
	case *ast.HashIndex:
		return "value"
	case *ast.Index:
		return "item"
	}
	return "value"
}

// rangeExpr lowers the range operator in list context.
func (l *Lowerer) rangeExpr(n *ast.BinOp) ir.Expr {
	// Both ends are scalar context, so a parenthesised expression is the
	// expression rather than a one-element list, and a comma expression is
	// its last element.
	lx := l.scalar(n.L)
	rx := l.scalar(n.R)
	if typeOrAny(lx).Kind == ir.String || typeOrAny(rx).Kind == ir.String {
		out := l.helperCall(hStrRange, ir.SliceOf(ir.TString), l.toStr(lx, n.L), l.toStr(rx, n.R))
		l.approximate(n, "P2G5030", "string range",
			"the string range operator uses Perl's magic increment",
			"'aa' .. 'ad' walks the range with Perl's magic string increment, which "+
				"carries like an odometer: 'az' becomes 'ba' and 'Zz' becomes 'AAa'. Go "+
				"has nothing like it.",
			"The generated code calls a helper that reproduces the increment. For a "+
				"fixed set of names, a slice literal is clearer.")
		return out
	}
	out := l.helperCall(hSeq, ir.SliceOf(ir.TInt), l.toInt(lx, n.L), l.toInt(rx, n.R))
	l.note(out, "Perl's range operator builds the whole list. In a for loop Go counts "+
		"instead of building anything, which is why a foreach over a range becomes a "+
		"counting loop; here the list itself is wanted, so it is built.",
		"range-is-not-foreach")
	return out
}

// unop lowers a prefix or postfix unary operator.
func (l *Lowerer) unop(n *ast.UnOp) ir.Expr {
	switch n.Op {
	case "!", "not":
		return negated(l.cond(n.X))
	case "-":
		x := l.expr(n.X)
		if isNum(typeOrAny(x)) {
			return ir.Un("-", x, x.Type())
		}
		return ir.Un("-", l.toFloat(x, n.X), ir.TFloat)
	case "+":
		return l.expr(n.X)
	case "~":
		return ir.Un("^", l.toInt(l.expr(n.X), n.X), ir.TInt)
	case "\\":
		return l.refGen(&ast.RefGen{X: n.X})
	case "defined":
		return l.definedExpr(n.X, n)
	case "++", "--":
		return l.incDecExpr(n)
	}
	return l.todoExpr(n, "P2G3511", "unary "+n.Op,
		"the unary "+n.Op+" operator is not implemented",
		"The converter has no rule for this operator.",
		"Translate the expression by hand.")
}

// incDecExpr handles ++ and -- used for their value, which Go does not allow:
// in Go they are statements. The statement layer handles the common case, so
// reaching here means the value was wanted.
func (l *Lowerer) incDecExpr(n *ast.UnOp) ir.Expr {
	target := l.expr(n.X)
	step := "++"
	if n.Op == "--" {
		step = "--"
	}
	if typeOrAny(target).Kind == ir.String && n.Op == "++" {
		out := l.helperCall(hStrInc, ir.TString, target)
		l.approximate(n, "P2G5030", "magic string increment",
			"++ on text uses Perl's magic increment",
			"Incrementing a string in Perl increments it like an odometer when it "+
				"looks like a word: 'aa' becomes 'ab', 'az' becomes 'ba', 'a9' becomes 'b0'. "+
				"Go has no such operation.",
			"If the value is really a counter, keep it in an int. If it is really a "+
				"label sequence, the helper reproduces Perl's rule.")
		l.emit(assign("=", []ir.Expr{target}, []ir.Expr{out}))
		return target
	}
	if typeOrAny(target).Kind == ir.Any {
		// Go steps numbers, and this one's type is not known until the program
		// runs, so the step is written as arithmetic through a conversion.
		op := "+"
		if n.Op == "--" {
			op = "-"
		}
		st := assign("=", []ir.Expr{target},
			[]ir.Expr{ir.Bin(op, l.toFloat(target, n.X), ir.FloatLit("1"), ir.TFloat)})
		l.note(st, "++ works on numbers, and this value's type did not resolve, so "+
			"the step goes through a conversion. Giving the container a concrete "+
			"element type turns this back into a plain ++.",
			"explicit-conversions-no-coercion")
		l.emit(st)
		return target
	}
	l.emit(&ir.IncDec{X: target, Dec: step == "--"})
	if n.Postfix {
		l.approximate(n, "P2G3520", "post-increment used for its value",
			"++ is a statement in Go, not an expression",
			"Perl's $i++ yields the value before the increment and can appear inside a "+
				"larger expression. Go's ++ is a statement, so it cannot.",
			"The increment was moved to its own line before this expression, which "+
				"changes the order if the surrounding expression also reads the variable.")
	}
	return target
}

// definedExpr lowers `defined EXPR`.
//
// This is one of the places where Go and Perl genuinely do not line up. Perl
// distinguishes "has no value" from "has the value 0 or the empty string"; a
// Go variable of type string is always a string, and the closest thing to
// undef is the zero value.
func (l *Lowerer) definedExpr(x ast.Expr, at ast.Node) ir.Expr {
	switch n := x.(type) {
	case *ast.HashIndex:
		m, key, _ := l.hashParts(n)
		if m != nil {
			name := l.tmp("ok")
			val := l.tmp("_")
			st := assign(":=", []ir.Expr{ir.NewIdent(val, nil), ir.NewIdent(name, ir.TBool)},
				[]ir.Expr{indexComma(m, key, elemOf(typeOrAny(m)))})
			l.note(st, "Reading a missing key from a Go map gives the value type's zero "+
				"value, with no way to tell it apart from a stored zero. The two-result "+
				"form of the index expression reports whether the key was there.",
				"comma-ok-idiom")
			l.emit(st)
			return ir.NewIdent(name, ir.TBool)
		}
	case *ast.Index:
		if base, idx, elem := l.indexParts(n); base != nil {
			_ = elem
			return ir.Bin("<", idx, lenOf(base), ir.TBool)
		}
	}

	e := l.expr(x)
	t := typeOrAny(e)
	switch t.Kind {
	case ir.Any, ir.Pointer, ir.Slice, ir.Map, ir.Error:
		return ir.Bin("!=", e, ir.Nil(t), ir.TBool)
	}
	out := ir.Bin("!=", e, zeroOf(t), ir.TBool)
	l.approximate(at, "P2G2110", "defined",
		"defined has no exact Go equivalent for this value",
		"Perl's undef is a value a scalar can hold, distinct from 0 and from the "+
			"empty string. A Go variable of type "+t.String()+" always holds a "+
			t.String()+"; the nearest thing to undef is the zero value, and a variable "+
			"that was never assigned is indistinguishable from one assigned that zero.",
		"If the distinction matters here, change the variable to a pointer type "+
			"(*"+t.String()+"), where nil really does mean absent. The generated code "+
			"tests against the zero value instead.",
		"nil-vs-undef", "static-types-and-zero-values")
	return out
}

// cond lowers an expression used as a condition, where Go insists on a bool.
func (l *Lowerer) cond(e ast.Expr) ir.Expr {
	switch n := e.(type) {
	case nil:
		return ir.BoolLit(true)
	case *ast.List:
		// A test is a scalar context, where parentheses around one thing are
		// only parentheses. Treating them as a one-element list would ask
		// whether that list is empty, which it never is.
		if len(n.Elems) == 1 {
			return l.cond(n.Elems[0])
		}
	case *ast.BinOp:
		switch n.Op {
		case "&&", "and":
			return ir.Bin("&&", l.cond(n.L), l.cond(n.R), ir.TBool)
		case "||", "or":
			return ir.Bin("||", l.cond(n.L), l.cond(n.R), ir.TBool)
		}
	case *ast.UnOp:
		switch n.Op {
		case "!", "not":
			return negated(l.cond(n.X))
		case "defined":
			return l.definedExpr(n.X, n)
		}
	case *ast.Call:
		if n.Name == "exists" && len(n.Args) == 1 {
			return l.existsExpr(n.Args[0], n)
		}
		if n.Name == "defined" && len(n.Args) == 1 {
			return l.definedExpr(n.Args[0], n)
		}
	}
	return l.toBool(l.expr(e), e)
}

// ternary lowers COND ? A : B.
//
// Go has no conditional operator. The Go answer is an if statement, and the
// language designers left it out on purpose, so the generated form is the
// idiomatic one rather than a workaround.
func (l *Lowerer) ternary(n *ast.Ternary) ir.Expr {
	a := l.expr(n.A)
	b := l.expr(n.B)
	t := join(typeOrAny(a), typeOrAny(b))

	name := l.tmp(defaultName(n.A))
	decl := &ir.DeclStmt{Names: []string{name}, Type: t}
	target := ir.NewIdent(name, t)
	stmt := &ir.If{
		Cond: l.cond(n.Cond),
		Then: &ir.Block{Stmts: []ir.Stmt{assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(a, t, n.A)})}},
		Else: &ir.Block{Stmts: []ir.Stmt{assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(b, t, n.B)})}},
	}
	l.setProv(decl, n)
	l.note(decl, "Go has no ?: operator. It was left out deliberately, and the "+
		"replacement is an if statement assigning to a variable declared just above it.",
		"var-vs-short-declaration")
	l.emit(decl)
	l.emit(stmt)
	return ir.NewIdent(name, t)
}

// evalForEffect lowers an expression whose value is discarded.
func (l *Lowerer) evalForEffect(e ast.Expr) {
	for _, st := range l.exprStatement(e) {
		l.emit(st)
	}
}

// quote is strconv.Quote, kept short because it appears everywhere.
func quote(s string) string { return strconv.Quote(s) }
