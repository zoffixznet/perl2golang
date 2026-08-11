package lower

import (
	"slices"
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file lowers file input and output.
//
// It is where the difference between the two languages is most visible and
// most worth teaching. Perl's open returns a truth value and leaves the reason
// in $!, so `open(...) or die` reads as one sentence. Go returns the file and
// an error together, and the caller has to look at the error before using the
// file. Reading lines is the same story: Perl's <$fh> hides a buffer, and Go
// asks which buffer you want and how big it is.

var fileType = ir.NamedType("*os.File", "os")

// openCall lowers open in statement position, with no failure handling of its
// own. The `or die` form goes through openGuarded instead.
func (l *Lowerer) openCall(n *ast.Call) []ir.Stmt {
	stmts, ok := l.openStatements(n, nil)
	if !ok {
		return nil
	}
	return stmts
}

// openGuarded lowers `open(...) or FAILURE`, which is the shape almost every
// real script uses.
//
// The failure branch is lowered here rather than by the caller so that $! can
// be bound to the error the open actually returned. `open ... or die "...: $!"`
// is the single most common line of Perl I/O there is, and it should read
// correctly in the Go.
func (l *Lowerer) openGuarded(n *ast.Call, onFail ast.Expr) ([]ir.Stmt, bool) {
	return l.openStatements(n, func(errName string) []ir.Stmt {
		saved := l.errVar
		l.errVar = errName
		defer func() { l.errVar = saved }()

		savedPre := l.pre
		l.pre = nil
		body := l.exprStatement(onFail)
		inner := l.takePre()
		l.pre = savedPre
		return append(inner, body...)
	})
}

// openStatements builds the open, the error check, and the close. onFail is
// called with the name of the error variable to build the failure branch, and
// may be nil.
func (l *Lowerer) openStatements(n *ast.Call, onFail func(errName string) []ir.Stmt) ([]ir.Stmt, bool) {
	args := flatten(argList(n))
	if len(args) < 2 {
		return nil, false
	}

	// What the handle will hold decides what a container it is stored in
	// holds, so the pipe modes are recognised before the target is resolved.
	pipeMode, isPipe := staticString(args[1])
	isPipe = isPipe && (pipeMode == "-|" || pipeMode == "|-")
	want := fileType
	if isPipe {
		want = pipeType
	}
	handle, place := l.openTarget(args[0], want)
	if handle == nil {
		return nil, false
	}
	if isPipe {
		return l.openPipe(handle, place, pipeMode, args[2:], n, onFail)
	}

	mode, path, ok := l.openMode(args, n)
	if !ok {
		return nil, false
	}

	var opener ir.Expr
	switch mode {
	case "<":
		opener = call("os", "os", "Open", fileType, path)
	case ">":
		opener = call("os", "os", "Create", fileType, path)
	case ">>":
		opener = ir.CallOf(ir.Pkg("os", "os", "OpenFile", nil), fileType, path,
			ir.Bin("|", ir.Bin("|", ir.Pkg("os", "os", "O_APPEND", ir.TInt),
				ir.Pkg("os", "os", "O_CREATE", ir.TInt), ir.TInt),
				ir.Pkg("os", "os", "O_WRONLY", ir.TInt), ir.TInt),
			ir.Raw("0o644", ir.TInt))
	default:
		// The handle is still declared, from the stand-in, so that every line
		// below this one goes on compiling and the program fails here rather
		// than refusing to build at all.
		todo := l.refuse(n, "P2G6002", "open mode "+mode,
			"this open mode is not implemented",
			"The mode "+mode+" selects a pipe or a duplicated handle, which is a "+
				"different operation in Go.",
			"A pipe open becomes os/exec with StdoutPipe or StdinPipe; duplicating a "+
				"handle becomes passing the *os.File value around.",
			"os-exec")
		handle.Type = fileType
		l.observe(handle, fileType)
		st := &ir.DeclStmt{Names: []string{handle.Go}, Type: fileType}
		todo.Spelled = true
		ir.MetaOf(st).Todo = &todo
		l.setProv(st, n)
		l.note(st, "The handle is declared but never opened, so everything below "+
			"this line still compiles and the program fails here rather than "+
			"refusing to build at all.",
			"nil-vs-undef")
		return []ir.Stmt{st}, true
	}

	handle.Type = fileType
	l.observe(handle, fileType)
	errName := l.tmp("err")
	openStmt := assign(":=", []ir.Expr{ir.NewIdent(handle.Go, fileType), ir.NewIdent(errName, ir.TError)},
		[]ir.Expr{opener})
	l.setProv(openStmt, n)
	l.note(openStmt, "Go returns the file and an error together, and there is no way "+
		"to get at the file without also being handed the error. That is the whole of "+
		"Go's error handling: no exceptions, no global $!, just a second return value "+
		"the compiler will not let you forget is there.",
		"errors-are-values", "multiple-return-values", "if-err-nil-rhythm")

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
		l.approximate(n, "P2G6005", "open with no failure handling",
			"a failed open now ends the program",
			"The original does not check whether the open succeeded, so it would "+
				"carry on with an unusable handle and fail later, or silently do nothing. "+
				"Go will not compile code that ignores the error without saying so.",
			"Decide what a missing file should mean here. Reporting it and stopping "+
				"is the safe default the generated code takes.",
			"errors-are-values")
	}
	check := &ir.If{
		Cond: ir.Bin("!=", ir.NewIdent(errName, ir.TError), ir.Nil(ir.TError), ir.TBool),
		Then: &ir.Block{Stmts: failBody},
	}
	l.note(check, "`if err != nil` immediately after the call is the rhythm of Go "+
		"code. It reads as noise at first and becomes invisible quickly, and it puts "+
		"the failure path next to the thing that can fail rather than at the bottom "+
		"of the function.",
		"if-err-nil-rhythm")

	out := []ir.Stmt{openStmt, check}
	if place != nil {
		out = append(out, l.storeHandle(place, handle, n))
	} else if !handle.Closed {
		closeStmt := &ir.Defer{Call: ir.CallOf(selector(ir.NewIdent(handle.Go, fileType), "Close", nil), ir.TError)}
		l.note(closeStmt, "defer runs this when the surrounding function returns, "+
			"however it returns. Perl closes a lexical handle when it goes out of "+
			"scope; Go asks for it explicitly, which also means you can see it.",
			"defer-timing")
		out = append(out, closeStmt)
	}
	return out, true
}

// openValue lowers an open whose truth value the program keeps.
//
// `my $ok = open(...)` and `if (open(...))` are a different program from
// `open(...) or die`. The failure is a value the script carries and acts on
// itself, and the script runs on either way, reading nothing out of a handle
// that was never opened. Go's zero *os.File behaves the same: its Read
// answers with an error rather than stopping the program, so the translation
// is the error test and nothing more.
func (l *Lowerer) openValue(n *ast.Call) (ir.Expr, bool) {
	args := flatten(argList(n))
	if len(args) < 2 {
		return nil, false
	}
	// The pipe forms start a program, which is a statement whatever is done
	// with the answer.
	if mode, ok := staticString(args[1]); ok && (mode == "-|" || mode == "|-") {
		return nil, false
	}
	handle, place := l.openTarget(args[0], fileType)
	if handle == nil {
		return nil, false
	}
	mode, path, ok := l.openMode(args, n)
	if !ok {
		return nil, false
	}
	var opener ir.Expr
	switch mode {
	case "<":
		opener = call("os", "os", "Open", fileType, path)
	case ">":
		opener = call("os", "os", "Create", fileType, path)
	case ">>":
		opener = ir.CallOf(ir.Pkg("os", "os", "OpenFile", nil), fileType, path,
			ir.Bin("|", ir.Bin("|", ir.Pkg("os", "os", "O_APPEND", ir.TInt),
				ir.Pkg("os", "os", "O_CREATE", ir.TInt), ir.TInt),
				ir.Pkg("os", "os", "O_WRONLY", ir.TInt), ir.TInt),
			ir.Raw("0o644", ir.TInt))
	default:
		return nil, false
	}

	handle.Type = fileType
	l.observe(handle, fileType)
	errName := l.tmp("err")
	openStmt := assign(":=", []ir.Expr{ir.NewIdent(handle.Go, fileType), ir.NewIdent(errName, ir.TError)},
		[]ir.Expr{opener})
	l.setProv(openStmt, n)
	l.note(openStmt, "Go returns the file and an error together. Here the program "+
		"wanted the truth value the open answered with, so the error becomes that "+
		"truth value on the next line and the file is whatever the call produced: "+
		"a usable file, or a nil one whose reads answer with an error rather than "+
		"stopping the program.",
		"errors-are-values", "multiple-return-values", "nil-vs-undef")
	l.emit(openStmt)
	if place != nil {
		l.emit(l.storeHandle(place, handle, n))
	}
	// $! means this error for the rest of the block, which is as far as the
	// variable holding it is in scope.
	l.errFallback = errName

	l.approximate(n, "P2G6006", "an open whose result is used as a value",
		"the failure is a value the program carries, and the open is otherwise unchecked",
		"The result of the open is kept rather than acted on, so the program "+
			"carries on whether or not the file opened, and the reads below run "+
			"against a handle that was never opened. Perl answered those reads with "+
			"nothing; a nil *os.File answers them with an error, which comes to the "+
			"same thing here and is visible if it is ever looked at.",
		"Act on the failure where it happens: `if err != nil` next to the open is "+
			"the shape that stops a later line from working with a file that is not "+
			"there.",
		"errors-are-values", "if-err-nil-rhythm")

	out := ir.Bin("==", ir.NewIdent(errName, ir.TError), ir.Nil(ir.TError), ir.TBool)
	l.note(out, "The open's truth value in Perl was whether it worked. Go says the "+
		"same thing by comparing the error against nil, which is the one test every "+
		"Go program is built out of.",
		"errors-are-values", "if-err-nil-rhythm")
	return out, true
}

// openTarget resolves the first argument of open into the binding the opened
// handle is bound to, plus the place to store it in when the handle is going
// into a container rather than into a variable of its own.
//
// `open $log{$name}, '>', $path` is ordinary Perl and has no variable to
// declare: the handle belongs to a slot. Go can only get a file out of os.Open
// alongside an error, so the pair goes into a temporary, the error is checked
// there, and the file is put in the slot afterwards.
func (l *Lowerer) openTarget(e ast.Expr, want *ir.Type) (*Binding, ir.Expr) {
	if b := l.openHandle(e); b != nil {
		return b, nil
	}
	place := l.assignTarget(e)
	if place == nil {
		return nil, nil
	}
	l.observeTargetValue(e, want)
	return &Binding{
		Perl:  "handle",
		Sigil: '$',
		Kind:  KindLocal,
		Line:  posLine(e),
		Type:  want,
		Go:    l.tmp("fh"),
	}, place
}

// storeHandle puts a freshly opened handle into the place the open named.
func (l *Lowerer) storeHandle(place ir.Expr, handle *Binding, n ast.Node) ir.Stmt {
	st := assign("=", []ir.Expr{place}, []ir.Expr{ir.NewIdent(handle.Go, handle.Type)})
	l.setProv(st, n)
	l.note(st, "The handle is being kept in a container rather than in a variable of "+
		"its own. Go hands back the file and the error together and there is nowhere "+
		"to put an error inside a collection of files, so the pair lands in a "+
		"temporary, the error is dealt with there, and the file goes into the slot "+
		"once it is known to be a file.",
		"errors-are-values", "multiple-return-values")
	return st
}

// openHandle resolves the first argument of open into a binding.
func (l *Lowerer) openHandle(e ast.Expr) *Binding {
	switch n := e.(type) {
	case *ast.My:
		vars := declaredVars(n)
		if len(vars) != 1 {
			return nil
		}
		return l.declare(vars[0], KindLocal)
	case *ast.Var:
		return l.lookup(n.Sigil, n.Name, n)
	case *ast.FileHandle:
		b := l.lookup('$', n.Name, n)
		b.Perl = n.Name
		return b
	}
	return nil
}

// openMode reads the mode and path from either the two or the three argument
// form.
func (l *Lowerer) openMode(args []ast.Expr, n *ast.Call) (string, ir.Expr, bool) {
	if len(args) >= 3 {
		mode, ok := staticString(args[1])
		if !ok {
			l.refuse(n, "P2G6003", "open with a computed mode",
				"the open mode is not known until the program runs",
				"The mode argument is not a literal, so which of os.Open, os.Create "+
					"and os.OpenFile is meant cannot be decided at conversion time.",
				"Pick the call that matches the mode, or use os.OpenFile with the "+
					"flags computed the same way the mode string was.")
			return "", nil, false
		}
		return strings.TrimSpace(mode), l.toStr(l.expr(args[2]), args[2]), true
	}

	// The two-argument form puts the mode and the path in one string.
	text, ok := staticString(args[1])
	if !ok {
		l.refuse(n, "P2G6001", "two-argument open",
			"the two-argument form of open is not implemented",
			"Two-argument open takes the mode and the filename in one string, so a "+
				"filename beginning with > or | changes what the call does. That is a "+
				"long-standing source of security holes, which is why the three-argument "+
				"form exists.",
			"Rewrite it as the three-argument form first, with the mode separate "+
				"from the path, then the translation is direct.")
		return "", nil, false
	}
	l.approximate(n, "P2G6001", "two-argument open",
		"the mode was read out of the filename string",
		"Two-argument open takes the mode and the filename in one string. The mode "+
			"was separated at conversion time, which is safe here because the string is "+
			"a literal, but it would not be safe for a filename that came from input.",
		"Use the three-argument form of open in the original, which keeps the mode "+
			"and the path apart.")
	trimmed := strings.TrimSpace(text)
	for _, m := range []string{">>", "<", ">", "+<", "+>"} {
		if rest, found := strings.CutPrefix(trimmed, m); found {
			return m, ir.Str(quote(strings.TrimSpace(rest))), true
		}
	}
	return "<", ir.Str(quote(trimmed)), true
}

// closeGuarded lowers `close($fh) or FAILURE`.
//
// Go's if-with-an-init clause is made for this: the error is created, tested
// and forgotten in one statement, and it is the only thing $! could sensibly
// have meant. What is being closed still decides the call inside it, the same
// way it does for an unguarded close.
func (l *Lowerer) closeGuarded(n *ast.Call, onFail ast.Expr) ([]ir.Stmt, bool) {
	args := flatten(argList(n))
	if len(args) == 0 {
		return nil, false
	}
	handle, kind := l.handleToClose(args[0])
	if handle == nil || kind == closeIsArgv {
		return nil, false
	}
	if b := l.handleBinding(args[0]); b != nil {
		b.Closed = true
	}

	errName := l.tmp("err")
	saved := l.errVar
	// A close through the helper has no error value for $! to name, so the
	// failure branch is lowered with nothing bound to it rather than with a
	// name that would not exist.
	l.errVar = errName
	if kind == closeIsUnknown {
		l.errVar = ""
	}
	savedPre := l.pre
	l.pre = nil
	body := l.exprStatement(onFail)
	inner := l.takePre()
	l.pre = savedPre
	l.errVar = saved
	fail := &ir.Block{Stmts: append(inner, body...)}

	// A handle of no fixed type is closed through the helper, which answers
	// with success rather than with an error, so the test is a plain negation
	// and there is no error for the failure branch to report.
	if kind == closeIsUnknown {
		st := &ir.If{
			Cond: ir.Un("!", l.helperCall(hCloseHandle, ir.TBool, handle), ir.TBool),
			Then: fail,
		}
		l.setProv(st, n)
		l.note(st, "What this handle holds is not known until the program runs, so the "+
			"close asks the value whether it can be closed at all. The answer is a plain "+
			"bool rather than an error, because there is nothing more specific to say "+
			"about a value that turned out not to be a handle.",
			"implicit-interfaces", "type-assertions-and-switches")
		return []ir.Stmt{st}, true
	}

	var out []ir.Stmt
	st := &ir.If{
		Init: assign(":=", []ir.Expr{ir.NewIdent(errName, ir.TError)},
			[]ir.Expr{ir.CallOf(selector(handle, "Close", nil), ir.TError)}),
		Cond: ir.Bin("!=", ir.NewIdent(errName, ir.TError), ir.Nil(ir.TError), ir.TBool),
		Then: fail,
	}
	l.setProv(st, n)
	l.note(st, "Close returns an error because a buffered write can fail when the "+
		"buffer is flushed, and that is the last chance to notice the data never "+
		"reached the disk. The init clause of the if scopes the error to the check "+
		"itself, which is the usual shape for an error nothing else needs.",
		"errors-are-values", "if-err-nil-rhythm", "var-vs-short-declaration")
	out = append(out, st)

	// A guarded pipe close still has to leave the child's status behind, and
	// the status only exists once Close has waited, so it is read after the
	// check rather than inside it.
	if kind == closeIsPipe {
		if _, wanted := l.globalSeen["$?"]; wanted {
			status := l.childStatus(n)
			set := assign("=", []ir.Expr{l.identFor(status)},
				[]ir.Expr{ir.CallOf(selector(handle, "Status", nil), ir.TInt)})
			status.Writes++
			l.setProv(set, n)
			l.note(set, "The status of the program behind the pipe is only known once "+
				"the close has waited for it, which is why this line comes after the "+
				"check rather than inside it.",
				"os-exec")
			out = append(out, set)
		}
	}
	return out, true
}

// closeKind says which of the four closes a handle expression asks for.
type closeKind int

const (
	closeIsFile closeKind = iota
	closeIsPipe
	closeIsArgv
	closeIsUnknown
)

// handleToClose resolves the argument of a close into the value to close and
// the kind of close it is. It returns a nil expression when the argument is
// not something this rule can name at all.
func (l *Lowerer) handleToClose(arg ast.Expr) (ir.Expr, closeKind) {
	if fh, ok := arg.(*ast.FileHandle); ok {
		switch fh.Name {
		case "ARGV":
			return ir.Nil(ir.TAny), closeIsArgv
		case "STDIN", "STDOUT", "STDERR":
			return l.fileHandleExpr(fh), closeIsFile
		}
	}
	if b := l.handleBinding(arg); b != nil {
		switch {
		case b.Type.Equal(fileType):
			return ir.NewIdent(b.Go, fileType), closeIsFile
		case b.Type.Equal(pipeType):
			return ir.NewIdent(b.Go, pipeType), closeIsPipe
		}
	}
	x := l.scalar(arg)
	if x == nil {
		return nil, closeIsUnknown
	}
	switch t := typeOrAny(x); {
	case t.Equal(fileType):
		return x, closeIsFile
	case t.Equal(pipeType):
		return x, closeIsPipe
	}
	return x, closeIsUnknown
}

// closeCall lowers close.
//
// Which close this is depends on what the handle turned out to be. A file
// closes; a pipe closes and waits for the program on the other end, leaving its
// status in $?; the diamond's ARGV restarts the line counter; and a handle
// whose type is only known while the program runs goes through a helper. None
// of them may go missing, because a close that vanished is a file left open or
// a status never collected.
func (l *Lowerer) closeCall(n *ast.Call) []ir.Stmt {
	args := flatten(argList(n))
	if len(args) == 0 {
		return nil
	}
	handle, kind := l.handleToClose(args[0])
	if handle == nil {
		return nil
	}
	if b := l.handleBinding(args[0]); b != nil {
		b.Closed = true
	}
	switch kind {
	case closeIsArgv:
		return l.closeArgv(n)
	case closeIsPipe:
		return l.closePipe(handle, n)
	case closeIsFile:
		st := exprStmt(ir.CallOf(selector(handle, "Close", nil), ir.TError))
		l.setProv(st, n)
		l.note(st, "Close returns an error, because a buffered write can fail when "+
			"the buffer is flushed. For a file that was only read the error is safe to "+
			"ignore; for one that was written it is the last chance to notice that the "+
			"data did not reach the disk.",
			"errors-are-values")
		return []ir.Stmt{st}
	}
	return l.closeValue(handle, n)
}

// closePipe closes a pipe and collects the status of the program on the other
// end, which is the half of a pipe close a file close does not have.
func (l *Lowerer) closePipe(handle ir.Expr, n *ast.Call) []ir.Stmt {
	st := exprStmt(ir.CallOf(selector(handle, "Close", nil), ir.TError))
	l.setProv(st, n)
	l.note(st, "Closing a pipe waits for the program at the other end, so this line "+
		"is where the program's own running time is spent and where its exit status "+
		"becomes known. A file close has nothing to wait for; this one does.",
		"os-exec", "errors-are-values")
	out := []ir.Stmt{st}

	// $? is only worth writing when the program reads it. The binding exists
	// by the second pass if the file mentions $? anywhere, whether above this
	// close or below it.
	if _, wanted := l.globalSeen["$?"]; wanted {
		status := l.childStatus(n)
		set := assign("=", []ir.Expr{l.identFor(status)},
			[]ir.Expr{ir.CallOf(selector(handle, "Status", nil), ir.TInt)})
		status.Writes++
		l.setProv(set, n)
		l.note(set, "Perl leaves the status of the program behind a closed pipe in $?, "+
			"with the exit code in the high byte. Go keeps it on the handle, so it is "+
			"read from there into the variable the lines below this one use.",
			"os-exec")
		out = append(out, set)
	}
	return out
}

// closeArgv lowers `close ARGV`, the diamond operator's own close.
//
// Its purpose in a real script is almost never to release a handle. It is the
// line that makes `$.` count from one again in the next file, which is why it
// is nearly always written as `close ARGV if eof`.
func (l *Lowerer) closeArgv(n *ast.Call) []ir.Stmt {
	counter := l.lineCounter(n)
	st := assign("=", []ir.Expr{l.identFor(counter)}, []ir.Expr{ir.IntLit("0")})
	counter.Writes++
	l.setProv(st, n)
	l.note(st, "Perl's line counter is shared by every handle and keeps counting "+
		"across the files the diamond reads, so closing ARGV is how a script restarts "+
		"the numbering at each file. The generated loop keeps that count in a variable "+
		"of its own, and restarting it is an assignment.",
		"io-reader-writer")
	l.approximate(n, "P2G6043", "close ARGV",
		"the line counter restarts and the file is left to the loop",
		"`close ARGV` closes whichever input the diamond is reading, which restarts "+
			"the line counter and abandons the rest of that file. The counter is reset "+
			"here; the generated loop moves to the next file when the current one runs "+
			"out, so the two agree for the usual `close ARGV if eof` and differ if the "+
			"close was meant to skip the rest of a file.",
		"To skip the rest of a file, break out of the loop that reads it.",
		"io-reader-writer")
	return []ir.Stmt{st}
}

// closeStandard lowers a close of one of the three standard streams.
func (l *Lowerer) closeStandard(fh *ast.FileHandle, n *ast.Call) []ir.Stmt {
	st := exprStmt(ir.CallOf(selector(l.fileHandleExpr(fh), "Close", nil), ir.TError))
	l.setProv(st, n)
	l.note(st, "The standard streams are ordinary *os.File values in Go, so closing "+
		"one is the same call as closing any other file. A script that closes standard "+
		"output usually does it to find out whether the last write reached the far end, "+
		"and the error this returns is that answer.",
		"errors-are-values")
	return []ir.Stmt{st}
}

// closeValue lowers a close whose handle never settled on a handle type: a
// field of an object, an element of a list of things, a value passed into a
// sub.
func (l *Lowerer) closeValue(x ir.Expr, n *ast.Call) []ir.Stmt {
	st := exprStmt(l.helperCall(hCloseHandle, ir.TBool, x))
	l.setProv(st, n)
	l.note(st, "What this handle holds is not known until the program runs, so the "+
		"close asks the value whether it can be closed. That question is Go's type "+
		"assertion against io.Closer, which is how a program works with a value it "+
		"knows only by what it can do.",
		"implicit-interfaces", "type-assertions-and-switches")
	l.approximate(n, "P2G6044", "close on a handle of unknown type",
		"the value is closed if it turns out to be closeable",
		"The value being closed did not settle on a handle type at conversion time, "+
			"so the generated code asks at run time whether it implements io.Closer and "+
			"closes it if it does.",
		"Give the variable holding the handle a type, or open it where it is used, "+
			"and the close becomes a direct call.",
		"implicit-interfaces")
	return []ir.Stmt{st}
}

// ---------------------------------------------------------------------------
// Directories

var dirType = ir.SliceOf(ir.TString)

// opendirGuarded lowers `opendir my $dh, DIR or FAILURE`.
func (l *Lowerer) opendirGuarded(n *ast.Call, onFail ast.Expr) ([]ir.Stmt, bool) {
	return l.opendirStatements(n, func(errName string) []ir.Stmt {
		saved := l.errVar
		l.errVar = errName
		defer func() { l.errVar = saved }()

		savedPre := l.pre
		l.pre = nil
		body := l.exprStatement(onFail)
		inner := l.takePre()
		l.pre = savedPre
		return append(inner, body...)
	})
}

// opendirCall lowers opendir with no failure handling of its own.
func (l *Lowerer) opendirCall(n *ast.Call) []ir.Stmt {
	stmts, _ := l.opendirStatements(n, nil)
	return stmts
}

// opendirStatements reads the directory up front and binds the handle to the
// names it holds.
//
// Perl opens a directory and then pulls names off it one at a time. Go reads
// the whole directory in one call, so the handle and the list it would have
// produced are the same thing here, and closedir has nothing left to close.
func (l *Lowerer) opendirStatements(n *ast.Call, onFail func(errName string) []ir.Stmt) ([]ir.Stmt, bool) {
	args := flatten(argList(n))
	if len(args) < 2 {
		return nil, false
	}
	handle := l.openHandle(args[0])
	if handle == nil {
		return nil, false
	}
	handle.Type = dirType
	handle.Closed = true
	l.observe(handle, dirType)

	dir := l.toStr(l.expr(args[1]), args[1])
	errName := l.tmp("err")
	read := assign(":=", []ir.Expr{ir.NewIdent(handle.Go, dirType), ir.NewIdent(errName, ir.TError)},
		[]ir.Expr{l.helperCall(hDirNames, dirType, dir)})
	l.setProv(read, n)
	l.note(read, "Go reads a directory in one call rather than handing back a handle "+
		"to pull names off. That means there is nothing to close afterwards, and the "+
		"whole listing is in memory, which is the trade os.ReadDir makes for you.",
		"errors-are-values", "multiple-return-values")

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
	l.approximate(n, "P2G6042", "opendir",
		"the whole directory is read at once",
		"opendir hands back a handle that readdir pulls names off one at a time, so "+
			"a huge directory never has to be in memory all at once. os.ReadDir reads "+
			"the lot, sorts it, and gives you a slice.",
		"For a directory too large to hold, os.File.ReadDir(n) reads it in batches "+
			"and is the closer match.")
	return []ir.Stmt{read, check}, true
}

// readdirCall lowers readdir, which by then is just the list opendir read.
func (l *Lowerer) readdirCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 0 {
		return composite(dirType, nil, nil)
	}
	b := l.handleBinding(args[0])
	if b == nil {
		return composite(dirType, nil, nil)
	}
	out := l.ident(b)
	l.note(out, "The names were read when the directory was opened, so this is the "+
		"list itself. In Perl readdir is a cursor and calling it twice gives different "+
		"answers; here it is a value and does not move.")
	return out
}

// closedirCall lowers closedir, which has nothing to do once the directory has
// been read in one go.
func (l *Lowerer) closedirCall(n *ast.Call) []ir.Stmt {
	l.inform(n, "P2G6041", "closedir",
		"The directory was read in a single call, so there is no open handle left "+
			"to close and the call has been dropped. Nothing leaks: os.ReadDir closes "+
			"the directory itself before it returns.")
	return nil
}

// unlinkCall lowers unlink, which returns how many files it removed.
func (l *Lowerer) unlinkCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 0 {
		return ir.IntLit("0")
	}
	paths := make([]ir.Expr, 0, len(args))
	spread := false
	for _, a := range args {
		x := l.expr(a)
		if x == nil {
			continue
		}
		if t := typeOrAny(x); t.Kind == ir.Slice {
			paths = append(paths, l.strSlice(x, a))
			spread = true
			continue
		}
		paths = append(paths, l.toStr(x, a))
	}
	out := l.helperCall(hRemoveFiles, ir.TInt, paths...)
	out.Ellipsis = spread && len(paths) == 1
	l.approximate(n, "P2G6045", "unlink",
		"the count is kept and the reason is not",
		"unlink returns how many files it managed to remove, so the reason a "+
			"removal failed is left in a global and usually never looked at. "+
			"os.Remove returns the error itself, one path at a time.",
		"Test the returned error rather than a count. os.Remove on a missing file "+
			"returns an error that errors.Is(err, fs.ErrNotExist) recognises, which "+
			"is often the case worth ignoring.",
		"errors-are-values")
	l.note(out, "One path at a time is the whole of os.Remove, so removing a list is "+
		"a loop. What the loop throws away is the error, which is the part worth "+
		"keeping.",
		"errors-are-values")
	return out
}

// handleBinding resolves an expression naming a filehandle.
func (l *Lowerer) handleBinding(e ast.Expr) *Binding {
	switch n := e.(type) {
	case *ast.Var:
		return l.lookup(n.Sigil, n.Name, n)
	case *ast.FileHandle:
		return l.lookup('$', n.Name, n)
	}
	return nil
}

// readlineExpr lowers <$fh> outside a loop condition, which means reading the
// whole handle.
func (l *Lowerer) readlineExpr(n *ast.Readline) ir.Expr {
	src := l.readSource(n)
	// A program that also reads single lines holds each handle's unread
	// bytes in a buffered reader, and a whole-handle read must continue
	// from there rather than from the file's own position.
	if l.mixedReads {
		src = l.helperCall(hReading, ir.NamedType("*bufio.Reader", "bufio"), src)
	}

	// What a read stops at is whatever $/ was last set to. Perl carries that
	// in a global the read consults; here it is known already and becomes part
	// of the call, which is how Go says it.
	switch l.seps.rec {
	case sepSlurp:
		out := l.helperCall(hReadAll, ir.TString, src)
		l.note(out, "With $/ set to undef a read takes the whole handle in one piece. "+
			"Go asks for that directly with io.ReadAll, and there is no mode left set "+
			"afterwards for the next read to trip over.",
			"io-reader-writer")
		l.approximate(n, "P2G6012", "<> with $/ undefined",
			"the whole handle is read into memory at once",
			"With $/ undefined a read consumes everything the handle has. The "+
				"generated code does the same with io.ReadAll, so the entire input is in "+
				"memory at once.",
			"For an input that may not fit in memory, read a line at a time with "+
				"bufio.Scanner and keep only what you need.",
			"io-reader-writer", "bufio-scanner-limit")
		return out
	case sepDynamic:
		out := l.helperCall(hReadRecords, ir.SliceOf(ir.TString), src, l.seps.dyn)
		l.note(out, "$/ holds a value the program worked out, so the separator rides "+
			"into the read as an argument, which is where Go names one anyway.",
			"io-reader-writer")
		return out
	case sepPara, sepCustom:
		sep := l.seps.text
		out := l.helperCall(hReadRecords, ir.SliceOf(ir.TString), src, ir.Str(quote(sep)))
		if l.seps.rec == sepPara {
			l.note(out, "Paragraph mode reads up to a run of blank lines. Go has no "+
				"reading mode, so the input is read and split, which says the same thing "+
				"in one place rather than in a global set somewhere earlier.",
				"io-reader-writer")
		} else {
			l.note(out, "$/ set to text makes a read stop there rather than at a newline. "+
				"Go names the separator at the point of use, so it appears in this call "+
				"and nowhere else.",
				"io-reader-writer")
		}
		return out
	}

	out := l.helperCall(hReadLines, ir.SliceOf(ir.TString), src)
	l.note(out, "In list context <$fh> reads the whole handle into a list of lines. "+
		"Go has os.ReadFile for a whole file and bufio.Scanner for a line at a time; "+
		"reading everything into memory first is a choice, and for a large file it is "+
		"usually the wrong one.",
		"io-reader-writer", "bufio-scanner-limit")
	return out
}

// readlineScalar lowers <$fh> where one value is wanted: a single line, or
// undef at the end of the input. This is a different operation from the
// list-context read, which takes everything the handle has.
//
// It also commits the whole program to reading through one buffered reader
// per handle. A single-line read has to buffer to find the newline, and a
// later read of any shape must continue where it stopped, not where the
// operating system's file position happens to be.
func (l *Lowerer) readlineScalar(n *ast.Readline) (ir.Expr, bool) {
	if l.seps.rec != sepLine {
		// With $/ changed, the scalar read takes a record or the whole
		// handle; the existing whole-handle forms stand in for those.
		return nil, false
	}
	if l.pass == 1 {
		l.mixedReads = true
	}
	src := l.readSource(n)
	out := l.helperCall(hReadLine, nullable(ir.TString), src)
	l.note(out, "One read takes one line here, where the same operator in list "+
		"context takes them all: Perl decides by the context, Go by which call "+
		"this is. The result is a *string so the end of the input reads as nil, "+
		"which is what undef meant, and every read on this handle shares one "+
		"buffered reader so none of them steals the others' read-ahead.",
		"nil-vs-undef", "io-reader-writer", "context-is-gone")
	// A program that reads $. counts every successful read, this one
	// included, so the line is named and counted before it is used.
	if l.countsLines {
		name := l.tmp("line")
		nt := nullable(ir.TString)
		get := assign(":=", []ir.Expr{ir.NewIdent(name, nt)}, []ir.Expr{out})
		l.setProv(get, n)
		l.emit(get)
		counter := l.lineCounter(n)
		l.emit(&ir.If{
			Cond: ir.Bin("!=", ir.NewIdent(name, nt), ir.Nil(nt), ir.TBool),
			Then: &ir.Block{Stmts: []ir.Stmt{
				assign("+=", []ir.Expr{ir.NewIdent(counter.Go, ir.TInt)}, []ir.Expr{ir.IntLit("1")}),
			}},
		})
		return ir.NewIdent(name, nt), true
	}
	return out, true
}

// readSource resolves what a readline reads from.
func (l *Lowerer) readSource(n *ast.Readline) ir.Expr {
	if n.Var != nil {
		return l.expr(n.Var)
	}
	switch n.Handle {
	case "STDIN", "":
		return ir.Pkg("os", "os", "Stdin", fileType)
	}
	b := l.lookup('$', n.Handle, n)
	return ir.NewIdent(b.Go, fileType)
}

// readLoop recognises `while (my $line = <$fh>)` and `while (<$fh>)` and emits
// the scanner loop that Go reads a file with.
func (l *Lowerer) readLoop(n *ast.While) ([]ir.Stmt, bool) {
	target, rl, ok := readLoopShape(n.Cond)
	if !ok {
		return nil, false
	}

	saved := l.scope
	l.scope = newScope(saved)
	defer func() { l.scope = saved }()

	// `while (<>)` walks every file named on the command line, falling back
	// to standard input when there are none. The inner loop below is built
	// against one open handle, and a loop over the names wraps it at the
	// end; $ARGV is the loop variable, which is exactly what it was.
	diamond := rl.Handle == "" && rl.Var == nil
	var src ir.Expr
	var argvName *Binding
	if diamond {
		argvName = l.special("$ARGV", '$', "argvName", n)
		argvName.Type = ir.TString
		inName := l.tmp("in")
		src = ir.NewIdent(inName, fileType)
	} else {
		src = l.readSource(rl)
	}
	scanner := l.tmp("scanner")
	scannerType := ir.NamedType("*bufio.Scanner", "bufio")

	mk := assign(":=", []ir.Expr{ir.NewIdent(scanner, scannerType)},
		[]ir.Expr{call("bufio", "bufio", "NewScanner", scannerType, src)})
	l.setProv(mk, n)
	l.note(mk, "bufio.Scanner reads a line at a time without loading the file. Its "+
		"default maximum line length is 64 kibibytes, and a longer line makes Scan "+
		"stop early and report bufio.ErrTooLong, so the buffer is raised here. Perl "+
		"has no such limit, which is exactly why this is easy to miss.",
		"bufio-scanner-limit", "io-reader-writer")

	buffer := exprStmt(ir.CallOf(selector(ir.NewIdent(scanner, scannerType), "Buffer", nil), ir.TVoid,
		ir.CallOf(ir.NewIdent("make", nil), ir.SliceOf(ir.NamedType("byte", "")),
			ir.Raw("[]byte", nil), ir.IntLit("0"), ir.Raw("64*1024", ir.TInt)),
		ir.Raw("1024*1024", ir.TInt)))

	// The body decides whether the newline has to be kept. The chomp is
	// usually the first statement, but a counter bump or another statement
	// that cannot be reading the line often precedes it, and missing the
	// chomp there means adding the newline back only to trim it off again,
	// with a note claiming the body never chomps.
	body := n.Body
	chomped := false
	for i, st := range body {
		if isChompOf(st, target) {
			chomped = true
			body = append(slices.Clone(body[:i]), body[i+1:]...)
			break
		}
		if !ignoresLine(st, target) {
			break
		}
	}

	var b *Binding
	if target != nil {
		if my, isMy := target.declNode.(*ast.My); isMy {
			vars := declaredVars(my)
			if len(vars) == 1 {
				b = l.declare(vars[0], KindLocal)
			}
		} else if v, isVar := target.declNode.(*ast.Var); isVar {
			b = l.lookup(v.Sigil, v.Name, v)
		}
	}
	if b == nil {
		b = l.declareNamed("line@"+itoa(posLine(n)), '$', "line", KindLoop, n)
		b.Perl = "$_"
	}
	if l.pass == 2 {
		b.Type = ir.TString
	}
	l.observe(b, ir.TString)

	// A program that also reads single lines outside its loops has to read
	// everything through one buffered reader per handle, or the loop's
	// read-ahead would swallow lines the single reads were waiting for. The
	// loop then walks the same readLine call the single reads use.
	if l.mixedReads {
		next := l.tmp("next")
		nextT := nullable(ir.TString)
		getLine := assign(":=", []ir.Expr{ir.NewIdent(next, nextT)},
			[]ir.Expr{l.helperCall(hReadLine, nextT, src)})
		l.setProv(getLine, n)
		stop := &ir.If{
			Cond: ir.Bin("==", ir.NewIdent(next, nextT), ir.Nil(nextT), ir.TBool),
			Then: &ir.Block{Stmts: []ir.Stmt{&ir.Branch{Kind: "break"}}},
		}
		deref := ir.Expr(ir.Un("*", ir.NewIdent(next, nextT), ir.TString))
		if chomped {
			deref = call("strings", "strings", "TrimSuffix", ir.TString, deref, ir.Str(`"\n"`))
		}
		mixDecl := assign(":=", []ir.Expr{ir.NewIdent(b.Go, ir.TString)}, []ir.Expr{deref})
		l.note(getLine, "This program also reads single lines from its handles, so "+
			"every read goes through one buffered reader per handle: a scanner of "+
			"its own here would read ahead and swallow lines the single reads were "+
			"waiting for. readLine hands back nil at the end of the input, which is "+
			"what ends the loop.",
			"io-reader-writer", "nil-vs-undef")
		if target == nil {
			l.topicStack = append(l.topicStack, ir.NewIdent(b.Go, ir.TString))
			defer func() { l.topicStack = l.topicStack[:len(l.topicStack)-1] }()
		}
		if l.pass == 1 {
			l.readLoops++
		}
		l.readLoopSeq++
		inner := l.block(body)
		lead := []ir.Stmt{getLine, stop, mixDecl}
		if l.pass == 2 && b.Used == 0 && b.Reads == 0 {
			lead = append(lead, l.discardIfUnused(b)...)
		}
		var reset ir.Stmt
		if l.countsLines {
			counter := l.lineCounter(n)
			lead = append(lead, assign("+=", []ir.Expr{ir.NewIdent(counter.Go, ir.TInt)}, []ir.Expr{ir.IntLit("1")}))
			if l.readLoops > 1 && l.readLoopSeq > 1 {
				reset = assign("=", []ir.Expr{ir.NewIdent(counter.Go, ir.TInt)}, []ir.Expr{ir.IntLit("0")})
			}
		}
		inner.Stmts = append(lead, inner.Stmts...)
		loop := &ir.For{Body: inner, Label: l.label(n.Label)}
		l.setProv(loop, n)
		out := []ir.Stmt{loop}
		if reset != nil {
			out = append([]ir.Stmt{reset}, out...)
		}
		if diamond {
			return l.wrapDiamond(argvName, src, out, n), true
		}
		return out, true
	}

	text := ir.Expr(ir.CallOf(selector(ir.NewIdent(scanner, scannerType), "Text", nil), ir.TString))
	if !chomped {
		text = ir.Bin("+", text, ir.Str(`"\n"`), ir.TString)
		l.inform(n, "P2G6011", "reading lines",
			"Perl's readline keeps the newline on the end of each line, and "+
				"bufio.Scanner strips it. The body here never chomps, so the newline is "+
				"added back to keep the behaviour identical. If the newline is not "+
				"actually wanted, drop that and the code gets simpler.")
	}

	lineDecl := assign(":=", []ir.Expr{ir.NewIdent(b.Go, ir.TString)}, []ir.Expr{text})
	if chomped {
		l.note(lineDecl, "Scanner.Text returns the line without its newline, so the "+
			"chomp that followed the read in the original is not needed.")
	}

	if target == nil {
		l.topicStack = append(l.topicStack, ir.NewIdent(b.Go, ir.TString))
		defer func() { l.topicStack = l.topicStack[:len(l.topicStack)-1] }()
	}

	if l.pass == 1 {
		l.readLoops++
	}
	l.readLoopSeq++

	inner := l.block(body)
	lead := []ir.Stmt{lineDecl}
	// The line is declared by the loop rather than by the body, so the pass
	// that drops unread declarations never sees it.
	if l.pass == 2 && b.Used == 0 && b.Reads == 0 {
		lead = append(lead, l.discardIfUnused(b)...)
	}
	var reset ir.Stmt
	if l.countsLines {
		counter := l.lineCounter(n)
		bump := assign("+=", []ir.Expr{ir.NewIdent(counter.Go, ir.TInt)}, []ir.Expr{ir.IntLit("1")})
		l.note(bump, "This is what $. was doing invisibly. Counting the line here, "+
			"next to the read, is one line of code and removes the question of which "+
			"handle the number belongs to.")
		lead = append(lead, bump)
		if l.readLoops > 1 && l.readLoopSeq > 1 {
			// More than one loop shares the counter, so each one starts it
			// again. Perl does the same when the previous handle was closed.
			reset = assign("=", []ir.Expr{ir.NewIdent(counter.Go, ir.TInt)}, []ir.Expr{ir.IntLit("0")})
		}
	}
	inner.Stmts = append(lead, inner.Stmts...)

	loop := &ir.For{
		Cond:  ir.CallOf(selector(ir.NewIdent(scanner, scannerType), "Scan", nil), ir.TBool),
		Body:  inner,
		Label: l.label(n.Label),
	}
	l.setProv(loop, n)
	l.note(loop, "Scan returns false at the end of the input and also when something "+
		"went wrong, so the loop is followed by a check of Err. Skipping that check "+
		"turns a read error into a file that quietly looks shorter than it is.",
		"errors-are-values")

	errCheck := &ir.If{
		Init: assign(":=", []ir.Expr{ir.NewIdent("err", ir.TError)},
			[]ir.Expr{ir.CallOf(selector(ir.NewIdent(scanner, scannerType), "Err", nil), ir.TError)}),
		Cond: ir.Bin("!=", ir.NewIdent("err", ir.TError), ir.Nil(ir.TError), ir.TBool),
		Then: &ir.Block{Stmts: []ir.Stmt{
			exprStmt(call("fmt", "fmt", "Fprintln", ir.TVoid, ir.Pkg("os", "os", "Stderr", nil),
				ir.NewIdent("err", ir.TError))),
			exprStmt(call("os", "os", "Exit", ir.TVoid, ir.IntLit("255"))),
		}},
	}
	l.note(errCheck, "The init clause of an if scopes err to the check itself, which "+
		"is the usual way to keep a short-lived error out of the surrounding "+
		"function.",
		"if-err-nil-rhythm", "var-vs-short-declaration")

	out := []ir.Stmt{mk, buffer, loop, errCheck}
	if reset != nil {
		out = append([]ir.Stmt{reset}, out...)
	}
	if diamond {
		return l.wrapDiamond(argvName, src, out, n), true
	}
	return out, true
}

// wrapDiamond wraps one file's reading loop in the walk over every file
// named on the command line, which is what <> was doing invisibly.
func (l *Lowerer) wrapDiamond(argvName *Binding, src ir.Expr, inner []ir.Stmt, n ast.Node) []ir.Stmt {
	args := l.argv(n)
	sources := l.tmp("sources")
	strsT := ir.SliceOf(ir.TString)
	decl := assign(":=", []ir.Expr{ir.NewIdent(sources, strsT)},
		[]ir.Expr{ir.NewIdent(args.Go, strsT)})
	l.setProv(decl, n)
	l.note(decl, "<> reads every file named on the command line, one after another, "+
		"and reads standard input when there are none. Go has no such operator, so "+
		"the file list is walked out loud, and the dash stands for standard input "+
		"the way it conventionally does.",
		"io-reader-writer", "range-is-not-foreach")
	fallback := &ir.If{
		Cond: ir.Bin("==", lenOf(ir.NewIdent(sources, strsT)), ir.IntLit("0"), ir.TBool),
		Then: &ir.Block{Stmts: []ir.Stmt{
			assign("=", []ir.Expr{ir.NewIdent(sources, strsT)},
				[]ir.Expr{composite(strsT, nil, []ir.Expr{ir.Str(`"-"`)})}),
		}},
	}

	// in := os.Stdin; if the name is not "-", open the file, warn and move
	// on when it cannot be opened, which is what perl's <> does.
	inIdent, _ := src.(*ir.Ident)
	openStmts := []ir.Stmt{
		assign(":=", []ir.Expr{src}, []ir.Expr{ir.Pkg("os", "os", "Stdin", fileType)}),
	}
	f := l.tmp("f")
	errName := l.tmp("err")
	openIf := &ir.If{
		Cond: ir.Bin("!=", ir.NewIdent(argvName.Go, ir.TString), ir.Str(`"-"`), ir.TBool),
		Then: &ir.Block{Stmts: []ir.Stmt{
			assign(":=", []ir.Expr{ir.NewIdent(f, fileType), ir.NewIdent(errName, ir.TError)},
				[]ir.Expr{call("os", "os", "Open", fileType, ir.NewIdent(argvName.Go, ir.TString))}),
			&ir.If{
				Cond: ir.Bin("!=", ir.NewIdent(errName, ir.TError), ir.Nil(ir.TError), ir.TBool),
				Then: &ir.Block{Stmts: []ir.Stmt{
					exprStmt(call("fmt", "fmt", "Fprintln", ir.TVoid,
						ir.Pkg("os", "os", "Stderr", nil), ir.NewIdent(errName, ir.TError))),
					&ir.Branch{Kind: "continue"},
				}},
			},
			assign("=", []ir.Expr{src}, []ir.Expr{ir.NewIdent(f, fileType)}),
		}},
	}
	openStmts = append(openStmts, openIf)
	closeIf := &ir.If{
		Cond: ir.Bin("!=", src, ir.Pkg("os", "os", "Stdin", fileType), ir.TBool),
		Then: &ir.Block{Stmts: []ir.Stmt{
			exprStmt(ir.CallOf(selector(src, "Close", nil), ir.TError)),
		}},
	}
	_ = inIdent

	bodyStmts := append(openStmts, inner...)
	bodyStmts = append(bodyStmts, closeIf)
	walk := &ir.Range{
		Key:    ir.NewIdent("_", ir.TInt),
		Value:  ir.NewIdent(argvName.Go, ir.TString),
		X:      ir.NewIdent(sources, strsT),
		Define: true,
		Body:   &ir.Block{Stmts: bodyStmts},
	}
	l.setProv(walk, n)
	l.approximate(n, "P2G6013", "the <> handle",
		"the file walk is written out",
		"<> reads every file named on the command line in order, warning and "+
			"moving on when one cannot be opened, and falls back to standard input "+
			"when none are named. The generated loop does the same; the warning's "+
			"wording is Go's rather than perl's.",
		"Where only one input is ever passed, open it directly and drop the walk.",
		"io-reader-writer")
	return []ir.Stmt{decl, fallback, walk}
}

// readTarget names what a read loop assigns each line to.
type readTarget struct {
	declNode ast.Expr
}

// readLoopShape recognises the two shapes of a line-reading loop.
func readLoopShape(cond ast.Expr) (*readTarget, *ast.Readline, bool) {
	switch n := cond.(type) {
	case *ast.Readline:
		return nil, n, true
	case *ast.Assign:
		if n.Op != "=" {
			return nil, nil, false
		}
		rl, ok := n.RHS.(*ast.Readline)
		if !ok {
			return nil, nil, false
		}
		return &readTarget{declNode: n.LHS}, rl, true
	case *ast.UnOp:
		if n.Op == "defined" {
			return readLoopShape(n.X)
		}
	case *ast.Call:
		if n.Name == "defined" && len(n.Args) == 1 {
			return readLoopShape(n.Args[0])
		}
	}
	return nil, nil, false
}

// isChompOf reports whether a statement is `chomp $x` for the loop's variable.
func isChompOf(st ast.Stmt, target *readTarget) bool {
	es, ok := st.(*ast.ExprStmt)
	if !ok {
		return false
	}
	c, ok := es.X.(*ast.Call)
	if !ok || c.Name != "chomp" {
		return false
	}
	if target == nil {
		return len(c.Args) == 0
	}
	args := flatten(argList(c))
	if len(args) == 0 {
		return false
	}
	want := targetVar(target.declNode)
	got, ok := args[0].(*ast.Var)
	return ok && want != nil && got.Sigil == want.Sigil && got.Name == want.Name
}

// ignoresLine reports whether a statement certainly does not read the loop's
// line variable, so a chomp after it still governs the newline. It recognises
// only the shapes that are trivially safe, the common one being a counter
// bump like $lines++ between the read and the chomp.
func ignoresLine(st ast.Stmt, target *readTarget) bool {
	es, ok := st.(*ast.ExprStmt)
	if !ok {
		return false
	}
	un, ok := es.X.(*ast.UnOp)
	if !ok || (un.Op != "++" && un.Op != "--") {
		return false
	}
	v, ok := un.X.(*ast.Var)
	if !ok {
		return false
	}
	if target == nil {
		// The loop reads into $_, which the bump must not touch.
		return !(v.Sigil == '$' && v.Name == "_")
	}
	want := targetVar(target.declNode)
	if want == nil {
		return false
	}
	return v.Sigil != want.Sigil || v.Name != want.Name
}

// targetVar pulls the variable out of a read loop's assignment target.
func targetVar(e ast.Expr) *ast.Var {
	switch n := e.(type) {
	case *ast.Var:
		return n
	case *ast.My:
		if vs := declaredVars(n); len(vs) == 1 {
			return vs[0]
		}
	}
	return nil
}

// fileTest lowers -e and its relatives.
//
// Each one asks the filesystem a single question, which in Go is os.Stat
// followed by a look at the FileInfo. The helpers keep the question at the call
// site, where the Perl had it, rather than spreading a stat and an error check
// through the middle of an expression.
// isTopicVar reports whether an expression is $_ itself.
func isTopicVar(e ast.Expr) bool {
	v, ok := e.(*ast.Var)
	return ok && v.Sigil == '$' && v.Name == "_"
}

func (l *Lowerer) fileTest(n *ast.FileTest) ir.Expr {
	node := n.Arg
	if isStatReuse(node) {
		// `-f _` reuses the stat the previous test performed. Go has no such
		// cache, so the path is tested again.
		if l.lastStat == nil {
			return l.todoExpr(n, "P2G6031", "the _ filehandle",
				"there is no earlier file test to reuse",
				"`_` reuses the result of the last file test, and no test came before "+
					"this one in a place the converter could see.",
				"Name the path and test it directly.")
		}
		node = l.lastStat
		l.approximate(n, "P2G6031", "the _ filehandle",
			"the path is inspected again rather than reused",
			"`_` reuses the stat the previous test already performed, which saves a "+
				"system call. Go keeps no such cache, so the generated code asks about "+
				"the path a second time.",
			"Call os.Stat once, keep the FileInfo, and read every answer off it: "+
				"IsDir, Mode().IsRegular(), and Size().")
	} else if node != nil {
		l.lastStat = node
	}

	var arg ir.Expr
	switch {
	case l.findWalk != nil && (node == nil || isTopicVar(node)):
		// Inside a directory walk $_ is the entry's bare name, because find
		// changed into each directory before calling the block. WalkDir
		// changes nothing, so a test against the bare name would look in
		// the wrong directory; the full path is what reaches the entry.
		arg = ir.NewIdent(l.findWalk.path, ir.TString)
	case node == nil:
		if len(l.topicStack) > 0 {
			arg = l.toStr(l.topicStack[len(l.topicStack)-1], nil)
		} else {
			arg = ir.Str(`""`)
		}
	default:
		arg = l.toStr(l.expr(node), node)
	}

	switch n.Op {
	case 'e':
		out := l.helperCall(hFileExists, ir.TBool, arg)
		l.note(out, "Go asks the filesystem with os.Stat and looks at the error. "+
			"There is no single-character test, and the error tells you why the file "+
			"is unavailable rather than only that it is.",
			"errors-are-values")
		return out
	case 'd':
		out := l.helperCall(hIsDir, ir.TBool, arg)
		l.note(out, "os.Stat returns a FileInfo, and IsDir is a method on it. One stat "+
			"answers every question the file tests ask separately.",
			"errors-are-values")
		return out
	case 'f':
		out := l.helperCall(hIsFile, ir.TBool, arg)
		l.note(out, "Mode().IsRegular() is the FileInfo's answer to -f: an ordinary "+
			"file rather than a directory, a device, or a socket.")
		return out
	case 's':
		out := l.helperCall(hFileSize, ir.TInt, arg)
		l.approximate(n, "P2G6032", "-s file test",
			"a missing file now reports a size of zero",
			"-s returns the file's size, and undef when the file cannot be inspected, "+
				"so `defined -s $p` tells a missing file from an empty one. A Go int has "+
				"no undef, so both answer 0.",
			"Test with os.Stat and look at the error where the difference matters.",
			"nil-vs-undef")
		return out
	case 'l':
		out := l.helperCall(hIsLink, ir.TBool, arg)
		l.note(out, "os.Stat follows a symbolic link and answers about what it points "+
			"at, so asking whether something is a link needs os.Lstat, the call that "+
			"does not follow. That is the whole difference between the two.",
			"filepath-and-paths")
		return out
	case 'z':
		out := ir.Bin("&&", l.helperCall(hFileExists, ir.TBool, arg),
			ir.Bin("==", l.helperCall(hFileSize, ir.TInt, arg), ir.IntLit("0"), ir.TBool), ir.TBool)
		l.note(out, "-z asks two things at once: that the file is there, and that it "+
			"is empty. Written out, they are two calls.")
		return out
	case 'r':
		out := l.helperCall(hIsReadable, ir.TBool, arg)
		l.note(out, "Whether a file can be read is answered by opening it. The "+
			"permission bits say what the owner and the group may do, which is a "+
			"different question from what this process may do.")
		return out
	case 'w':
		out := l.helperCall(hIsWritable, ir.TBool, arg)
		l.approximate(n, "P2G6033", "-w file test",
			"writability is read off the permission bits",
			"-w asks whether this process may write to the path, taking the running "+
				"user into account. The generated code looks at the permission bits "+
				"instead, which does not know who is running.",
			"Where it matters, try the write and handle the error, which is the only "+
				"answer that cannot go stale between the test and the write.",
			"errors-are-values")
		return out
	case 'x':
		out := l.helperCall(hIsExecutable, ir.TBool, arg)
		l.approximate(n, "P2G6033", "-x file test",
			"executability is read off the permission bits",
			"-x asks whether this process may execute the path. The generated code "+
				"looks at the permission bits instead, which does not know who is running.",
			"Where it matters, run the program and handle the error.",
			"errors-are-values")
		return out
	}
	return l.todoExpr(n, "P2G6030", "-"+string(n.Op)+" file test",
		"this file test is not implemented",
		"Perl's file test operators ask one question each. Go asks os.Stat once and "+
			"reads the answer off the returned FileInfo.",
		"Call os.Stat, check the error, then use the FileInfo: ModTime for -M and "+
			"-A, Mode() for the type tests, and Sys() for the fields Go does not "+
			"expose portably.",
		"errors-are-values")
}

// isStatReuse reports whether a file test's argument is the `_` handle, which
// asks for the previous test's answer rather than naming a path.
func isStatReuse(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.Call:
		return n.Name == "_" && argCount(n) == 0
	case *ast.FileHandle:
		return n.Name == "_"
	}
	return false
}
