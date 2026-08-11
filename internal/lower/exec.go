package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file lowers the ways a script runs another program.
//
// The difference that matters is not the spelling. A command written as one
// string is handed to a shell, which splits it into words and runs anything a
// semicolon introduces; a command written as a list of words is started
// directly. Go's exec.Command takes the list, so the list form converts into
// something both shorter and safer, and the string form converts into an
// explicit `sh -c` that says out loud what was happening all along.

// childStatus returns the variable standing in for $?, which every one of
// these calls writes and the code afterwards reads.
func (l *Lowerer) childStatus(at ast.Node) *Binding {
	b := l.special("$?", '$', "childStatus", at)
	b.Type = ir.TInt
	if b.Init == nil {
		b.Init = ir.IntLit("0")
		b.Doc = "childStatus is how the last program this one ran finished: the exit " +
			"code in the high byte, a signal number in the low seven bits."
		b.Explain = "Perl keeps this in $?, a global that every wait overwrites. Go " +
			"hands the status back from the call that produced it, so this variable " +
			"exists only because the code after the call reads it that way."
	}
	return b
}

// commandWords reads a command written as a list of words, when it is one.
//
// `system($prog, '-e', $code)` is the list form and needs no shell at all.
// `system("$prog -e '...'")` is one string, which the shell takes apart.
func (l *Lowerer) commandWords(args []ast.Expr) ([]ir.Expr, bool) {
	if len(args) < 2 {
		return nil, false
	}
	out := make([]ir.Expr, 0, len(args))
	for _, a := range args {
		x := l.scalar(a)
		if x == nil {
			return nil, false
		}
		out = append(out, l.toStr(x, a))
	}
	return out, true
}

// systemCall lowers `system`, which runs a program and answers with its wait
// status.
func (l *Lowerer) systemCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 0 {
		return l.todoExpr(n, "P2G6502", "system",
			"system was called with no command",
			"There is nothing here naming a program to run.",
			"Give the call a program and its arguments.",
			"os-exec")
	}
	status := l.childStatus(n)
	var call ir.Expr
	if words, ok := l.commandWords(args); ok {
		call = l.helperCall(hRunProgram, ir.TInt, words...)
		l.note(call, "The command was written as a list of words, so no shell is "+
			"involved and none is needed here: exec.Command takes the program and its "+
			"arguments separately and starts it directly. An argument with a space or "+
			"a semicolon in it arrives whole.",
			"os-exec")
	} else {
		call = l.helperCall(hRunShell, ir.TInt, l.toStr(l.scalar(args[0]), args[0]))
		l.note(call, "The command was written as one string, which means a shell took "+
			"it apart: it split the words, expanded any globs, and would have run "+
			"whatever a semicolon introduced. Go starts programs directly, so the "+
			"shell is asked for explicitly here, which is the honest translation and "+
			"also the line to rewrite: passing the words separately closes that door.",
			"os-exec")
		l.approximate(n, "P2G6505", "system with a command line",
			"the shell is now asked for by name",
			"A command written as one string went to a shell, which split it into "+
				"words and would have run anything a semicolon introduced. Go starts a "+
				"program directly, so the shell appears in the generated call.",
			"Pass the program and its arguments as separate words. The shell then has "+
				"nothing to do, and a value with a space or a quote in it stops being a "+
				"hazard.",
			"os-exec")
	}
	st := assign("=", []ir.Expr{l.identFor(status)}, []ir.Expr{call})
	status.Writes++
	l.setProv(st, n)
	l.note(st, "Perl leaves the wait status in $?, and system also answers with it. "+
		"Go returns it from the call, so it is stored here for the lines that read it "+
		"afterwards.",
		"os-exec", "errors-are-values")
	l.emit(st)
	status.Reads++
	return l.identFor(status)
}

// backtickCmd lowers `command` and qx//, which run a command and capture what
// it printed.
func (l *Lowerer) backtickCmd(n *ast.BacktickCmd, list bool) ir.Expr {
	line := l.interp(&ast.InterpLit{Parts: n.Parts})
	status := l.childStatus(n)
	name := l.tmp("out")
	t := ir.TString
	helper := hCaptureShell
	if list {
		t = ir.SliceOf(ir.TString)
		helper = hCaptureLines
	}
	decl := &ir.DeclStmt{Names: []string{name}, Type: t}
	st := assign("=", []ir.Expr{ir.NewIdent(name, t), l.identFor(status)},
		[]ir.Expr{l.helperCall(helper, t, l.toStr(line, n))})
	status.Writes++
	l.setProv(decl, n)
	l.setProv(st, n)
	l.emit(decl)
	if list {
		l.note(st, "Backticks in list context hand back one string per line, with the "+
			"newline still on the end, which is why the loops that use them chomp. The "+
			"helper splits the output the same way.",
			"os-exec", "strings-package")
	} else {
		l.note(st, "Backticks run a command and hand back everything it printed on "+
			"standard output; its standard error still goes to this program's. Go "+
			"returns the output and the error separately, and the wait status is kept "+
			"here because the lines after this one read it.",
			"os-exec", "errors-are-values")
	}
	l.emit(st)
	l.approximate(n, "P2G6501", "backticks",
		"the command line still goes through a shell",
		"Backticks hand the command to a shell, which splits it into words and "+
			"expands anything in it. The generated code says so by calling the shell "+
			"explicitly rather than pretending the words were separate.",
		"Where the pieces are known, pass them to exec.Command separately: no shell "+
			"runs, nothing is re-split, and a value with a space in it stays one "+
			"argument.",
		"os-exec", "errors-are-values")
	return ir.NewIdent(name, t)
}

// pipeType is the handle a pipe open produces. It reads and writes like a
// file, so the code around it does not change, and closing it waits for the
// program at the other end.
var pipeType = ir.NamedType("*pipeHandle", "")

// openPipe lowers `open $fh, '-|', PROG, ARGS` and its writing twin.
func (l *Lowerer) openPipe(handle *Binding, place ir.Expr, mode string, words []ast.Expr, n *ast.Call, onFail func(string) []ir.Stmt) ([]ir.Stmt, bool) {
	if len(words) == 0 {
		return nil, false
	}
	args := make([]ir.Expr, 0, len(words))
	for _, w := range words {
		args = append(args, l.toStr(l.scalar(w), w))
	}
	helper, reading := hOpenRead, true
	if mode == "|-" {
		helper, reading = hOpenWrite, false
	}
	if len(words) == 1 {
		// One word is a command line, which the shell took apart. Saying so
		// costs a shell and keeps the meaning.
		args = []ir.Expr{ir.Str(`"sh"`), ir.Str(`"-c"`), args[0]}
		l.approximate(n, "P2G6505", "a pipe open with a command line",
			"the shell is now asked for by name",
			"The command was written as one string, so a shell split it into words "+
				"and would have run anything a semicolon introduced. Go starts a program "+
				"directly, so the shell appears in the generated call.",
			"Pass the program and its arguments as separate words, which is the "+
				"list form of the same open.",
			"os-exec")
	}

	handle.Type = pipeType
	l.observe(handle, pipeType)
	errName := l.tmp("err")
	open := assign(":=", []ir.Expr{ir.NewIdent(handle.Go, pipeType), ir.NewIdent(errName, ir.TError)},
		[]ir.Expr{l.helperCall(helper, pipeType, args...)})
	l.setProv(open, n)
	if reading {
		l.note(open, "A pipe open starts a program and keeps one end of a pipe to it. "+
			"Go's exec.Cmd hands out that end through StdoutPipe, and the handle here "+
			"reads like a file, so the loop below it is the same loop it would be over "+
			"a file.",
			"os-exec", "io-reader-writer")
	} else {
		l.note(open, "This end of the pipe is the program's standard input, so writing "+
			"to the handle feeds it. Closing the handle is what tells the program there "+
			"is no more, and then waits for it to finish.",
			"os-exec", "io-reader-writer")
	}

	var failBody []ir.Stmt
	if onFail != nil {
		failBody = onFail(errName)
	}
	if len(failBody) == 0 {
		failBody = []ir.Stmt{
			exprStmt(call("fmt", "fmt", "Fprintln", ir.TVoid, ir.Pkg("os", "os", "Stderr", nil),
				ir.NewIdent(errName, ir.TError))),
			exprStmt(call("os", "os", "Exit", ir.TVoid, ir.IntLit("255"))),
		}
	}
	check := &ir.If{
		Cond: ir.Bin("!=", ir.NewIdent(errName, ir.TError), ir.Nil(ir.TError), ir.TBool),
		Then: &ir.Block{Stmts: failBody},
	}
	l.note(check, "Starting a program can fail for reasons that have nothing to do "+
		"with the program itself: the name may not be on the path, or there may be no "+
		"room for another process. Go hands that back as an error here rather than "+
		"leaving it to be discovered later.",
		"if-err-nil-rhythm", "errors-are-values")

	out := []ir.Stmt{open, check}
	if place != nil {
		out = append(out, l.storeHandle(place, handle, n))
	} else if !handle.Closed {
		closeStmt := &ir.Defer{Call: ir.CallOf(selector(ir.NewIdent(handle.Go, pipeType), "Close", nil), ir.TError)}
		l.note(closeStmt, "Closing a pipe waits for the program at the other end, which "+
			"is the part a file close does not do. Without it this program could finish "+
			"first and the other one would be killed halfway through.",
			"defer-timing", "os-exec")
		out = append(out, closeStmt)
	}
	return out, true
}
