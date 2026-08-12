package lower

import (
	"strconv"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// binop lowers a binary operator.
//
// Perl decides what an operator means from the operator itself, not from its
// operands: + is always numeric and . is always textual, whatever the values
// happen to be. Go decides from the operands and refuses to guess. So every
// operator here first coerces its sides into the shape the Go operator needs,
// and the coercion is exactly the lesson the annotated output carries.
func (l *Lowerer) binop(n *ast.BinOp) ir.Expr {
	if x, ok := l.overloadedOperator(n); ok {
		return x
	}
	switch n.Op {
	case "+", "-", "*":
		lx, rx, t := l.numPair(n.L, n.R)
		return ir.Bin(n.Op, lx, rx, t)

	case "/":
		if l.integerPragma {
			// `use integer` is in force, so the division truncates towards
			// zero, which is what Go's / on two ints already does.
			out := ir.Bin("/", l.toInt(l.scalar(n.L), n.L), l.toInt(l.scalar(n.R), n.R), ir.TInt)
			l.note(out, "`use integer` is in force here, so this division truncates "+
				"towards zero rather than producing a fraction. Go's / on two int "+
				"values is that division, so no conversion is needed and none appears.",
				"explicit-conversions-no-coercion")
			return out
		}
		lx := l.toFloat(l.scalar(n.L), n.L)
		rx := l.toFloat(l.scalar(n.R), n.R)
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
		lx := l.toStr(l.scalar(n.L), n.L)
		rx := l.toStr(l.scalar(n.R), n.R)
		return ir.Bin("+", lx, rx, ir.TString)

	case "x":
		return l.repeatOp(n)

	case "==", "!=", "<", ">", "<=", ">=":
		// == on two references compares their addresses: it is Perl's
		// identity test, `$m == $survivor` asking whether two variables hold
		// the same object. Reading the operands as numbers would compare two
		// zeros and answer yes for every pair.
		if n.Op == "==" || n.Op == "!=" {
			lp, rp := l.peek(n.L), l.peek(n.R)
			lt, rt := typeOrAny(lp), typeOrAny(rp)
			identity := func(t *ir.Type) bool {
				return t.Kind == ir.Pointer || (t.Kind == ir.Named && l.classOf(t) != nil)
			}
			if identity(lt) && identity(rt) {
				out := ir.Bin(n.Op, l.scalar(n.L), l.scalar(n.R), ir.TBool)
				l.note(out, "Perl's == on two references compares addresses, which is "+
					"the identity question: are these the same object. Go's == on two "+
					"pointers asks exactly that.",
					"pointers-vs-references")
				return out
			}
		}
		lx, rx, _ := l.numPair(n.L, n.R)
		return ir.Bin(n.Op, lx, rx, ir.TBool)

	case "eq", "ne", "lt", "gt", "le", "ge":
		goOp := map[string]string{"eq": "==", "ne": "!=", "lt": "<", "gt": ">", "le": "<=", "ge": ">="}[n.Op]
		lx := l.toStr(l.scalar(n.L), n.L)
		rx := l.toStr(l.scalar(n.R), n.R)
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
		lx := l.toStr(l.scalar(n.L), n.L)
		rx := l.toStr(l.scalar(n.R), n.R)
		return call("strings", "strings", "Compare", ir.TInt, lx, rx)

	case "&&", "and":
		return l.andValue(n, false)

	case "||", "or":
		return l.orValue(n, false, false)

	case "//":
		return l.orValue(n, true, false)

	case "xor":
		return ir.Bin("!=", l.cond(n.L), l.cond(n.R), ir.TBool)

	case "&", "|", "^", "<<", ">>":
		lx := l.toInt(l.scalar(n.L), n.L)
		rx := l.toInt(l.scalar(n.R), n.R)
		return ir.Bin(n.Op, lx, rx, ir.TInt)

	case "..", "...":
		// A range whose ends are conditions is the scalar-context flip-flop
		// whichever path reached it: nothing numeric could sit in a range
		// between two matches.
		if x, ok := l.flipFlopExpr(n); ok {
			return x
		}
		return l.rangeExpr(n)

	case ",", "=>":
		parts, t := l.listParts([]ast.Expr{n})
		return l.listValue(parts, t)

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

// flipFlopExpr lowers the scalar-context range operator, which is not a
// range at all: it is a toggle with hidden per-occurrence state, on from the
// evaluation where its left condition first holds to the one where its right
// condition holds. The state becomes a package-level variable declared for
// this occurrence alone, and the operator becomes a method call on it, so
// what Perl kept invisible is a named thing the reader can find.
//
// It fires only when a side is a match, which is the shape the operator is
// actually written in; a range between two numbers stays a range.
func (l *Lowerer) flipFlopExpr(n *ast.BinOp) (ir.Expr, bool) {
	if n.Op != ".." && n.Op != "..." {
		return nil, false
	}
	isMatch := func(e ast.Expr) bool {
		_, ok := e.(*ast.Match)
		return ok
	}
	if !isMatch(n.L) && !isMatch(n.R) {
		return nil, false
	}

	line, col := posOf(n)
	key := "flipflop@" + itoa(line) + ":" + itoa(col)
	b := l.special(key, '$', "span", n)
	t := ir.NamedType(l.use("flipFlop"), "")
	b.Type = t
	method := "next"
	if n.Op == "..." {
		method = "nextWait"
	}
	out := ir.CallOf(selector(ir.NewIdent(b.Go, t), method, nil), ir.TString,
		l.cond(n.L), l.cond(n.R))
	l.note(out, "The scalar range operator is a flip-flop: a toggle that stays on "+
		"between the line matching its left side and the line matching its right, "+
		"with state that belongs to this occurrence of the operator itself. Go has "+
		"no expression with a memory, so the state is the package variable this "+
		"method is called on, one per occurrence.",
		"context-is-gone", "statements-vs-expressions")
	l.approximate(n, "P2G3560", "the scalar range flip-flop",
		"the operator's hidden state became a package variable",
		"In scalar context `..` is a stateful flip-flop, not a range: it turns on "+
			"at the left condition, counts evaluations, and turns off after the right "+
			"one, keeping that state invisibly per occurrence. The emitted code keeps "+
			"the same state in a declared variable and the same sequence values, "+
			"\"1\" up to the final one with \"E0\" appended.",
		"Where the toggle guarded a block of lines, a plain bool you set and clear "+
			"reads better than the operator ever did.",
		"context-is-gone")
	return out, true
}

// numPair lowers both sides of an arithmetic operator and agrees on a result
// type: int when both sides are integral, float64 otherwise.
//
// Both sides are scalar context, which is what makes `($n - $m) / $d` divide a
// number rather than a one-element list, and `@items + 0` count the items.
func (l *Lowerer) numPair(a, b ast.Expr) (ir.Expr, ir.Expr, *ir.Type) {
	lx := l.scalar(a)
	rx := l.scalar(b)
	if l.integerPragma {
		// `use integer` takes both operands as whole numbers before the
		// operator runs, so 7.9 + 0.2 is 7 rather than 8.1. It is not only
		// the result that is truncated.
		return l.toInt(lx, a), l.toInt(rx, b), ir.TInt
	}
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

// typeOr answers with a fallback where a type was never worked out.
func typeOr(t, fallback *ir.Type) *ir.Type {
	if t == nil {
		return fallback
	}
	return t
}

// isNilLit reports whether an expression is a bare nil. Go's short
// declaration form cannot infer a type from one, so a declaration
// initialised with nothing has to say what it holds.
func isNilLit(x ir.Expr) bool {
	lit, ok := x.(*ir.Lit)
	return ok && lit.Kind == ir.LitNil
}

// modulo lowers %, which is one of the genuine semantic traps.
func (l *Lowerer) modulo(n *ast.BinOp) ir.Expr {
	lx := l.toInt(l.scalar(n.L), n.L)
	rx := l.toInt(l.scalar(n.R), n.R)
	if l.integerPragma {
		// `use integer` puts C's rule in force, where the remainder takes its
		// sign from the left operand. That is Go's rule too, so % is % here
		// and the helper that reproduces Perl's rule is not wanted.
		out := ir.Bin("%", lx, rx, ir.TInt)
		l.note(out, "`use integer` is in force here, so % takes its sign from the "+
			"left operand, which is C's rule and Go's. Outside this scope Perl takes "+
			"the sign from the right operand instead, and the two disagree on every "+
			"negative operand.",
			"explicit-conversions-no-coercion")
		return out
	}
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
	lx := l.scalar(n.L)
	rx := l.scalar(n.R)
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
	// A count below zero repeats nothing in Perl and panics in Go, and a
	// count worked out from a width or a remaining-space calculation goes
	// negative more often than it looks like it will.
	count := atLeastZero(l.toInt(l.scalar(n.R), n.R))
	if isListish(n.L) {
		src := l.list(n.L)
		out := l.helperCall(hRepeatList, typeOrAny(src), src, count)
		l.note(out, "The x operator repeats a list when its left side is a list. Go "+
			"has no such operator, so a small generic helper does the appending.")
		return out
	}
	s := l.toStr(l.expr(n.L), n.L)
	out := call("strings", "strings", "Repeat", ir.TString, s, count)
	if _, folded := count.(*ir.Lit); folded {
		l.note(out, "strings.Repeat is Go's x operator for text.")
	} else {
		l.note(out, "strings.Repeat is Go's x operator for text, with one difference "+
			"that bites: a negative count repeats nothing in Perl and panics here. The "+
			"max keeps the Perl behaviour, and it is worth a moment to ask whether a "+
			"negative count means the calculation above went wrong.")
	}
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
//
// one says the surrounding expression wants a single value, Perl's scalar
// context, which the operands inherit: `(sort @a)[-1] // ”` reads the last
// element, not a one-element list.
func (l *Lowerer) orValue(n *ast.BinOp, definedOr, one bool) ir.Expr {
	ev := l.expr
	if one {
		ev = l.scalar
	}
	if !definedOr {
		if out, ok := l.orderingChain(n); ok {
			return out
		}
	}
	if definedOr {
		return l.definedOrChain(n, one)
	}
	// `my $x = f() || die ...` never reads a value from the right side: the
	// die is what happens when there is no value. Lowering it as a value
	// would run it before the test, so it becomes the body of the guard.
	if sts, diverts := l.divertingStmts(n.R); diverts {
		lx := ev(n.L)
		t := typeOrAny(lx)
		name := l.tmp(defaultName(n.L))
		decl := &ir.DeclStmt{Names: []string{name}, Type: t, Values: []ir.Expr{lx}}
		l.setProv(decl, n)
		target := ir.NewIdent(name, t)
		guard := &ir.If{Cond: negated(l.toBool(target, n.L)), Then: &ir.Block{Stmts: sts}}
		l.note(decl, "Perl's || hands back the first true operand, and the right side "+
			"here never produces one: it is what runs when the left side is false. "+
			"Written as an if, the order of events is on the page.",
			"static-types-and-zero-values")
		l.emit(decl)
		l.emit(guard)
		return target
	}
	lx := ev(n.L)
	// The right side only runs when the left side is false, so any statements
	// it needs are held back and placed inside the guard.
	rx, rxPre := l.diverted(func() ir.Expr { return ev(n.R) })
	t := join(typeOrAny(lx), typeOrAny(rx))
	// Evidence about one variable treats `any` as an absence of information,
	// because a later observation can still settle it. The two sides of a
	// default are not that: both are values this expression can hand back, so
	// a side whose type did not resolve is a value of unknown type, and the
	// slot has to hold it as well as the other side.
	//
	// It only matters where the other side is a function, because that is the
	// case where the wrong answer does not compile: the unresolved side would
	// be asserted into a signature invented from the fallback's body, and the
	// caller would then be checked against a signature the real value never
	// had. `$x || 0` keeps its int, which is what the reader wants to see.
	if lt, rt := typeOrAny(lx), typeOrAny(rx); (isDynamic(lt) && rt.Kind == ir.Func) ||
		(isDynamic(rt) && lt.Kind == ir.Func) {
		t = ir.TAny
	}

	// `$x || 0` over a number is an identity: the fallback is the one value
	// the test fires on. Emitting the guard produced `if v == 0 { v = 0 }`,
	// which a Go reader can only stare at.
	if zeroDefault(t, rx) && len(rxPre) == 0 && typeOrAny(lx).Kind == t.Kind {
		out := l.assignable(lx, t, n.L)
		l.note(out, "The || here defaulted to the value a false operand already is: "+
			"a Perl number is false exactly when it is zero, so `|| 0` changes "+
			"nothing and the Go reads the value directly.",
			"static-types-and-zero-values")
		return out
	}

	name := l.tmp(defaultName(n.L))
	decl := &ir.DeclStmt{Names: []string{name}, Type: t, Values: []ir.Expr{l.assignable(lx, t, n.L)}}
	l.setProv(decl, n)
	target := ir.NewIdent(name, t)
	body := &ir.Block{Stmts: append(rxPre, assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(rx, t, n.R)}))}
	guard := &ir.If{Cond: negated(l.toBool(target, n.L)), Then: body}
	l.note(decl, "Perl's || returns the first true operand, not a bool, which is "+
		"why it works for defaulting. Go's || is strictly boolean, so the default "+
		"is applied with an if.",
		"static-types-and-zero-values")
	l.emit(decl)
	l.emit(guard)
	return ir.NewIdent(name, t)
}

// zeroDefault reports whether a fallback is the number zero applied to a
// numeric slot, which makes the whole default a no-op.
func zeroDefault(t *ir.Type, rx ir.Expr) bool {
	if t == nil || (t.Kind != ir.Int && t.Kind != ir.Float) {
		return false
	}
	lit, ok := rx.(*ir.Lit)
	if !ok {
		return false
	}
	switch lit.Value {
	case "0", "0.0", "0.", ".0":
		return true
	}
	return false
}

// definedProbe is one operand of a // chain: how to read it, how to ask whether
// it had a value at all, and the value itself once it did.
type definedProbe struct {
	init ir.Stmt // read the operand into a name the test and the value share
	cond ir.Expr // the test that says it had a value
	val  ir.Expr // what the chain answers with when the test passes
	// last marks an operand that always has a value, so nothing after it in
	// the chain can ever be reached.
	last bool
	// note explains this rung, and is attached where the rung is built.
	note string
}

// definedOrChain lowers a whole run of //, which is what the operator is nearly
// always written as: `$opt{a} // $opt{b} // 5`.
//
// Lowering one // at a time cannot get this right, because the value of
// `$a // $b` is itself either a value or undef and a Go expression has nowhere
// to put that second answer. Taking the run as a unit lets each operand keep
// its own way of asking the question, which for a hash element is the
// two-result index form and never a test against zero: a stored 0 is defined.
func (l *Lowerer) definedOrChain(n *ast.BinOp, one bool) ir.Expr {
	ev := l.expr
	if one {
		ev = l.scalar
	}
	operands := flattenDefinedOr(n)
	var probes []definedProbe
	var fallback ir.Expr
	var fallbackPre []ir.Stmt
	var fallbackStmts []ir.Stmt
	for i, x := range operands {
		if i == len(operands)-1 {
			// `$h{k} // die ...` reads no value from its last operand: the
			// die is what happens when nothing else answered. It has to run
			// inside the ladder's final else, never before the ladder.
			if sts, diverts := l.divertingStmts(x); diverts {
				fallbackStmts = sts
				break
			}
			fallback, fallbackPre = l.diverted(func() ir.Expr { return ev(x) })
			break
		}
		p := l.definedProbeOf(x, n, one)
		if p.last {
			// This operand always has a value, so nothing after it in the
			// chain can be reached and it becomes the answer.
			fallback = p.val
			l.note(fallback, "This operand always has a value, so // can never reach "+
				"what follows it and the rest of the chain is dropped. Perl's undef is a "+
				"state a Go variable of this type does not have.",
				"nil-vs-undef", "static-types-and-zero-values")
			break
		}
		probes = append(probes, p)
	}
	return l.buildDefinedOr(probes, fallback, fallbackPre, fallbackStmts, n)
}

// divertingStmts recognises an operand that can never yield a value because
// it leaves instead: a die or an exit. It comes back as the statements to run
// at the point the chain would have read it.
func (l *Lowerer) divertingStmts(e ast.Expr) ([]ir.Stmt, bool) {
	c, ok := e.(*ast.Call)
	if !ok || (c.Name != "die" && c.Name != "exit") {
		return nil, false
	}
	return l.statementForm(c), true
}

// buildDefinedOr writes the ladder out: one if per operand that might be
// undefined, and the last operand as the else that always runs. A last
// operand that leaves instead of answering, a die or an exit, arrives as
// fallbackStmts and becomes the body of that else.
func (l *Lowerer) buildDefinedOr(probes []definedProbe, fallback ir.Expr, fallbackPre, fallbackStmts []ir.Stmt, n *ast.BinOp) ir.Expr {
	if len(probes) == 0 {
		if fallbackStmts != nil {
			// Nothing before the die could miss, so the chain is just it.
			for _, st := range fallbackStmts {
				l.emit(st)
			}
			return ir.Nil(ir.TAny)
		}
		for _, st := range fallbackPre {
			l.emit(st)
		}
		return fallback
	}
	types := make([]*ir.Type, 0, len(probes)+1)
	for _, p := range probes {
		types = append(types, typeOrAny(p.val))
	}
	if fallbackStmts == nil {
		types = append(types, typeOrAny(fallback))
	}
	t := joinAll(types)
	if t == nil || isUnresolved(t) {
		t = ir.TAny
	}

	name := l.tmp(defaultName(n.L))
	decl := &ir.DeclStmt{Names: []string{name}, Type: t}
	l.setProv(decl, n)
	l.note(decl, "// asks whether the left side is defined, not whether it is true, "+
		"so a stored 0 or empty string stops the chain. Go has no undefined value, so "+
		"each step asks the question in the form that type allows and the answers meet "+
		"in one variable.",
		"nil-vs-undef", "comma-ok-idiom")
	target := ir.NewIdent(name, t)

	base := append(fallbackPre, assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(fallback, t, n.R)}))
	if fallbackStmts != nil {
		base = fallbackStmts
	}
	var chain ir.Stmt = &ir.Block{Stmts: base}
	for i := len(probes) - 1; i >= 0; i-- {
		p := probes[i]
		step := &ir.If{
			Init: p.init,
			Cond: p.cond,
			Then: &ir.Block{Stmts: []ir.Stmt{
				assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(p.val, t, n.L)}),
			}},
			Else: chain,
		}
		if p.note != "" {
			l.note(step, p.note, "nil-vs-undef", "comma-ok-idiom")
		}
		chain = step
	}
	l.setProv(chain, n)
	l.emit(decl)
	l.emit(chain)
	return ir.NewIdent(name, t)
}

// flattenDefinedOr returns the operands of a run of //, left to right. Perl's
// // is left-associative, so `a // b // c` nests to the left.
func flattenDefinedOr(n *ast.BinOp) []ast.Expr {
	if inner, ok := n.L.(*ast.BinOp); ok && inner.Op == "//" {
		return append(flattenDefinedOr(inner), n.R)
	}
	return []ast.Expr{n.L, n.R}
}

// definedProbeOf works out how to ask whether one operand of a // chain has a
// value, choosing the exact question wherever the type allows one.
func (l *Lowerer) definedProbeOf(x ast.Expr, at ast.Node, one bool) definedProbe {
	ev := l.expr
	if one {
		ev = l.scalar
	}
	// A literal, or arithmetic on literals, cannot be undef.
	if definiteValue(x) {
		return definedProbe{val: ev(x), last: true}
	}
	if _, ok := l.alwaysDefined(x, at); ok {
		return definedProbe{val: ev(x), last: true}
	}
	if h, ok := x.(*ast.HashIndex); ok {
		m, key, elem, field := l.hashPartsField(h)
		if m != nil && key != nil && field == nil {
			if isNullable(elem) {
				name := l.tmp(defaultName(h))
				p := ir.NewIdent(name, elem)
				return definedProbe{
					init: assign(":=", []ir.Expr{p}, []ir.Expr{index(m, key, elem)}),
					cond: ir.Bin("!=", p, ir.Nil(elem), ir.TBool),
					val:  ir.Un("*", p, elem.Elem),
					note: "The map has room for undef, so nil answers both halves of the " +
						"question at once: the key was missing, or it held nothing.",
				}
			}
			name := l.tmp(defaultName(h))
			okName := l.tmp("ok")
			v := ir.NewIdent(name, elem)
			return definedProbe{
				init: assign(":=", []ir.Expr{v, ir.NewIdent(okName, ir.TBool)},
					[]ir.Expr{indexComma(m, key, elem)}),
				cond: ir.NewIdent(okName, ir.TBool),
				val:  v,
				note: "Reading a missing key from a Go map gives the value type's zero " +
					"value, which a stored zero is indistinguishable from. The two-result " +
					"form of the index expression is the only way to ask which happened, " +
					"and it is the question // is asking.",
			}
		}
	}
	lowered := ev(x)
	t := typeOrAny(lowered)
	if isNullable(t) {
		name := l.tmp(defaultName(x))
		p := ir.NewIdent(name, t)
		return definedProbe{
			init: assign(":=", []ir.Expr{p}, []ir.Expr{lowered}),
			cond: ir.Bin("!=", p, ir.Nil(t), ir.TBool),
			val:  ir.Un("*", p, t.Elem),
		}
	}
	switch t.Kind {
	case ir.Any, ir.Pointer, ir.Slice, ir.Map, ir.Error:
		name := l.tmp(defaultName(x))
		v := ir.NewIdent(name, t)
		return definedProbe{
			init: assign(":=", []ir.Expr{v}, []ir.Expr{lowered}),
			cond: ir.Bin("!=", v, ir.Nil(t), ir.TBool),
			val:  v,
		}
	}
	// Nothing exact is available: the value has a concrete Go type, so the
	// nearest question is whether it holds that type's zero value.
	name := l.tmp(defaultName(x))
	v := ir.NewIdent(name, t)
	l.approximate(at, "P2G2110", "//",
		"defined has no exact Go equivalent for this value",
		"Perl's undef is a value a scalar can hold, distinct from 0 and from the "+
			"empty string. A Go variable of type "+t.String()+" always holds a "+
			t.String()+", so the test here asks whether it holds the zero value and "+
			"a stored zero takes the default where the original kept it.",
		"Where the difference matters, declare the variable as *"+t.String()+
			", which has nil for absent and a value for everything else.",
		"nil-vs-undef", "static-types-and-zero-values")
	return definedProbe{
		init: assign(":=", []ir.Expr{v}, []ir.Expr{lowered}),
		cond: ir.Bin("!=", v, zeroOf(t), ir.TBool),
		val:  v,
	}
}

// andValue lowers `&&` used for its value rather than as a test.
//
// Perl's && answers with the first false operand or, when both are true, with
// the second one, which is why `$name && uc $name` is a defaulting idiom and
// not a yes-or-no. Go's && is strictly boolean, so where the two sides are not
// booleans the choice is written out.
func (l *Lowerer) andValue(n *ast.BinOp, one bool) ir.Expr {
	ev := l.expr
	if one {
		ev = l.scalar
	}
	lx := ev(n.L)
	// The right side only runs when the left side is true, so any statements
	// it needs are held back and placed inside the guard.
	rx, rxPre := l.diverted(func() ir.Expr { return ev(n.R) })
	t := join(typeOrAny(lx), typeOrAny(rx))
	if t == nil || t.Kind == ir.Bool || t.Kind == ir.Any {
		// Two tests, or two values with nothing in common to hold the answer
		// in. The boolean form is both correct and the one a reader expects.
		for _, st := range rxPre {
			l.emit(st)
		}
		return ir.Bin("&&", l.toBool(lx, n.L), l.toBool(rx, n.R), ir.TBool)
	}
	name := l.tmp(defaultName(n.L))
	decl := &ir.DeclStmt{Names: []string{name}, Type: t, Values: []ir.Expr{l.assignable(lx, t, n.L)}}
	target := ir.NewIdent(name, t)
	guard := &ir.If{
		Cond: l.toBool(target, n.L),
		Then: &ir.Block{Stmts: append(rxPre, assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(rx, t, n.R)}))},
	}
	l.setProv(decl, n)
	l.note(decl, "Perl's && answers with an operand rather than with true or false: "+
		"the first false one, or the second when both are true. Go's && is strictly "+
		"boolean, so the choice is written out, which is also where the "+
		"short-circuit becomes visible.",
		"static-types-and-zero-values")
	l.emit(decl)
	l.emit(guard)
	return target
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
	case "||", "or":
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
		x := l.scalar(n.X)
		if isNum(typeOrAny(x)) {
			return ir.Un("-", x, x.Type())
		}
		return ir.Un("-", l.toFloat(x, n.X), ir.TFloat)
	case "+":
		return l.expr(n.X)
	case "~":
		return ir.Un("^", l.toInt(l.scalar(n.X), n.X), ir.TInt)
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
	// Counting through a slot says the slot holds a number, whether the step
	// is a statement or is being read for its value. `grep { !$seen{$_}++ }`
	// is the second shape, and it is the commonest counting idiom there is:
	// without this the hash it counts into is left holding `any`.
	l.observeTargetValue(n.X, ir.TInt)
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
				"looks like a word, carrying from one position into the next: 'aa' "+
				"becomes 'ab', 'az' carries to 'ba', 'a9' carries to 'b0', and 'Zz' "+
				"carries all the way out to 'AAa'. Go has no such operation.",
			"If the value is really a counter, keep it in an int. If it is really a "+
				"label sequence, the helper reproduces Perl's rule.")
		// The value may have been read through a helper, which is not a
		// place Go can store into; the store needs the element itself.
		store := target
		if _, isCall := target.(*ir.Call); isCall {
			if at := l.assignTarget(n.X); at != nil {
				store = at
			}
		}
		l.emit(assign("=", []ir.Expr{store}, []ir.Expr{out}))
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
		if n.Postfix {
			before := l.tmp("before")
			hold := assign(":=", []ir.Expr{ir.NewIdent(before, ir.TAny)}, []ir.Expr{target})
			l.setProv(hold, n)
			l.emit(hold)
			l.emit(st)
			return ir.NewIdent(before, ir.TAny)
		}
		l.emit(st)
		return target
	}
	if n.Postfix {
		// The value of a post-step is what the variable held before it, so
		// that is named before the step happens. Stepping first and reading
		// the variable afterwards would give the value after, which is what
		// the pre-step form means and a different answer entirely.
		before := l.tmp("before")
		t := typeOrAny(target)
		hold := assign(":=", []ir.Expr{ir.NewIdent(before, t)}, []ir.Expr{target})
		l.setProv(hold, n)
		l.note(hold, "Go's ++ and -- are statements, not expressions, so a step used "+
			"for its value has to be written out. The old value is named first "+
			"because that is what the post-step form yields.",
			"statements-vs-expressions")
		l.emit(hold)
		l.emit(&ir.IncDec{X: target, Dec: step == "--"})
		l.approximate(n, "P2G3520", "post-increment used for its value",
			"++ is a statement in Go, not an expression",
			"Perl's $i++ yields the value before the increment and can appear inside "+
				"a larger expression. Go's ++ is a statement, so the step is written on "+
				"its own line and the old value is named.",
			"Where the surrounding expression also writes the variable, check the "+
				"order: the step now happens before the rest of the line rather than "+
				"in the middle of it.",
			"statements-vs-expressions")
		return ir.NewIdent(before, t)
	}
	l.emit(&ir.IncDec{X: target, Dec: step == "--"})
	return target
}

// definedExpr lowers `defined EXPR`.
//
// This is one of the places where Go and Perl genuinely do not line up. Perl
// distinguishes "has no value" from "has the value 0 or the empty string"; a
// Go variable of type string is always a string, and the closest thing to
// undef is the zero value.
func (l *Lowerer) definedExpr(x ast.Expr, at ast.Node) ir.Expr {
	if out, ok := l.optionGiven(x, at); ok {
		return out
	}
	if out, ok := l.alwaysDefined(x, at); ok {
		return out
	}
	switch n := x.(type) {
	case *ast.HashIndex:
		m, key, elem, field := l.hashPartsField(n)
		if m != nil && key != nil && isNullable(elem) {
			// The map has room for undef, so the question has an exact
			// answer: a missing key and a stored undef are both nil, which
			// is what `defined` asks and what `exists` does not.
			out := ir.Bin("!=", index(m, key, elem), ir.Nil(elem), ir.TBool)
			l.note(out, "This map holds pointers because the program stored undef in "+
				"it, and that makes `defined` an exact question in Go: nil means the key "+
				"was missing or held nothing, and anything else means a value is there. "+
				"`exists` is the different question, and it is the two-result index form.",
				"nil-vs-undef", "comma-ok-idiom")
			return out
		}
		if m != nil && key == nil {
			// A struct field or a constructor parameter: there is no key to
			// ask about, so the question becomes the zero-value test.
			_ = field
			return l.definedValue(m, at)
		}
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
			inRange := ir.Bin("<", idx, lenOf(base), ir.TBool)
			if text, neg := negativeLiteral(n.Idx); neg {
				// A negative Perl index counts back from the end, so being in
				// range means the array is at least that long.
				inRange = ir.Bin(">=", lenOf(base), ir.IntLit(text), ir.TBool)
				idx = ir.Bin("-", lenOf(base), ir.IntLit(text), ir.TInt)
			}
			if !isNullable(elem) {
				return inRange
			}
			// The slice has room for undef, so being in range is not the
			// whole answer: a slot inside the slice can still hold nothing.
			out := ir.Bin("&&", inRange,
				ir.Bin("!=", index(base, idx, elem), ir.Nil(elem), ir.TBool), ir.TBool)
			l.note(out, "Two things can make a Perl array element undefined: the index "+
				"is past the end, or the slot itself holds undef. Go needs both tests, "+
				"and the order matters because the second one would panic on its own.",
				"nil-vs-undef", "slices-not-arrays")
			return out
		}
	}

	return l.definedValue(l.expr(x), at)
}

// alwaysDefined answers `defined` for a variable that can never hold nothing.
//
// A scalar declared with an initialiser and never assigned undef always has a
// value, in Perl as much as in Go, so the question has one answer and it can
// be written in. This matters because the fallback is a zero-value test, and
// that test says a variable holding 0 or the empty string is undefined, which
// is a different and wrong answer rather than an approximate one.
func (l *Lowerer) alwaysDefined(x ast.Expr, at ast.Node) (ir.Expr, bool) {
	v, ok := x.(*ast.Var)
	if !ok || v.Sigil != '$' {
		return nil, false
	}
	b, found := l.scope.lookup(varKey('$', v.Name))
	if !found || !b.Definite || b.Kind == KindParam {
		return nil, false
	}
	switch typeOrAny(l.identFor(b)).Kind {
	case ir.Int, ir.Float, ir.String, ir.Bool:
	default:
		return nil, false
	}
	out := ir.BoolLit(true)
	l.note(out, "This variable is given a value where it is declared and never set "+
		"to undef, so it always has one and the test has a single answer. Testing "+
		"against the zero value instead would call a variable holding 0 or the empty "+
		"string undefined, which is a different question.",
		"nil-vs-undef", "static-types-and-zero-values")
	return out, true
}

// definedValue answers `defined` for a value whose type is already known.
func (l *Lowerer) definedValue(e ir.Expr, at ast.Node) ir.Expr {
	// An untyped nil is undef written out, and Go will not compare one with
	// anything, so the answer is written in instead.
	if lit, ok := e.(*ir.Lit); ok && lit.Kind == ir.LitNil {
		out := ir.BoolLit(false)
		l.note(out, "This is undef itself rather than a variable that might hold it, "+
			"so the answer is no and it is written in.",
			"nil-vs-undef")
		return out
	}
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
func (l *Lowerer) ternary(n *ast.Ternary, one bool) ir.Expr {
	ev := l.expr
	if one {
		ev = l.scalar
	}
	// Two function literals as the arms of a ternary share one slot, exactly
	// as two in a list or a table do, so they take the uniform closure
	// signature: `my $cmp = $by_count ? sub {...} : sub {...}` is one variable
	// and Go gives it one type.
	saved := l.uniformFn
	l.uniformFn = l.uniformFn || closureList([]ast.Expr{n.A, n.B})
	// Perl evaluates the condition and then one arm, never both. Each arm's
	// side effects are held back and placed inside its branch, so
	// `@ARGV ? shift @ARGV : 3` moves the array only when the test passes,
	// and only after the test has looked at it.
	cond := l.cond(n.Cond)
	a, aPre := l.diverted(func() ir.Expr { return ev(n.A) })
	b, bPre := l.diverted(func() ir.Expr { return ev(n.B) })
	l.uniformFn = saved
	t := join(typeOrAny(a), typeOrAny(b))

	name := l.tmp(defaultName(n.A))
	decl := &ir.DeclStmt{Names: []string{name}, Type: t}
	target := ir.NewIdent(name, t)
	stmt := &ir.If{
		Cond: cond,
		Then: &ir.Block{Stmts: append(aPre, assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(a, t, n.A)}))},
		Else: &ir.Block{Stmts: append(bPre, assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(b, t, n.B)}))},
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

// overloadedOperator turns down an operator that `use overload` made a method
// call on one of its operands.
//
// Nothing about it can be translated as an operator: Go has no way to give a
// named type its own +, and applying the built-in one to the struct would not
// compile even if it were the right thing to do.
func (l *Lowerer) overloadedOperator(n *ast.BinOp) (ir.Expr, bool) {
	var overloader *Class
	for _, side := range []ast.Expr{n.L, n.R} {
		c := l.classOf(typeOrAny(l.peek(side)))
		if c != nil && c.Overloads[n.Op] {
			overloader = c
			break
		}
	}
	if overloader == nil {
		return nil, false
	}
	return l.todoExpr(n, "P2G7025", "overloaded "+n.Op,
		"this operator is a method call on "+overloader.Perl,
		"`use overload` in "+overloader.Perl+" gave `"+n.Op+"` its own subroutine, so "+
			"this line calls that subroutine rather than doing arithmetic. Go decides "+
			"what an operator means from the operand types alone and has no hook to "+
			"install one.",
		"Call the method by name. `"+overloader.Go+"` has the same subroutine on it, "+
			"so `a.Add(b)` says what `a + b` was doing and reads the same to anyone who "+
			"has not seen the overload declaration.",
		"methods-and-receivers"), true
}

// peek lowers an expression only far enough to ask what type it has, for the
// shapes where doing so has no side effect. Anything else answers nothing.
func (l *Lowerer) peek(e ast.Expr) ir.Expr {
	switch n := e.(type) {
	case *ast.Var:
		if n.Sigil == '$' {
			if b, ok := l.scope.lookup(varKey('$', n.Name)); ok {
				return l.identFor(b)
			}
			if b, ok := l.globalSeen[varKey('$', n.Name)]; ok {
				return l.identFor(b)
			}
		}
	}
	return nil
}

// markIndefinite records that a scalar was given a value that may be undef,
// which makes a later `defined` test a real question again.
func (l *Lowerer) markIndefinite(e ast.Expr) {
	v, ok := e.(*ast.Var)
	if !ok || v.Sigil != '$' {
		return
	}
	if b, found := l.scope.lookup(varKey('$', v.Name)); found {
		b.Definite = false
	}
}

// definiteValue reports whether an expression can be relied on to produce a
// value rather than undef.
//
// The list is deliberately short. A hash read, a capture group, a sub call and
// a list assignment can all leave undef behind, so none of them is here, and
// anything not recognised is treated as though it could.
func definiteValue(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.NumberLit, *ast.StrLit, *ast.InterpLit, *ast.QwLit:
		return true
	case *ast.UnOp:
		switch n.Op {
		case "-", "+", "!", "not":
			return definiteValue(n.X)
		}
	case *ast.BinOp:
		switch n.Op {
		case "+", "-", "*", "/", "%", "**", ".", "x",
			"==", "!=", "<", ">", "<=", ">=",
			"eq", "ne", "lt", "gt", "le", "ge", "<=>", "cmp":
			return true
		case "//", "||", "or":
			// A default chain is worth exactly as much as its last term.
			return definiteValue(n.R)
		}
	case *ast.Ternary:
		return definiteValue(n.A) && definiteValue(n.B)
	case *ast.Call:
		switch n.Name {
		case "length", "scalar", "int", "abs", "sqrt", "uc", "lc", "ucfirst",
			"lcfirst", "join", "sprintf", "time", "index", "rindex", "keys",
			"values", "exists", "defined", "ord", "chr", "hex", "oct":
			return true
		}
	}
	return false
}
