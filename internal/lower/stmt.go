package lower

import (
	"strconv"
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// stmts lowers a statement list, flushing whatever setup each statement needed.
func (l *Lowerer) stmts(list []ast.Stmt) []ir.Stmt {
	var out []ir.Stmt
	for _, st := range list {
		savedPre := l.pre
		l.pre = nil

		lead := leadComments(st)
		body := l.stmt(st)
		pre := l.takePre()
		l.pre = savedPre

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
	l.scope = newScope(saved)
	b := l.markUnused(&ir.Block{Stmts: l.stmts(list)})
	l.scope = saved
	l.seps = savedSeps
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
	if b := l.bindingOfTarget(n.X); b != nil {
		// Incrementing $h{k} says the values are numbers, not that the hash
		// itself is one, which is what makes a counting hash come out as
		// map[string]int rather than map[string]any.
		switch n.X.(type) {
		case *ast.HashIndex, *ast.Index:
			l.observeElem(b, ir.TInt)
		default:
			l.observe(b, ir.TInt)
		}
	}
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
		branch := &ir.If{Cond: l.cond(ei.Cond), Then: l.block(ei.Then)}
		branch.Else = tail
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
	if len(n.ElseIfs) > 0 || out.Else != nil || out.Then == nil || len(out.Then.Stmts) != 1 {
		return false
	}
	switch out.Then.Stmts[0].(type) {
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

	depth := l.captureDepth()
	defer l.restoreCaptures(depth)

	cond := l.cond(n.Cond)
	if n.Until {
		cond = negated(cond)
	}
	setup := l.takePre()

	out := &ir.For{Cond: cond, Body: l.block(n.Body), Label: l.label(n.Label)}
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

	body := l.block(n.Body)
	body.Stmts = append(body.Stmts, extraPost...)
	out := &ir.For{Init: init, Cond: cond, Post: post, Body: body, Label: l.label(n.Label)}
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
		out := &ir.Branch{Kind: "break", Label: l.label(n.Label)}
		l.setProv(out, n)
		return []ir.Stmt{out}
	case "next":
		out := &ir.Branch{Kind: "continue", Label: l.label(n.Label)}
		l.setProv(out, n)
		return []ir.Stmt{out}
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
			for _, e := range flat {
				x := l.scalar(e)
				results = append(results, x)
				kinds = append(kinds, typeOrAny(x))
			}
		}
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
