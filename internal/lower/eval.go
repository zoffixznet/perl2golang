package lower

import (
	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// This file lowers Perl's exception handling.
//
// `die` throws and `eval { }` catches, which is the one part of Perl that maps
// onto panic and recover rather than onto Go's errors-as-values. That mapping
// is deliberate and narrow: a program with no eval in it keeps the honest Go
// shape, where die prints and exits, because a panic that nothing recovers is
// a worse translation of an intentional exit. A program that does catch gets
// the panic-and-recover pair, because nothing else reproduces the unwinding.

// evalCall lowers `eval { BLOCK }`, whose value is the block's last expression
// and whose failure lands in $@.
func (l *Lowerer) evalCall(n *ast.Call) ir.Expr {
	if n.Block == nil {
		return l.todoExpr(n, "P2G8001", "string eval",
			"eval of a string is not implemented",
			"The string form of eval compiles Perl source while the program runs. Go "+
				"has no compiler at run time, so there is nothing to translate it into.",
			"A fixed set of expressions becomes named functions selected from a map. "+
				"Anything less fixed than that has no Go equivalent.",
			"errors-are-values")
	}

	l.traps = true
	errVar := l.errText(n)
	clear := assign("=", []ir.Expr{ir.NewIdent(errVar.Go, ir.TString)}, []ir.Expr{ir.Str(`""`)})
	l.setProv(clear, n)
	l.note(clear, "eval clears $@ before it runs the block, so a stale message from "+
		"an earlier failure cannot be mistaken for this one. That is a real trap in "+
		"Perl and the reason $@ has to be read immediately.",
		"errors-are-values")

	saved := l.scope
	l.scope = newScope(saved)
	stmts, value := l.blockValue(n.Block)
	l.scope = saved

	result := ""
	t := ir.TVoid
	if value != nil {
		t = typeOrAny(value)
		if t.Kind == ir.Void || t.Kind == ir.Invalid {
			t = ir.TAny
		}
		result = l.tmp("result")
		stmts = append(stmts, assign("=", []ir.Expr{ir.NewIdent(result, t)},
			[]ir.Expr{l.assignable(value, t, nil)}))
	}

	body := l.markUnused(&ir.Block{Stmts: append([]ir.Stmt{l.recoverInto(errVar)}, stmts...)})
	run := exprStmt(ir.CallOf(funcLit(nil, nil, body), ir.TVoid))
	l.setProv(run, n)
	l.note(run, "The block runs inside a function literal so that defer has "+
		"something to attach to: recover only works in a deferred call, and defer "+
		"only runs when a function returns. Calling the literal immediately is what "+
		"gives the block its own function to leave.",
		"panic-and-recover", "defer-timing")

	l.emit(clear)
	if result != "" {
		decl := &ir.DeclStmt{Names: []string{result}, Type: t}
		l.note(decl, "The block's value has to outlive the function literal, so it is "+
			"declared outside and assigned inside. A closure can see and write the "+
			"variables around it, which is what makes that work.",
			"closures-and-loop-capture")
		l.emit(decl)
	}
	l.emit(run)

	l.approximate(n, "P2G8002", "eval block",
		"a trapped failure arrives as a panic rather than as an error",
		"eval traps a die from anywhere inside it, however deep. Go's equivalent of "+
			"that unwinding is panic and recover, so the generated code uses them. Go "+
			"reserves them for genuine bugs: a function that can fail returns an error, "+
			"and the caller checks it.",
		"Rewriting the block's contents to return an error, and testing that error "+
			"here, is what this code would look like written as Go. That change is "+
			"worth making by hand: it is the single biggest difference between Perl "+
			"error handling and Go error handling.",
		"errors-are-values", "panic-and-recover", "if-err-nil-rhythm")

	if result == "" {
		return ir.BoolLit(true)
	}
	return ir.NewIdent(result, t)
}

// recoverInto builds the deferred call that turns a panic back into a message
// in $@.
func (l *Lowerer) recoverInto(errVar *Binding) ir.Stmt {
	caught := l.tmp("caught")
	handler := &ir.If{
		Init: assign(":=", []ir.Expr{ir.NewIdent(caught, ir.TAny)},
			[]ir.Expr{ir.CallOf(ir.NewIdent("recover", nil), ir.TAny)}),
		Cond: ir.Bin("!=", ir.NewIdent(caught, ir.TAny), ir.Nil(ir.TAny), ir.TBool),
		Then: &ir.Block{Stmts: []ir.Stmt{
			assign("=", []ir.Expr{ir.NewIdent(errVar.Go, ir.TString)},
				[]ir.Expr{l.toStr(ir.NewIdent(caught, ir.TAny), nil)}),
		}},
	}
	st := &ir.Defer{Call: ir.CallOf(funcLit(nil, nil, &ir.Block{Stmts: []ir.Stmt{handler}}), ir.TVoid)}
	l.note(st, "recover stops a panic and returns whatever was passed to panic. It "+
		"only does anything inside a deferred function, and it returns nil when "+
		"nothing is unwinding, which is why the result is tested.",
		"panic-and-recover")
	return st
}

// errText returns the variable standing in for $@.
func (l *Lowerer) errText(at ast.Node) *Binding {
	b := l.lookup('$', "_err", at)
	b.Perl = "$@"
	b.Type = ir.TString
	l.observe(b, ir.TString)
	if b.Doc == "" {
		b.Doc = b.Go + " holds the message from the last trapped failure."
		b.Explain = "Perl keeps this in $@, which every eval overwrites, which is " +
			"why it has to be read immediately after the eval that set it."
	}
	return b
}

// mainRecover builds the deferred handler main needs when the program throws:
// it prints the message and exits, which is what an uncaught die does.
func (l *Lowerer) mainRecover() ir.Stmt {
	caught := l.tmp("caught")
	handler := &ir.If{
		Init: assign(":=", []ir.Expr{ir.NewIdent(caught, ir.TAny)},
			[]ir.Expr{ir.CallOf(ir.NewIdent("recover", nil), ir.TAny)}),
		Cond: ir.Bin("!=", ir.NewIdent(caught, ir.TAny), ir.Nil(ir.TAny), ir.TBool),
		Then: &ir.Block{Stmts: []ir.Stmt{
			exprStmt(call("fmt", "fmt", "Fprint", ir.TVoid, ir.Pkg("os", "os", "Stderr", nil),
				l.toStr(ir.NewIdent(caught, ir.TAny), nil))),
			exprStmt(call("os", "os", "Exit", ir.TVoid, ir.IntLit("255"))),
		}},
	}
	st := &ir.Defer{Call: ir.CallOf(funcLit(nil, nil, &ir.Block{Stmts: []ir.Stmt{handler}}), ir.TVoid)}
	l.note(st, "A failure that nothing traps ends the program with its message on "+
		"standard error and status 255. Go's own answer to an unrecovered panic is a "+
		"stack trace and status 2, so this handler is here to keep the original "+
		"behaviour.",
		"panic-and-recover")
	return st
}
