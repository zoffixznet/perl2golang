package lower

import (
	"strconv"
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// mayVanish reports whether a statement can honestly lower to nothing at all.
// A sub or package declaration is lowered elsewhere; a use has its effect at
// compile time; a declaration lowers to nothing when its variables were
// hoisted to package level; and an expression with no effect, such as the
// bare `1;` that ends a module, has nothing worth keeping.
func mayVanish(st ast.Stmt) bool {
	switch n := st.(type) {
	case nil, *ast.SubDecl, *ast.PackageDecl, *ast.Use:
		return true
	case *ast.ExprStmt:
		if _, isDecl := n.X.(*ast.My); isDecl {
			return true
		}
		return effectFree(n.X)
	}
	return false
}

// effectFree reports whether evaluating an expression for effect and keeping
// nothing could change anything: literals and bare variable reads cannot.
func effectFree(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.NumberLit, *ast.StrLit, *ast.QwLit, *ast.Var:
		return true
	case *ast.List:
		for _, part := range n.Elems {
			if !effectFree(part) {
				return false
			}
		}
		return true
	}
	return false
}

// stmts lowers a statement list, flushing whatever setup each statement needed.
func (l *Lowerer) stmts(list []ast.Stmt) []ir.Stmt {
	var out []ir.Stmt
	for _, st := range list {
		savedPre := l.pre
		l.pre = nil
		// The name $! resolves to is whatever the call on this line put it
		// in, and that variable is only in scope for this statement.
		savedErr := l.errVar
		l.errVar = ""

		lead := leadComments(st)
		entriesBefore := len(l.rep.Entries)
		body := l.stmt(st)
		body = l.applyDestroyPlan(st, body)
		pre := l.takePre()
		l.pre = savedPre
		l.errVar = savedErr

		// The safety net under every lowering rule: a statement that produced
		// no code and no diagnostic has simply vanished, and a vanished
		// statement is a silent change of meaning, the one wrong this tool
		// promises never to commit. Any lowering path that gives up must
		// either emit something or say something; when one does neither, the
		// statement is marked here rather than dropped.
		if l.pass == 2 && len(pre) == 0 && len(body) == 0 &&
			len(l.rep.Entries) == entriesBefore && !mayVanish(st) {
			body = []ir.Stmt{l.todoStmt(st, "P2G3598", "statement",
				"this statement's behaviour is missing",
				"This statement lowered to nothing: no Go came out of it, and no other diagnostic explains why.",
				"Translate the statement by hand. The original is quoted above the marker.")}
		}

		if len(lead) > 0 {
			out = append(out, &ir.CommentStmt{Lines: lead})
		}
		out = append(out, pre...)
		out = append(out, body...)
	}
	return out
}

// block lowers a statement list in its own lexical scope.
//
// Perl's separator variables are restored here too. `local $/` puts the old
// value back when the block ends, and since what the separators say is folded
// into the calls inside the block, the restore is a matter of the converter
// forgetting it on the way out.
func (l *Lowerer) block(list []ast.Stmt) *ir.Block {
	saved := l.scope
	savedSeps := l.seps
	// `use integer` is lexical: it changes what / and % mean until the end of
	// the enclosing block and no further, which is exactly what makes the
	// pragma usable at all.
	savedInteger := l.integerPragma
	// The submatch slice a match inside this block declared is a local of the
	// Go block, so nothing after the block can name it. Perl scopes its
	// capture variables to the enclosing block too, restoring the previous
	// match's values on the way out, so dropping the frame here is both what
	// compiles and what the original meant.
	depth := l.captureDepth()
	l.scope = newScope(saved)
	stmts := l.stmts(list)
	// An object declared in this block dies at its closing brace, and the
	// call that says so belongs inside it.
	stmts = append(stmts, l.flushDestroys(l.scope)...)
	b := l.markUnused(&ir.Block{Stmts: stmts})
	l.restoreCaptures(depth)
	l.scope = saved
	l.seps = savedSeps
	l.integerPragma = savedInteger
	return b
}

// leadComments extracts the developer's own comments above a statement. They
// are carried into both output variants, because the developer wrote them and
// they are usually the best documentation in the file.
func leadComments(st ast.Stmt) []string {
	var c *ast.StmtComments
	switch n := st.(type) {
	case *ast.ExprStmt:
		c = &n.StmtComments
	case *ast.If:
		c = &n.StmtComments
	case *ast.While:
		c = &n.StmtComments
	case *ast.ForC:
		c = &n.StmtComments
	case *ast.Foreach:
		c = &n.StmtComments
	case *ast.Block:
		c = &n.StmtComments
	case *ast.SubDecl:
		c = &n.StmtComments
	case *ast.PackageDecl:
		c = &n.StmtComments
	case *ast.Use:
		c = &n.StmtComments
	case *ast.Return:
		c = &n.StmtComments
	case *ast.LoopCtl:
		c = &n.StmtComments
	case *ast.Untranslated:
		c = &n.StmtComments
	}
	if c == nil {
		return nil
	}
	var out []string
	for _, cm := range c.Lead {
		if cm.Pod {
			continue
		}
		raw := strings.TrimSpace(cm.Text)
		// The shebang is an instruction to the operating system about which
		// interpreter to run, not a comment about the program. A compiled
		// binary has no use for it.
		if strings.HasPrefix(raw, "#!") {
			continue
		}
		out = append(out, strings.TrimSpace(strings.TrimPrefix(raw, "#")))
	}
	return out
}

// stmt lowers one statement.
func (l *Lowerer) stmt(st ast.Stmt) []ir.Stmt {
	if l.pass == 2 {
		l.rep.Stats.Statements++
	}
	savedStmt := l.curStmt
	l.curStmt = st
	defer func() { l.curStmt = savedStmt }()
	switch n := st.(type) {
	case nil:
		return nil
	case *ast.ExprStmt:
		return l.exprStatement(n.X)
	case *ast.If:
		return []ir.Stmt{l.ifStmt(n)}
	case *ast.While:
		return l.whileStmt(n)
	case *ast.ForC:
		return l.forStmt(n)
	case *ast.Foreach:
		return l.foreachStmt(n)
	case *ast.Block:
		// A Perl bare block is a loop that runs once, which is why last and
		// next work inside it. When the body uses them, the Go form has to be
		// a real loop or the branch has nothing to leave.
		if usesLoopControl(n.Body) {
			body := l.block(n.Body)
			body.Stmts = append(body.Stmts, &ir.Branch{Kind: "break"})
			out := &ir.For{Body: body, Label: l.label(n.Label)}
			l.setProv(out, n)
			l.note(out, "A bare block in Perl is a loop that runs exactly once, which "+
				"is why last and next work inside one. Go has no such block, so it "+
				"becomes a for loop that breaks at the bottom.")
			return []ir.Stmt{out}
		}
		out := &ir.BlockStmt{Body: l.block(n.Body)}
		l.setProv(out, n)
		l.note(out, "A bare block in Perl exists to scope its variables. Go blocks "+
			"do the same, and this one is kept so the variables inside stay local.")
		return []ir.Stmt{out}
	case *ast.SubDecl:
		l.lowerSubDecl(n)
		return nil
	case *ast.Use:
		return l.useStmt(n)
	case *ast.Return:
		return l.returnStmt(n)
	case *ast.LoopCtl:
		return l.loopCtl(n)
	case *ast.PackageDecl:
		return l.packageStmt(n)
	case *ast.Untranslated:
		return []ir.Stmt{l.todoStmt(n, "P2G1514", "unparsed statement",
			"this statement was not understood",
			"The parser could not read this statement: "+n.Reason+".",
			"Translate it by hand. The original is quoted above it.")}
	}
	return []ir.Stmt{l.todoStmt(st, "P2G3599", "statement",
		"this statement is not implemented",
		"The converter has no rule for this kind of statement.",
		"Translate it by hand.")}
}

// usesLoopControl reports whether a statement list branches out of its
// enclosing loop, not counting loops nested inside it, which have their own.
func usesLoopControl(body []ast.Stmt) bool {
	for _, st := range body {
		switch n := st.(type) {
		case *ast.LoopCtl:
			return true
		case *ast.If:
			if usesLoopControl(n.Then) || usesLoopControl(n.Else) {
				return true
			}
			for _, ei := range n.ElseIfs {
				if usesLoopControl(ei.Then) {
					return true
				}
			}
		case *ast.Block:
			if usesLoopControl(n.Body) {
				return true
			}
		}
	}
	return false
}

// exprStatement lowers an expression evaluated for effect.
func (l *Lowerer) exprStatement(e ast.Expr) []ir.Stmt {
	switch n := e.(type) {
	case nil:
		return nil

	case *ast.Assign:
		return l.assignStmts(n)

	case *ast.My:
		return l.declarationOnly(n)

	case *ast.List:
		// A parenthesised list evaluated for effect is each of its elements
		// evaluated for effect, which is what `$i++, $j--` means.
		var out []ir.Stmt
		for _, part := range n.Elems {
			out = append(out, l.exprStatement(part)...)
		}
		return out

	case *ast.UnOp:
		if n.Op == "++" || n.Op == "--" {
			return l.incDecStmt(n)
		}

	case *ast.BinOp:
		switch n.Op {
		case ",":
			var out []ir.Stmt
			for _, part := range flatten(n) {
				out = append(out, l.exprStatement(part)...)
			}
			return out
		case "or", "||":
			// `open(...) or die "..."` deserves the real Go shape rather than a
			// negated truth test, because error handling is the thing a Perl
			// developer most needs to see written out.
			if c, isCall := n.L.(*ast.Call); isCall {
				switch c.Name {
				case "open":
					if sts, ok := l.openGuarded(c, n.R); ok {
						return sts
					}
				case "close":
					if sts, ok := l.closeGuarded(c, n.R); ok {
						return sts
					}
				case "opendir":
					if sts, ok := l.opendirGuarded(c, n.R); ok {
						return sts
					}
				case "seek":
					if sts, ok := l.seekGuarded(c, n.R); ok {
						return sts
					}
				case "closedir":
					// The directory was read in one call, so there is no
					// handle left to close and nothing that can fail.
					return l.closedirCall(c)
				}
			}
			// `my ($a, $b) = /re/ or next` assigns and then tests, which
			// works in Perl because a list assignment in boolean context
			// reports how many values were on its right.
			if a, isAssign := n.L.(*ast.Assign); isAssign {
				if cond, ok := l.assignCond(a); ok {
					guard := &ir.If{Cond: negated(cond), Then: &ir.Block{Stmts: l.exprStatement(n.R)}}
					l.setProv(guard, n)
					l.note(guard, "A list assignment in Perl is also a test: in boolean "+
						"context it reports how many values were on its right, so a match "+
						"that found nothing is false. Go separates the two, so the "+
						"assignment happens and then the result is tested.",
						"comma-ok-idiom")
					return []ir.Stmt{guard}
				}
			}
			// The `something() or die "..."` idiom: a guard, not a value.
			cond := negated(l.cond(n.L))
			l.countGuardRead(n.L)
			guard := &ir.If{
				Cond: cond,
				Then: &ir.Block{Stmts: l.exprStatement(n.R)},
			}
			l.setProv(guard, n)
			l.note(guard, "Perl's `X or die` reads as a sentence and works because or "+
				"short-circuits. Go has no statement form of it: the test is written as "+
				"an if, which is the same shape every error check in Go takes.",
				"errors-are-values", "if-err-nil-rhythm")
			return []ir.Stmt{guard}
		case "and", "&&":
			cond := l.cond(n.L)
			l.countGuardRead(n.L)
			guard := &ir.If{Cond: cond, Then: &ir.Block{Stmts: l.exprStatement(n.R)}}
			l.setProv(guard, n)
			return []ir.Stmt{guard}
		}

	case *ast.Call:
		// `... or next` puts a loop control word in expression position,
		// where the parser cannot tell it from a call to a sub of that name.
		if sts, ok := l.loopCtlCall(n); ok {
			return sts
		}
		// `EXPR or do { ... }` runs the block for its effect, so there is no
		// value to carry out of it and the statements stand as they are.
		if n.Name == "do" && n.Block != nil {
			return l.doBlockStmts(n)
		}
		return l.callStatement(n)

	case *ast.Subst:
		return l.substStmt(n)

	case *ast.Trans:
		return l.transStmt(n)
	}

	x := l.expr(e)
	if x == nil {
		return nil
	}
	// Go allows only a call in statement position. Anything else evaluated
	// for effect has already had its effect through the setup statements
	// queued in front of it, and the value itself is discarded.
	if _, isCall := x.(*ir.Call); !isCall {
		return nil
	}
	st := exprStmt(x)
	l.setProv(st, e)
	return []ir.Stmt{st}
}

// declarationOnly lowers `my $x;` and `my ($a, $b);` with no initialiser.
func (l *Lowerer) declarationOnly(n *ast.My) []ir.Stmt {
	// `local X;` with no value on the right is a localisation to undef, not a
	// declaration. It is the shape `local $/;` takes, which is how a Perl
	// program says "read everything".
	if n.Keyword == "local" {
		return l.localStmts(localTargets(n), nil, n)
	}
	var out []ir.Stmt
	for _, v := range declaredVars(n) {
		b := l.declare(v, KindLocal)
		// An array declared with nothing in it starts empty, which is a
		// length the converter knows exactly.
		l.noteLength(b, 0)
		if b.Kind == KindGlobal {
			// Hoisted to package level, where the declaration already
			// stands. Repeating it here would not compile.
			continue
		}
		var st ir.Stmt = &ir.DeclStmt{Names: []string{b.Go}, Type: b.Type}
		if b.Type != nil && b.Type.Kind == ir.Map {
			// A map needs making, and the short form says so in one line
			// without repeating the type.
			st = assign(":=", []ir.Expr{ir.NewIdent(b.Go, b.Type)},
				[]ir.Expr{composite(b.Type, nil, nil)})
			l.note(st, "A Go map has to be made before anything can be written to it. "+
				"A declared but unmade map is nil, and writing to a nil map panics, "+
				"which is one of the first surprises coming from Perl.",
				"nil-slices-vs-nil-maps")
		} else {
			l.note(st, "A Perl variable starts out undef. A Go variable starts at its "+
				"type's zero value: 0, the empty string, false, or nil, depending on the "+
				"type. There is no separate undefined state.",
				"static-types-and-zero-values", "nil-vs-undef")
		}
		l.setProv(st, v)
		out = append(out, st)
		out = append(out, l.discardIfUnused(b)...)
	}
	return out
}

// incDecStmt lowers ++ and -- in statement position, which is where Go allows
// them.
func (l *Lowerer) incDecStmt(n *ast.UnOp) []ir.Stmt {
	// Incrementing $h{k} says the values are numbers, not that the hash itself
	// is one, which is what makes a counting hash come out as map[string]int
	// rather than map[string]any. The same holds however deep the counter is:
	// $h{a}{b}++ says the outer hash holds hashes of numbers.
	l.observeTargetValue(n.X, ir.TInt)
	l.autovivifyTarget(n.X)
	target := l.assignTarget(n.X)
	if target == nil {
		return []ir.Stmt{exprStmt(l.expr(n))}
	}
	if typeOrAny(target).Kind == ir.String && n.Op == "++" {
		// The step itself is the statement; the value it produces is not
		// wanted, and a bare identifier is not a statement in Go.
		savedPre := l.pre
		l.pre = nil
		l.incDecExpr(n)
		out := l.takePre()
		l.pre = savedPre
		return out
	}
	if t := typeOrAny(target); isNullable(t) {
		// The slot can be absent, so it holds a pointer and Go has no ++ for
		// one. undef counts as 0 in Perl's arithmetic, which is exactly what
		// reading a nil pointer as the zero value gives.
		step := "+"
		if n.Op == "--" {
			step = "-"
		}
		st := assign("=", []ir.Expr{target}, []ir.Expr{
			l.helperCall(hPtr, t, ir.Bin(step, l.helperCall(hDeref, t.Elem, target),
				ir.IntLit("1"), t.Elem)),
		})
		l.setProv(st, n)
		l.note(st, "This element can be absent, so it holds a pointer. Counting through "+
			"one means reading the value behind it, stepping that, and storing a new "+
			"pointer: a missing slot counts as zero, exactly as undef did.",
			"nil-vs-undef", "pointers-vs-references")
		return []ir.Stmt{st}
	}
	if typeOrAny(target).Kind == ir.Any {
		// Go's ++ needs a numeric type, and this one is not known until the
		// program runs, so the step is written as arithmetic instead.
		step := "+"
		if n.Op == "--" {
			step = "-"
		}
		st := assign("=", []ir.Expr{target},
			[]ir.Expr{ir.Bin(step, l.toFloat(target, n.X), ir.FloatLit("1"), ir.TFloat)})
		l.setProv(st, n)
		l.note(st, "++ works on numbers, and this variable's type did not resolve, so "+
			"the step goes through a conversion. Declaring the variable as an int "+
			"turns this back into a plain ++.",
			"explicit-conversions-no-coercion")
		return []ir.Stmt{st}
	}
	st := &ir.IncDec{X: target, Dec: n.Op == "--"}
	l.setProv(st, n)
	if _, isHash := n.X.(*ast.HashIndex); isHash {
		l.note(st, "Incrementing a missing map key works because reading a missing key "+
			"gives the value type's zero value: counters need no initialisation, exactly "+
			"as in Perl. The map itself still has to exist.",
			"nil-slices-vs-nil-maps")
	}
	return []ir.Stmt{st}
}

// ifStmt lowers if/elsif/else and unless.
func (l *Lowerer) ifStmt(n *ast.If) ir.Stmt {
	depth := l.captureDepth()
	keepCaptures := false
	defer func() {
		if !keepCaptures {
			l.restoreCaptures(depth)
		}
	}()

	cond := l.cond(n.Cond)
	if n.Unless {
		cond = negated(cond)
	}
	// The condition may have needed setup; it belongs before the if.
	setup := l.takePre()

	out := &ir.If{Cond: cond, Then: l.block(n.Then)}
	l.setProv(out, n)

	// Build the elsif chain from the back so each one nests in the previous.
	var tail ir.Stmt
	if len(n.Else) > 0 {
		tail = l.block(n.Else)
	}
	for i := len(n.ElseIfs) - 1; i >= 0; i-- {
		ei := n.ElseIfs[i]
		// An elsif condition that needs setup has to run that setup only when
		// the branches above it have failed. Leaving it in front of the whole
		// statement would evaluate every condition in the chain, which for a
		// scanning match means every alternative consumes input before the
		// first one is even tested.
		saved := l.pre
		l.pre = nil
		cond := l.cond(ei.Cond)
		branchSetup := l.takePre()
		l.pre = saved

		branch := &ir.If{Cond: cond, Then: l.block(ei.Then)}
		branch.Else = tail
		if len(branchSetup) > 0 {
			if len(branchSetup) == 1 {
				if a, ok := branchSetup[0].(*ir.Assign); ok && a.Op == ":=" {
					branch.Init = a
					tail = branch
					continue
				}
			}
			blk := &ir.Block{Stmts: append(branchSetup, branch)}
			l.note(blk, "The test in this branch needs a step of its own, and that step "+
				"belongs inside the else: running it in front of the whole chain would "+
				"run it even when an earlier branch had already been taken.")
			tail = blk
			continue
		}
		tail = branch
	}
	out.Else = tail

	if n.Unless {
		l.note(out, "unless is if with the condition negated. Go has no unless, and "+
			"negating at the top is the closest reading of the original.")
	}
	// A guard leaves the block only when its condition says so, so whatever
	// the condition bound is in scope for everything after it. `next unless
	// $line =~ /re/;` followed by $1 is the shape this exists for: scoping the
	// submatch slice to the if would leave every capture after it empty.
	if guardsTheRest(n, out) && l.captureDepth() > depth {
		keepCaptures = true
		// Putting the setup back leaves it to the enclosing statement list,
		// which emits it in front of this statement rather than inside it.
		l.pre = append(setup, l.pre...)
		return out
	}

	if len(setup) == 1 {
		// A single setup statement fits in the if's own init clause, which is
		// the Go idiom for scoping a value to the branch that uses it.
		if a, ok := setup[0].(*ir.Assign); ok && a.Op == ":=" {
			out.Init = a
			return out
		}
	}
	if len(setup) > 0 {
		return &ir.BlockStmt{Body: &ir.Block{Stmts: append(setup, out)}}
	}
	return out
}

// guardsTheRest reports whether an if is the early-exit shape: no else, and a
// body that does nothing but leave.
func guardsTheRest(n *ast.If, out *ir.If) bool {
	if len(n.ElseIfs) > 0 || out.Else != nil || out.Then == nil || len(out.Then.Stmts) == 0 {
		return false
	}
	// What makes it a guard is that control leaves, not that nothing else
	// happens on the way out. `unless (/re/) { $bad++; next }` is the same
	// shape as a bare next, and the lines below it are just as unreachable
	// when it fires.
	switch out.Then.Stmts[len(out.Then.Stmts)-1].(type) {
	case *ir.Branch, *ir.Return:
		return true
	}
	return false
}

// whileStmt lowers while, until, and do-while.
func (l *Lowerer) whileStmt(n *ast.While) []ir.Stmt {
	if n.DoWhile {
		return l.doWhile(n)
	}
	if st, ok := l.readLoop(n); ok {
		return st
	}
	if st, ok := l.eachLoop(n); ok {
		return st
	}
	if st, ok := l.drainLoop(n); ok {
		return st
	}

	depth := l.captureDepth()
	defer l.restoreCaptures(depth)

	// The conjuncts of an `and` are lowered one at a time, each keeping its
	// own setup, because Perl's and stops at the first false: a read in the
	// second conjunct must not run when the first has already failed.
	conjs := andChain(n.Cond)
	var conds []ir.Expr
	var pres [][]ir.Stmt
	for _, cj := range conjs {
		savedPre := l.pre
		l.pre = nil
		conds = append(conds, l.cond(cj))
		pres = append(pres, l.takePre())
		l.pre = savedPre
	}
	laterSetup := false
	for _, p := range pres[1:] {
		if len(p) > 0 {
			laterSetup = true
		}
	}
	if laterSetup && !n.Until {
		body, label := l.loopBody(n.Body, l.label(n.Label))
		var lead []ir.Stmt
		for i, c := range conds {
			lead = append(lead, pres[i]...)
			lead = append(lead, &ir.If{
				Cond: negated(c),
				Then: &ir.Block{Stmts: []ir.Stmt{&ir.Branch{Kind: "break"}}},
			})
		}
		out := &ir.For{Body: &ir.Block{Stmts: append(lead, body.Stmts...)}, Label: label}
		l.setProv(out, n)
		l.note(out, "The loop's condition does work of its own, a read, in a later "+
			"conjunct, and Perl's `and` stops at the first false. Each test gets its "+
			"own break so the work only happens when everything before it held, which "+
			"is the order the original ran in.",
			"range-is-not-foreach", "statements-vs-expressions")
		return []ir.Stmt{out}
	}

	cond := conds[0]
	for _, c := range conds[1:] {
		cond = ir.Bin("&&", cond, c, ir.TBool)
	}
	if n.Until {
		cond = negated(cond)
	}
	var setup []ir.Stmt
	for _, p := range pres {
		setup = append(setup, p...)
	}

	body, label := l.loopBody(n.Body, l.label(n.Label))
	out := &ir.For{Cond: cond, Body: body, Label: label}
	l.setProv(out, n)
	l.note(out, "Go has one loop keyword. `for cond { }` is its while, `for { }` is "+
		"its infinite loop, and there is no do-while and no until.",
		"range-is-not-foreach")

	if len(setup) > 0 {
		// The condition needs work on every iteration, so the setup has to be
		// inside the loop with an explicit break.
		body := out.Body
		out.Cond = nil
		out.Body = &ir.Block{Stmts: append(append(setup, &ir.If{
			Cond: negated(cond),
			Then: &ir.Block{Stmts: []ir.Stmt{&ir.Branch{Kind: "break"}}},
		}), body.Stmts...)}
	}
	return []ir.Stmt{out}
}

// andChain flattens a condition's top-level conjunction into its operands,
// in evaluation order.
func andChain(e ast.Expr) []ast.Expr {
	if n, ok := e.(*ast.BinOp); ok && (n.Op == "and" || n.Op == "&&") {
		return append(andChain(n.L), andChain(n.R)...)
	}
	return []ast.Expr{e}
}

// drainLoop lowers `while (defined(my $x = shift @queue))`, which is how Perl
// empties a list one element at a time.
//
// The test is about whether there was an element to take, not about what it
// was, so a queue holding a 0 or an empty string keeps going. Lowering the
// pieces separately loses exactly that: shift on an empty Go slice has to hand
// back something, whatever it hands back is the element type's zero value, and
// the defined test then reads as a truth test. Taking the loop as a whole
// gives the shape a Go developer writes, where the length is the condition.
func (l *Lowerer) drainLoop(n *ast.While) ([]ir.Stmt, bool) {
	if n.Until || n.DoWhile {
		return nil, false
	}
	declNode, arr, front, ok := drainShape(n.Cond)
	if !ok {
		return nil, false
	}

	saved := l.scope
	l.scope = newScope(saved)
	defer func() { l.scope = saved }()

	b := l.lookup('@', arr.Name, arr)
	b.Reads++
	b.Writes++
	list := l.identFor(b)
	elem := elemOf(typeOrAny(list))

	var item *Binding
	switch t := declNode.(type) {
	case *ast.My:
		vars := declaredVars(t)
		if len(vars) != 1 || vars[0].Sigil != '$' {
			return nil, false
		}
		item = l.declare(vars[0], KindLocal)
	case *ast.Var:
		if t.Sigil != '$' {
			return nil, false
		}
		item = l.lookup('$', t.Name, t)
	default:
		return nil, false
	}
	item.Writes++
	l.observe(item, elem)
	if l.pass == 2 {
		elem = item.Type
	}

	var pick, rest ir.Expr
	if front {
		pick = index(list, ir.IntLit("0"), elem)
		rest = slicing(list, ir.IntLit("1"), nil, typeOrAny(list))
	} else {
		last := ir.Bin("-", lenOf(list), ir.IntLit("1"), ir.TInt)
		pick = index(list, last, elem)
		rest = slicing(list, nil, last, typeOrAny(list))
	}
	take := assign(":=", []ir.Expr{ir.NewIdent(item.Go, elem)}, []ir.Expr{pick})
	shrink := assign("=", []ir.Expr{l.writeTarget(b)}, []ir.Expr{rest})

	body, label := l.loopBody(n.Body, l.label(n.Label))
	body.Stmts = append([]ir.Stmt{take, shrink}, body.Stmts...)

	out := &ir.For{Cond: ir.Bin(">", lenOf(list), ir.IntLit("0"), ir.TBool), Body: body, Label: label}
	l.setProv(out, n)
	l.note(out, "The loop asks whether there was an element to take, not whether the "+
		"element was true, which is why a queue holding a 0 does not stop it early. "+
		"Go asks that question with the length, and taking the element is then two "+
		"plain statements at the top of the body.",
		"slices-not-arrays", "nil-vs-undef")
	return []ir.Stmt{out}, true
}

// drainShape reads `defined(my $x = shift @a)` and its relatives, reporting
// what is being declared, which array is being emptied, and from which end.
func drainShape(cond ast.Expr) (declNode ast.Expr, arr *ast.Var, front, ok bool) {
	var inner ast.Expr
	switch c := cond.(type) {
	case *ast.Call:
		if c.Name != "defined" || len(c.Args) != 1 {
			return nil, nil, false, false
		}
		inner = c.Args[0]
	case *ast.UnOp:
		if c.Op != "defined" {
			return nil, nil, false, false
		}
		inner = c.X
	default:
		return nil, nil, false, false
	}
	for {
		p, isParen := inner.(*ast.List)
		if !isParen || len(p.Elems) != 1 {
			break
		}
		inner = p.Elems[0]
	}
	a, isAssign := inner.(*ast.Assign)
	if !isAssign || a.Op != "=" {
		return nil, nil, false, false
	}
	call, isCall := a.RHS.(*ast.Call)
	if !isCall || len(call.Args) != 1 {
		return nil, nil, false, false
	}
	switch call.Name {
	case "shift":
		front = true
	case "pop":
	default:
		return nil, nil, false, false
	}
	src := flatten(call.Args[0])
	if len(src) != 1 {
		return nil, nil, false, false
	}
	v, isVar := src[0].(*ast.Var)
	if !isVar || v.Sigil != '@' || v.Name == "_" {
		return nil, nil, false, false
	}
	return a.LHS, v, front, true
}

// doWhile lowers `do { ... } while COND`, which Go has no form of.
func (l *Lowerer) doWhile(n *ast.While) []ir.Stmt {
	cond := l.cond(n.Cond)
	if n.Until {
		cond = negated(cond)
	}
	body := l.block(n.Body)
	body.Stmts = append(body.Stmts, &ir.If{
		Cond: negated(cond),
		Then: &ir.Block{Stmts: []ir.Stmt{&ir.Branch{Kind: "break"}}},
	})
	out := &ir.For{Body: body, Label: l.label(n.Label)}
	l.setProv(out, n)
	l.note(out, "Go has no do-while. The body-first shape is written as an "+
		"unconditional for loop that breaks at the bottom, which runs the body at "+
		"least once exactly as the original did.")
	return []ir.Stmt{out}
}

// forStmt lowers a C-style for loop, which Go writes almost identically.
func (l *Lowerer) forStmt(n *ast.ForC) []ir.Stmt {
	saved := l.scope
	l.scope = newScope(saved)
	defer func() { l.scope = saved }()

	var init ir.Stmt
	if n.Init != nil {
		sts := l.exprStatement(n.Init)
		if len(sts) == 1 {
			init = sts[0]
		} else if len(sts) > 1 {
			for _, s := range sts[:len(sts)-1] {
				l.emit(s)
			}
			init = sts[len(sts)-1]
		}
	}
	var cond ir.Expr
	if n.Cond != nil {
		cond = l.cond(n.Cond)
	}
	var post ir.Stmt
	var extraPost []ir.Stmt
	if n.Post != nil {
		sts := l.exprStatement(n.Post)
		if len(sts) > 0 {
			post = sts[0]
			extraPost = sts[1:]
		}
		if len(extraPost) > 0 {
			l.approximate(n, "P2G3530", "for loop with several post expressions",
				"only one post expression fits a Go for header",
				"Perl allows a comma expression in the third slot of a C-style for, so "+
					"a loop can advance two counters at once. Go allows exactly one "+
					"statement there.",
				"The extra expressions run at the end of the loop body instead, which "+
					"behaves the same unless the body uses next, because next skips them "+
					"and the header would not have.")
		}
	}

	body, forLabel := l.loopBody(n.Body, l.label(n.Label))
	body.Stmts = append(body.Stmts, extraPost...)
	out := &ir.For{Init: init, Cond: cond, Post: post, Body: body, Label: forLabel}
	l.setProv(out, n)
	l.note(out, "A C-style for loop carries over almost unchanged. Go drops the "+
		"parentheses around the header and requires the braces.")
	return []ir.Stmt{out}
}

// packageStmt handles a package declaration.
func (l *Lowerer) packageStmt(n *ast.PackageDecl) []ir.Stmt {
	if n.Name == "main" || n.Name == l.curPkg {
		return nil
	}
	if c, ok := l.classes[n.Name]; ok && c.IsType {
		// The file's packages were partitioned before lowering began, so the
		// declaration has already done its work and there is nothing to run.
		return nil
	}
	// A package declaration does nothing when the program runs; it changes
	// where the subs after it belong. There is no step to stand in for, so the
	// marker goes in on its own and the statements around it still run.
	return []ir.Stmt{l.todoDecl(n, "P2G7010", "package "+n.Name,
		"this package declaration is somewhere the converter could not follow",
		"Perl decides the current package while it compiles the file, so a package "+
			"statement inside a loop or a conditional still applies to everything "+
			"written after it. The converter partitions a file's packages block by "+
			"block, and this one is nested somewhere it could not follow.",
		"Move the package statement to file scope, which is where a script normally "+
			"declares a class.",
		"packages-and-exported-names", "methods-and-receivers")}
}

// assignCond lowers a list assignment used as a test, yielding the truth of
// the list it assigned.
//
// Only a match has a truth worth reporting: everything else on the right of a
// list assignment has a length the converter already knows, so testing it
// would be testing a constant.
func (l *Lowerer) assignCond(a *ast.Assign) (ir.Expr, bool) {
	if a.Op != "=" {
		return nil, false
	}
	switch a.LHS.(type) {
	case *ast.My, *ast.List:
	default:
		return nil, false
	}
	if _, isMatch := a.RHS.(*ast.Match); !isMatch {
		return nil, false
	}
	depth := l.captureDepth()
	for _, st := range l.assignStmts(a) {
		l.emit(st)
	}
	if l.captureDepth() <= depth {
		return nil, false
	}
	frame := l.captureStack[len(l.captureStack)-1]
	out := ir.Bin("!=", ir.NewIdent(frame.Name, ir.SliceOf(ir.TString)),
		ir.Nil(ir.SliceOf(ir.TString)), ir.TBool)
	return out, true
}

// loopCtlCall recognises `next`, `last` and `redo` written where an
// expression was expected, which is what `EXPR or next` does.
func (l *Lowerer) loopCtlCall(n *ast.Call) ([]ir.Stmt, bool) {
	switch n.Name {
	case "next", "last", "redo":
	default:
		return nil, false
	}
	if l.findSubExists(n.Name) {
		return nil, false
	}
	ctl := &ast.LoopCtl{Op: n.Name}
	if len(n.Args) == 1 {
		label, ok := n.Args[0].(*ast.Call)
		if !ok || len(label.Args) > 0 {
			return nil, false
		}
		ctl.Label = label.Name
	} else if len(n.Args) > 0 {
		return nil, false
	}
	ctl.SetSpan(n.Pos(), n.End())
	return l.loopCtl(ctl), true
}

// findSubExists reports whether the file declares a sub of that name, which
// would make the word an ordinary call after all.
func (l *Lowerer) findSubExists(name string) bool {
	_, ok := l.findSub(name)
	return ok
}

// loopCtl lowers last, next, and redo.
func (l *Lowerer) loopCtl(n *ast.LoopCtl) []ir.Stmt {
	if n.Label != "" {
		if l.usedLabels == nil {
			l.usedLabels = map[string]bool{}
		}
		l.usedLabels[n.Label] = true
	}
	switch n.Op {
	case "last":
		out := &ir.Branch{Kind: "break", Label: l.outerLabel(n.Label)}
		l.setProv(out, n)
		return []ir.Stmt{out}
	case "next":
		out := &ir.Branch{Kind: "continue", Label: l.outerLabel(n.Label)}
		l.setProv(out, n)
		return []ir.Stmt{out}
	case "redo":
		if n.Label == "" && l.redoLabel != nil {
			out := &ir.Branch{Kind: "continue"}
			l.setProv(out, n)
			l.note(out, "The body is wrapped in a loop of its own so that redo has "+
				"something to continue, which re-runs it without moving on.",
				"range-is-not-foreach")
			l.approximate(n, "P2G3510", "redo",
				"the body was wrapped in a loop of its own",
				"redo restarts the iteration without advancing, and Go has no keyword "+
					"for that. The body now sits inside its own loop, which breaks at the "+
					"bottom so it runs once by default.",
				"Where the restart is really a retry, a counted inner loop reads better "+
					"than a conditional continue.",
				"range-is-not-foreach")
			return []ir.Stmt{out}
		}
	}
	return []ir.Stmt{l.todoStmt(n, "P2G3510", "redo",
		"redo has no Go equivalent",
		"redo restarts the current iteration without re-evaluating the loop "+
			"condition or the increment. Go has break and continue and nothing that "+
			"restarts an iteration.",
		"Wrap the body in an inner `for { ... }` and use continue to restart it, "+
			"with a break at the end so it runs once by default.")}
}

// returnStmt lowers `return`.
func (l *Lowerer) returnStmt(n *ast.Return) []ir.Stmt {
	// Inside the block a tree walk runs, a bare return means "nothing more
	// for this entry", and the walk reads that as a nil error.
	if l.findWalk != nil && len(n.Exprs) == 0 {
		out := &ir.Return{Results: []ir.Expr{ir.Nil(ir.TError)}}
		l.setProv(out, n)
		l.note(out, "The walk decides what to do next from what the callback returns, "+
			"so leaving early is a nil error rather than a bare return.",
			"errors-are-values")
		return []ir.Stmt{out}
	}
	s := l.curSub
	if s == nil {
		// A return at file scope ends the program.
		out := &ir.Return{}
		l.setProv(out, n)
		return []ir.Stmt{out}
	}

	if sts, ok := l.returnCaptures(n, s); ok {
		return sts
	}

	var results []ir.Expr
	var kinds []*ir.Type
	if len(n.Exprs) > 0 {
		flat := flatten(n.Exprs[0])
		if len(n.Exprs) > 1 {
			flat = nil
			for _, e := range n.Exprs {
				flat = append(flat, flatten(e)...)
			}
		}
		// `return sort keys %h` yields the list when the caller wanted a
		// list and the count when it wanted one value. A Go function has to
		// pick, and the list is the answer that keeps the information: a
		// caller who wanted the count can still take len of it, and one who
		// wanted the values cannot get them back out of a number.
		if len(flat) == 1 && l.producesList(flat[0]) {
			x := l.list(flat[0])
			results = append(results, x)
			kinds = append(kinds, typeOrAny(x))
			if l.pass == 2 {
				l.approximate(n, "P2G2121", "return of a list",
					"the list is returned, not its length",
					"A Perl sub returning a list hands back the values when the caller "+
						"wanted a list and how many there were when it wanted one value. A Go "+
						"function has one shape, and this one returns the values.",
					"Where a caller wanted the count, take len of what comes back.",
					"context-is-gone", "multiple-return-values")
			}
		} else {
			last := len(flat) - 1
			for i, e := range flat {
				// `return ($cost, @path)` hands back a flat list whose length
				// only the array knows. Go's fixed result count cannot say
				// that, but a slice as the final result can: the caller that
				// wrote `my ($cost, @path) = ...` peels the fixed results off
				// and takes the slice whole, which is the same split Perl
				// made. The array must stay a list here; reading it as one
				// value would return its count, which loses the elements for
				// good, where a caller who wanted the count can still ask the
				// slice with len.
				if i == last && l.producesList(e) {
					x := l.list(e)
					l.spliced[x] = true
					results = append(results, x)
					kinds = append(kinds, typeOrAny(x))
					if l.pass == 1 {
						s.TailSpill = true
					}
					continue
				}
				if i != last && l.producesList(e) && l.pass == 2 {
					l.approximate(n, "P2G2121", "an array in the middle of a return",
						"the array is returned as its element count",
						"Perl flattens this array into the returned list, so the "+
							"values around it shift by however many elements it holds. "+
							"A Go function returns a fixed number of values, and only a "+
							"final result can absorb a run of unknown length, so this "+
							"one is reduced to its count.",
						"Return the array as its own slice result, or move it to the "+
							"end of the list, where its elements survive whole.",
						"context-is-gone", "multiple-return-values")
				}
				// `return $self` from a method of a hierarchy returns the
				// whole object, not the embedded part the receiver names.
				if x, ok := l.receiverReturn(e); ok {
					results = append(results, x)
					kinds = append(kinds, typeOrAny(x))
					continue
				}
				// A sub that hands back a container element the file has put
				// undef into is handing back the absence too, and its Go
				// signature is the only place that can be said. Reading the
				// value out here would turn a missing key into a 0 at the
				// boundary, where nothing downstream can tell the two apart.
				if isUndefLiteral(e) && l.pass == 1 {
					if s.NilResults == nil {
						s.NilResults = map[int]bool{}
					}
					s.NilResults[i] = true
				}
				x := l.scalarKeepingNil(e)
				results = append(results, x)
				kinds = append(kinds, typeOrAny(x))
			}
		}
	}
	// A closure that shares a slot with others answers with one value of no
	// fixed type, and a return with several values folds them into one list,
	// which the caller takes apart again. Trimming to the first would throw
	// the rest away silently.
	if s != nil && s.Uniform && len(results) > 1 {
		one := l.listValue(results, ir.TAny)
		if l.pass == 2 {
			l.note(one, "This sub shares a table or a variable with other "+
				"subs, so they all answer with one value of no fixed type. A return "+
				"of several values folds them into one list, and the caller that "+
				"wrote my ($a, $b) = ... takes the list apart again.",
				"context-is-gone", "multiple-return-values")
		}
		results = []ir.Expr{one}
		kinds = []*ir.Type{ir.SliceOf(ir.TAny)}
	}

	if l.pass == 1 {
		s.ResultEvidence = append(s.ResultEvidence, kinds)
	}

	// Pad or trim to the sub's settled shape so the Go function keeps its
	// promise about how many values it returns.
	if l.pass == 2 {
		for len(results) < len(s.Results) {
			i := len(results)
			results = append(results, zeroOf(s.Results[i]))
		}
		if len(results) > len(s.Results) {
			results = results[:len(s.Results)]
		}
		for i := range results {
			results[i] = l.assignable(results[i], s.Results[i], nil)
		}
		if len(n.Exprs) == 0 && len(s.Results) > 0 {
			l.approximate(n, "P2G2120", "bare return in a sub that returns values",
				"a bare return becomes explicit zero values",
				"A bare return in Perl yields the empty list in list context and undef "+
					"in scalar context, so the caller can tell it apart from a real result. "+
					"A Go function always returns the number of values it declares.",
				"If the caller needs to tell 'no result' from a real one, add an error "+
					"or a bool result and check it, which is the Go convention.",
				"multiple-return-values", "comma-ok-idiom")
		}
	}

	out := &ir.Return{Results: results}
	l.setProv(out, n)
	return []ir.Stmt{out}
}

// todoStmt records a refusal and produces the statement that stands in for the
// construct: a call that names the missing step on stderr and returns.
//
// Skipping the statement outright would change the program's meaning with
// nothing to show for it, and the panic this used to emit was worse still. A
// panic is not reached at the point of the refusal; it is reached instead of
// everything after it. One unconvertible line near the top of a file made the
// whole of the rest of the program unreachable, including all of the code that
// did convert, which is the only part a reader can learn anything from.
func (l *Lowerer) todoStmt(n ast.Node, code, construct, short, message, advice string, concepts ...string) ir.Stmt {
	todo := l.refuse(n, code, construct, short, message, advice, concepts...)
	st := l.todoMarker(todo, n)
	st.Stub = l.helperCall(hNotImplementedHere, ir.TVoid,
		ir.Str(strconv.Quote(todo.Code)), ir.Str(strconv.Quote(todoWording(todo))))
	st.Info.Spelled = true
	return st
}

// todoDecl records a refusal for a construct that did nothing at run time, such
// as a declaration. There is no step to stand in for, so the marker is the
// whole of what is left behind.
func (l *Lowerer) todoDecl(n ast.Node, code, construct, short, message, advice string, concepts ...string) ir.Stmt {
	return l.todoMarker(l.refuse(n, code, construct, short, message, advice, concepts...), n)
}

// todoMarker builds the bare marker both forms share.
func (l *Lowerer) todoMarker(todo ir.Todo, n ast.Node) *ir.TodoStmt {
	st := &ir.TodoStmt{Info: todo}
	line, col := posOf(n)
	st.Meta.Prov = ir.Provenance{Line: line, Col: col, Text: todo.Perl}
	return st
}

// globalDecl builds the package-level declaration for a binding that outlived
// its block.
func (l *Lowerer) globalDecl(b *Binding) ir.Decl {
	d := &ir.VarDecl{Names: []string{b.Go}, Type: b.Type}
	switch {
	case b.Init != nil:
		d.Type = nil
		d.Values = []ir.Expr{b.Init}
	case b.Type != nil && b.Type.Kind == ir.Map:
		d.Values = []ir.Expr{composite(b.Type, nil, nil)}
	}
	d.Doc = []string{b.Go + " holds " + typeWords(b.Type) + "."}
	if b.Doc != "" {
		d.Doc = []string{b.Doc}
	}
	if l.pass == 2 {
		if b.Explain != "" {
			ir.Annotate(d, b.Explain)
			return d
		}
		ir.Annotate(d, "This was a package variable in the original, reachable from "+
			"every sub in the file. Go's package-level variables work the same way, "+
			"and a name starting with a lower-case letter is visible only inside this "+
			"package.",
			"packages-and-exported-names")
	}
	return d
}

// patternDecl builds the package-level compiled regular expression.
func (l *Lowerer) patternDecl(p *patternVar) ir.Decl {
	d := &ir.VarDecl{
		Names:  []string{p.Name},
		Values: []ir.Expr{call("regexp", "regexp", "MustCompile", ir.NamedType("*regexp.Regexp", "regexp"), ir.Str(quote(p.GoRegex)))},
		Doc:    []string{p.Name + " matches " + describePattern(p.Perl) + "."},
	}
	if l.pass == 2 {
		ir.Annotate(d, "Perl compiles a literal pattern once and caches it. Go makes "+
			"that explicit: regexp.MustCompile at package level compiles the pattern "+
			"when the program starts, and panics immediately if the pattern is bad, "+
			"rather than failing on the first line of input.",
			"mustcompile-pattern", "regexp-is-re2")
	}
	return d
}

// describePattern gives a package-level pattern variable a doc comment that
// says something, without pretending to explain the regex.
func describePattern(perl string) string {
	trimmed := strings.TrimSpace(perl)
	if len(trimmed) > 48 {
		trimmed = trimmed[:45] + "..."
	}
	return "the pattern " + trimmed
}

// countGuardRead records the read a guard performs on a variable its own left
// side just declared.
//
// `my $file = shift @ARGV or die "usage\n"` declares $file and then tests it,
// both on one line. The test goes through the value the assignment produced
// rather than through a second mention of the name, so nothing counted the
// read, and a declaration nothing reads is given a blank assignment and a
// comment saying so. The comment was wrong: the next line reads it.
func (l *Lowerer) countGuardRead(e ast.Expr) {
	a, ok := e.(*ast.Assign)
	if !ok {
		return
	}
	target := a.LHS
	if m, isMy := target.(*ast.My); isMy {
		if len(m.Vars) != 1 {
			return
		}
		target = m.Vars[0]
	}
	v, ok := target.(*ast.Var)
	if !ok {
		return
	}
	if b := l.lookup(v.Sigil, v.Name, v); b != nil {
		b.Reads++
	}
}

// loopBody lowers a loop's body, wrapping it in an inner loop when something
// in it restarts the iteration.
//
// Go has break and continue and nothing that re-runs an iteration without
// advancing. An inner `for { ... break }` gives redo somewhere to continue
// to, and the outer loop takes a label so that next and last still mean the
// outer one. That is the shape a Go developer writes for the same problem,
// and it is the only one that keeps all three keywords meaning what they did.
func (l *Lowerer) loopBody(body []ast.Stmt, label string) (*ir.Block, string) {
	saved := l.redoLabel
	defer func() { l.redoLabel = saved }()

	if !hasRedo(body) {
		l.redoLabel = nil
		return l.block(body), label
	}
	// The label is only needed when something else branches out of the body:
	// Go rejects a label nothing uses.
	if label == "" && hasLoopExit(body) {
		label = l.tmp("eachTurn")
	}
	l.redoLabel = &label
	blk := l.block(body)
	inner := &ir.For{Body: &ir.Block{Stmts: append(blk.Stmts, &ir.Branch{Kind: "break"})}}
	l.note(inner, "redo re-runs the body without moving on to the next element, and "+
		"Go has nothing that does that. The body sits in a loop of its own, which "+
		"redo continues and which breaks at the bottom so it runs once by default. "+
		"next and last name the outer loop, because unlabelled they would now mean "+
		"this one.",
		"range-is-not-foreach")
	return &ir.Block{Stmts: []ir.Stmt{inner}}, label
}

// hasRedo reports whether a loop body restarts itself, without looking inside
// a nested loop or a nested sub, where a redo would belong to that one.
func hasRedo(body []ast.Stmt) bool {
	return scanLoopBody(body, func(c *ast.LoopCtl) bool { return c.Op == "redo" && c.Label == "" })
}

// hasLoopExit reports whether a loop body branches out of itself, which is
// what decides whether the outer loop needs a label.
func hasLoopExit(body []ast.Stmt) bool {
	return scanLoopBody(body, func(c *ast.LoopCtl) bool {
		return (c.Op == "next" || c.Op == "last") && c.Label == ""
	})
}

// scanLoopBody looks for a loop-control statement belonging to this loop.
func scanLoopBody(body []ast.Stmt, want func(*ast.LoopCtl) bool) bool {
	for _, st := range body {
		switch n := st.(type) {
		case *ast.LoopCtl:
			if want(n) {
				return true
			}
		case *ast.If:
			if scanLoopBody(n.Then, want) || scanLoopBody(n.Else, want) {
				return true
			}
			for _, ei := range n.ElseIfs {
				if scanLoopBody(ei.Then, want) {
					return true
				}
			}
		case *ast.Block:
			if scanLoopBody(n.Body, want) {
				return true
			}
		}
	}
	return false
}
