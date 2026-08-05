package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file lowers the handle-position builtins: seek, tell, and the
// fixed-length read. They all become os.File calls, which is the one place
// the *os.File value earns its keep over an io.Reader: a position is a
// property of a file, not of a stream.

// seekGuarded lowers `seek($fh, POS, WHENCE) or FAILURE`, binding $! in the
// failure branch to the error Seek returned.
func (l *Lowerer) seekGuarded(n *ast.Call, onFail ast.Expr) ([]ir.Stmt, bool) {
	seekCall, ok := l.seekExpr(n)
	if !ok {
		return nil, false
	}

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
		Init: assign(":=", []ir.Expr{ir.NewIdent("_", nil), ir.NewIdent(errName, ir.TError)},
			[]ir.Expr{seekCall}),
		Cond: ir.Bin("!=", ir.NewIdent(errName, ir.TError), ir.Nil(ir.TError), ir.TBool),
		Then: &ir.Block{Stmts: append(inner, body...)},
	}
	l.setProv(st, n)
	l.note(st, "Seek returns the new position and an error, and the position is "+
		"blanked here because the original only asked whether the seek worked. The "+
		"init clause scopes the error to the check itself.",
		"errors-are-values", "if-err-nil-rhythm", "multiple-return-values")
	return []ir.Stmt{st}, true
}

// seekStatement lowers a bare seek whose result nothing looks at.
func (l *Lowerer) seekStatement(n *ast.Call) ([]ir.Stmt, bool) {
	seekCall, ok := l.seekExpr(n)
	if !ok {
		return nil, false
	}
	st := exprStmt(seekCall)
	l.setProv(st, n)
	l.note(st, "Seek returns the new position and an error, and this line reads "+
		"neither, exactly as the original did not. A seek that can genuinely fail "+
		"deserves the `if _, err :=` form.",
		"errors-are-values")
	return []ir.Stmt{st}, true
}

// seekExpr builds the fh.Seek(pos, whence) call itself.
func (l *Lowerer) seekExpr(n *ast.Call) (*ir.Call, bool) {
	args := flatten(argList(n))
	if len(args) != 3 {
		return nil, false
	}
	b := l.handleBinding(args[0])
	if b == nil {
		return nil, false
	}
	if l.pass == 2 && (b.Type == nil || !b.Type.Equal(fileType)) {
		return nil, false
	}
	pos := l.toInt(l.expr(args[1]), args[1])
	whence, named := seekWhence(args[2])
	if !named {
		whence = l.toInt(l.expr(args[2]), args[2])
	}
	out := ir.CallOf(selector(ir.NewIdent(b.Go, fileType), "Seek", nil),
		ir.NamedType("int64", ""), conversion(ir.NamedType("int64", ""), pos), whence)
	l.inform(n, "P2G6080", "seek",
		"seek is a method on the file, and its whence argument is a named constant: "+
			"io.SeekStart, io.SeekCurrent and io.SeekEnd say what 0, 1 and 2 meant.",
		"io-reader-writer", "errors-are-values")
	return out, true
}

// seekWhence turns a written-out whence of 0, 1 or 2 into the io constant
// with that meaning, which is how Go spells the argument.
func seekWhence(e ast.Expr) (ir.Expr, bool) {
	lit, ok := e.(*ast.NumberLit)
	if !ok {
		return nil, false
	}
	name := ""
	switch lit.Text {
	case "0":
		name = "SeekStart"
	case "1":
		name = "SeekCurrent"
	case "2":
		name = "SeekEnd"
	default:
		return nil, false
	}
	return ir.Pkg("io", "io", name, ir.TInt), true
}

// tellCall lowers tell to the seek-of-zero-bytes idiom, wrapped in a helper
// so it can stand in expression position.
func (l *Lowerer) tellCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) != 1 {
		return nil
	}
	b := l.handleBinding(args[0])
	if b == nil {
		return nil
	}
	if l.pass == 2 && (b.Type == nil || !b.Type.Equal(fileType)) {
		return nil
	}
	out := l.helperCall(hTellPos, ir.TInt, ir.NewIdent(b.Go, fileType))
	l.inform(n, "P2G6082", "tell",
		"There is no Tell in the os package: the position is asked for by seeking "+
			"zero bytes from the current position, which moves nothing and reports "+
			"where it stayed.",
		"io-reader-writer")
	return out
}

// readStatement lowers `read FH, TARGET, LENGTH` when the count nothing
// reads: the target gets the bytes and the statement is the assignment.
func (l *Lowerer) readStatement(n *ast.Call) ([]ir.Stmt, bool) {
	chunk, target, declares, ok := l.readParts(n)
	if !ok {
		return nil, false
	}
	op := "="
	if declares {
		op = ":="
	}
	st := assign(op, []ir.Expr{target}, []ir.Expr{chunk})
	l.setProv(st, n)
	return []ir.Stmt{st}, true
}

// readValue lowers the same call where its value is wanted, which in the
// original was how many bytes arrived.
func (l *Lowerer) readValue(n *ast.Call) ir.Expr {
	chunk, target, declares, ok := l.readParts(n)
	if !ok {
		return nil
	}
	op := "="
	if declares {
		op = ":="
	}
	st := assign(op, []ir.Expr{target}, []ir.Expr{chunk})
	l.setProv(st, n)
	l.emit(st)
	return lenOf(target)
}

// readParts builds the readChunk call and resolves the target it is
// assigned to. The four-argument offset form has no rule here.
func (l *Lowerer) readParts(n *ast.Call) (chunk, target ir.Expr, declares, ok bool) {
	args := flatten(argList(n))
	if len(args) != 3 {
		return nil, nil, false, false
	}
	b := l.handleBinding(args[0])
	if b == nil {
		return nil, nil, false, false
	}
	if l.pass == 2 && (b.Type == nil || !b.Type.Equal(fileType)) {
		return nil, nil, false, false
	}

	var tb *Binding
	switch t := args[1].(type) {
	case *ast.My:
		vars := declaredVars(t)
		if len(vars) != 1 || vars[0].Sigil != '$' {
			return nil, nil, false, false
		}
		tb = l.declare(vars[0], KindLocal)
		declares = true
	case *ast.Var:
		if t.Sigil != '$' {
			return nil, nil, false, false
		}
		tb = l.lookup('$', t.Name, t)
	default:
		return nil, nil, false, false
	}
	tb.Type = ir.TString
	l.observe(tb, ir.TString)
	tb.Writes++

	length := l.toInt(l.expr(args[2]), args[2])
	out := l.helperCall(hReadChunk, ir.TString, ir.NewIdent(b.Go, fileType), length)
	l.approximate(n, "P2G6081", "read",
		"an exact-length read, with the error folded into a short answer",
		"Reading an exact number of bytes is io.ReadFull, because a plain Read "+
			"may return fewer bytes than asked with no error at all. The original's "+
			"read reported failure as undef; the helper reports it the way a short "+
			"file looks.",
		"Code that must tell a closed pipe from a short file should call "+
			"io.ReadFull directly and look at the error.",
		"io-reader-writer", "errors-are-values")
	l.note(out, "readChunk is io.ReadFull with the short answer folded in: the "+
		"bytes that arrived, however many that was.",
		"io-reader-writer")
	return out, ir.NewIdent(tb.Go, ir.TString), declares, true
}
