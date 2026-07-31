package lower

import (
	"strings"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
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

	handle := l.openHandle(args[0])
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
		l.refuse(n, "P2G6002", "open mode "+mode,
			"this open mode is not implemented",
			"The mode "+mode+" selects a pipe or a duplicated handle, which is a "+
				"different operation in Go.",
			"A pipe open becomes os/exec with StdoutPipe or StdinPipe; duplicating a "+
				"handle becomes passing the *os.File value around.",
			"os-exec")
		return nil, false
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
	if !handle.Closed {
		closeStmt := &ir.Defer{Call: ir.CallOf(selector(ir.NewIdent(handle.Go, fileType), "Close", nil), ir.TError)}
		l.note(closeStmt, "defer runs this when the surrounding function returns, "+
			"however it returns. Perl closes a lexical handle when it goes out of "+
			"scope; Go asks for it explicitly, which also means you can see it.",
			"defer-timing")
		out = append(out, closeStmt)
	}
	return out, true
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
// have meant.
func (l *Lowerer) closeGuarded(n *ast.Call, onFail ast.Expr) ([]ir.Stmt, bool) {
	args := flatten(argList(n))
	if len(args) == 0 {
		return nil, false
	}
	b := l.handleBinding(args[0])
	if b == nil {
		return nil, false
	}
	b.Closed = true

	errName := l.tmp("err")
	saved := l.errVar
	l.errVar = errName
	savedPre := l.pre
	l.pre = nil
	body := l.exprStatement(onFail)
	inner := l.takePre()
	l.pre = savedPre
	l.errVar = saved

	st := &ir.If{
		Init: assign(":=", []ir.Expr{ir.NewIdent(errName, ir.TError)},
			[]ir.Expr{ir.CallOf(selector(ir.NewIdent(b.Go, fileType), "Close", nil), ir.TError)}),
		Cond: ir.Bin("!=", ir.NewIdent(errName, ir.TError), ir.Nil(ir.TError), ir.TBool),
		Then: &ir.Block{Stmts: append(inner, body...)},
	}
	l.setProv(st, n)
	l.note(st, "Close returns an error because a buffered write can fail when the "+
		"buffer is flushed, and that is the last chance to notice the data never "+
		"reached the disk. The init clause of the if scopes the error to the check "+
		"itself, which is the usual shape for an error nothing else needs.",
		"errors-are-values", "if-err-nil-rhythm", "var-vs-short-declaration")
	return []ir.Stmt{st}, true
}

// closeCall lowers close.
func (l *Lowerer) closeCall(n *ast.Call) []ir.Stmt {
	args := flatten(argList(n))
	if len(args) == 0 {
		return nil
	}
	if b := l.handleBinding(args[0]); b != nil && b.Type.Equal(fileType) {
		b.Closed = true
		st := exprStmt(ir.CallOf(selector(ir.NewIdent(b.Go, fileType), "Close", nil), ir.TError))
		l.setProv(st, n)
		l.note(st, "Close returns an error, because a buffered write can fail when "+
			"the buffer is flushed. For a file that was only read the error is safe to "+
			"ignore; for one that was written it is the last chance to notice that the "+
			"data did not reach the disk.",
			"errors-are-values")
		return []ir.Stmt{st}
	}
	return nil
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
	if len(args) == 1 {
		out := call("os", "os", "Remove", ir.TError, l.toStr(l.expr(args[0]), args[0]))
		l.approximate(n, "P2G6045", "unlink",
			"unlink's count becomes an error value",
			"unlink returns how many files it managed to remove, so the reason a "+
				"removal failed is left in $! and usually never looked at. os.Remove "+
				"returns the error itself.",
			"Test the returned error rather than a count. os.Remove on a missing file "+
				"returns an error that errors.Is(err, fs.ErrNotExist) recognises, which "+
				"is often the case worth ignoring.",
			"errors-are-values")
		return out
	}
	l.refuse(n, "P2G6045", "unlink of several files",
		"removing several paths in one call is not implemented",
		"unlink takes a list and returns how many of them it removed. os.Remove "+
			"takes one path and returns one error.",
		"Loop over the paths and call os.Remove for each, deciding what a failure "+
			"means as you go.",
		"errors-are-values")
	return ir.IntLit("0")
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
	out := l.helperCall(hReadLines, ir.SliceOf(ir.TString), src)
	l.note(out, "In list context <$fh> reads the whole handle into a list of lines. "+
		"Go has os.ReadFile for a whole file and bufio.Scanner for a line at a time; "+
		"reading everything into memory first is a choice, and for a large file it is "+
		"usually the wrong one.",
		"io-reader-writer", "bufio-scanner-limit")
	return out
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

	src := l.readSource(rl)
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

	// The body decides whether the newline has to be kept.
	body := n.Body
	chomped := false
	if len(body) > 0 && isChompOf(body[0], target) {
		chomped = true
		body = body[1:]
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
		if l.readLoops > 1 {
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
	return out, true
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
	if node == nil {
		if len(l.topicStack) > 0 {
			arg = l.toStr(l.topicStack[len(l.topicStack)-1], nil)
		} else {
			arg = ir.Str(`""`)
		}
	} else {
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
