package lower

import (
	"sort"
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
	"perl2golang/internal/perl/token"
)

// declare creates (pass 1) or recovers (pass 2) the binding for a declaration
// site. Keying on the AST node means both passes agree about which record a
// name refers to, which is the whole reason the two-pass design works.
func (l *Lowerer) declare(v *ast.Var, kind Kind) *Binding {
	if b, ok := l.decls[v]; ok {
		// A name that an earlier discovery round read as a local can turn
		// out to be a parameter once its sub's signature settles, and a
		// parameter settles earlier than a local, so the record moves up.
		if kind == KindParam && b.Kind == KindLocal {
			b.Kind = KindParam
		}
		l.scope.define(varKey(v.Sigil, v.Name), b)
		return b
	}
	// A file-scope `my` that a sub also reads is one variable in Perl and has
	// to be one variable here, which at file scope means a package-level one.
	if l.hoisted[v] {
		b := l.lookup(v.Sigil, v.Name, v)
		b.Kind = KindGlobal
		l.decls[v] = b
		return b
	}
	b := &Binding{
		Perl:  string(v.Sigil) + v.Name,
		Sigil: v.Sigil,
		Kind:  kind,
		Line:  posLine(v),
		Type:  defaultFor(v.Sigil),
	}
	b.Go = l.names.take(goName(v.Name))
	l.decls[v] = b
	l.scope.define(varKey(v.Sigil, v.Name), b)
	return b
}

// declareNamed creates a binding that has no declaration site in the source,
// such as a recovered subroutine parameter or a loop variable the converter
// introduced.
func (l *Lowerer) declareNamed(key string, sigil rune, name string, kind Kind, at ast.Node) *Binding {
	if b, ok := l.decls[keyNode{key}]; ok {
		l.scope.define(varKey(sigil, name), b)
		return b
	}
	b := &Binding{
		Perl:  string(sigil) + name,
		Sigil: sigil,
		Kind:  kind,
		Line:  posLine(at),
		Type:  defaultFor(sigil),
	}
	b.Go = l.names.take(goName(name))
	l.decls[keyNode{key}] = b
	l.scope.define(varKey(sigil, name), b)
	return b
}

// keyNode lets a synthetic binding live in the same map as the source-derived
// ones without a second field on the Lowerer.
type keyNode struct{ key string }

func (keyNode) Pos() token.Pos { return token.Pos{} }
func (keyNode) End() token.Pos { return token.Pos{} }

// lookup finds a binding by sigil and name, creating a package-level one when
// the name has never been seen. Perl without `use strict` allows that, and so
// does Perl with `our`.
func (l *Lowerer) lookup(sigil rune, name string, at ast.Node) *Binding {
	key := varKey(sigil, name)
	if b, ok := l.scope.lookup(key); ok {
		return b
	}
	if b, ok := l.globalSeen[key]; ok {
		return b
	}
	// A package variable has one identity however it is spelled: %CALLS
	// inside package TextUtil and %TextUtil::CALLS anywhere name the same
	// hash. A qualified name falls back to the bare binding created under
	// that package, and a bare name to the qualified binding of the package
	// being lowered, whichever the file happened to declare first.
	if i := strings.LastIndex(name, "::"); i >= 0 {
		pkg, bare := name[:i], name[i+2:]
		if b, ok := l.globalSeen[varKey(sigil, bare)]; ok && b.Pkg == pkg {
			l.globalSeen[key] = b
			return b
		}
	} else if l.curPkg != "" && l.curPkg != "main" {
		if b, ok := l.globalSeen[varKey(sigil, l.curPkg+"::"+name)]; ok {
			l.globalSeen[key] = b
			return b
		}
	}
	b := &Binding{
		Perl:  key,
		Sigil: sigil,
		Kind:  KindGlobal,
		Line:  posLine(at),
		Type:  defaultFor(sigil),
		Pkg:   l.curPkg,
	}
	if i := strings.LastIndex(name, "::"); i >= 0 {
		b.Pkg = name[:i]
	}
	b.Go = l.names.take(goName(name))
	l.globalSeen[key] = b
	l.globals = append(l.globals, b)
	return b
}

// special returns the package-level variable standing in for one of Perl's own
// variables, creating it the first time the program asks for it.
//
// It is deliberately not `lookup`. Perl's own variables have names no ordinary
// variable can have, and the Go name they are given, `args` for @ARGV, is a
// perfectly ordinary one a script may well use for something else. Going
// through `lookup` under that Go name let a program's own `my @args` capture
// @ARGV's binding and produce `args := args`, which does not compile. Keying on
// the Perl spelling keeps the two apart, and the name set still hands out a
// free Go identifier, so the collision resolves in the generated names instead.
func (l *Lowerer) special(key string, sigil rune, goBase string, at ast.Node) *Binding {
	if b, ok := l.globalSeen[key]; ok {
		return b
	}
	b := &Binding{
		Perl:  key,
		Sigil: sigil,
		Kind:  KindGlobal,
		Line:  posLine(at),
		Type:  defaultFor(sigil),
	}
	b.Go = l.names.take(goBase)
	l.globalSeen[key] = b
	l.globals = append(l.globals, b)
	return b
}

// argv returns the variable standing in for @ARGV, creating it the first time
// the program asks for it.
//
// Everything that reads or rewrites the arguments has to come through here.
// An option block consumes the options it recognised and puts the leftovers
// back, so a second binding for the same array would leave the code after the
// option block reading the arguments as they arrived, options and all.
func (l *Lowerer) argv(at ast.Node) *Binding {
	// @ARGV is an ordinary array in Perl: scripts shift it, sort it and
	// assign to it. os.Args[1:] is an expression rather than a variable, so
	// the arguments get a real variable of their own.
	b := l.special("@ARGV", '@', "args", at)
	b.Type = ir.SliceOf(ir.TString)
	if b.Init == nil {
		b.Init = slicing(ir.Pkg("os", "os", "Args", ir.SliceOf(ir.TString)),
			ir.IntLit("1"), nil, ir.SliceOf(ir.TString))
		b.Doc = "args holds the command line arguments, without the program name."
		b.Explain = "Perl's @ARGV holds the arguments after the program name, and " +
			"is an ordinary array a script can shift or sort. Go's os.Args includes " +
			"the program name at index 0, and it is a slice expression rather than a " +
			"variable, so the arguments are given a name here."
	}
	return b
}

// lineCounter returns the variable standing in for $., creating it the first
// time the program asks for it.
//
// It lives at package level because $. does: the value outlives the loop that
// set it, and the line after the loop can still read it.
func (l *Lowerer) lineCounter(at ast.Node) *Binding {
	const key = "$."
	b, ok := l.globalSeen[key]
	if !ok {
		b = &Binding{
			Perl:  key,
			Sigil: '$',
			Kind:  KindGlobal,
			Line:  posLine(at),
			Type:  ir.TInt,
			Init:  ir.IntLit("0"),
		}
		b.Go = l.names.take("lineNo")
		b.Doc = b.Go + " counts the lines read so far from the input being read."
		b.Explain = "Perl keeps this count for you in $., updated by every read and " +
			"shared by every handle. Go keeps no such count, so the loops that read " +
			"lines maintain it here."
		l.globalSeen[key] = b
		l.globals = append(l.globals, b)
	}
	l.countsLines = true
	l.observe(b, ir.TInt)
	return b
}

// matchPos returns the variable holding a scalar's match position, creating it
// the first time a global match or a call to pos needs it.
//
// Perl hangs the position off the scalar itself, so it survives between
// statements and is reset when the variable is assigned. Go has nowhere to hang
// it, so it becomes a package-level int beside the program's other state.
func (l *Lowerer) matchPos(b *Binding, at ast.Node) *Binding {
	if b == nil {
		return nil
	}
	if b.Pos != nil {
		return b.Pos
	}
	key := b.Perl + "\x00pos"
	p, ok := l.globalSeen[key]
	if !ok {
		p = &Binding{
			Perl:  "pos(" + b.Perl + ")",
			Sigil: '$',
			Kind:  KindGlobal,
			Line:  posLine(at),
			Type:  ir.TInt,
			Init:  ir.IntLit("0"),
		}
		p.Go = l.names.take(goName(b.Perl[1:]) + "Pos")
		p.Doc = p.Go + " is how far a global match has walked through " + goName(b.Perl[1:]) + "."
		p.Explain = "Perl keeps this position on the scalar itself, which is why " +
			"pos() takes a variable and why assigning to the variable forgets it. " +
			"Go has nowhere to keep it but a variable of its own."
		l.globalSeen[key] = p
		l.globals = append(l.globals, p)
	}
	l.observe(p, ir.TInt)
	b.Pos = p
	return p
}

// observe records a type the binding was seen holding. Only pass 1 collects;
// pass 2 reads the settled answer.
func (l *Lowerer) observe(b *Binding, t *ir.Type) {
	if b == nil || t == nil || l.pass != 1 {
		return
	}
	if t.Kind == ir.Void || t.Kind == ir.Invalid {
		return
	}
	b.Evidence = l.recordObservation(b.Evidence, t)
}

// observeElem records a type for the *elements* of a container binding.
func (l *Lowerer) observeElem(b *Binding, t *ir.Type) {
	if b == nil || t == nil || l.pass != 1 {
		return
	}
	switch b.Sigil {
	case '@':
		l.observe(b, ir.SliceOf(t))
	case '%':
		l.observe(b, ir.MapOf(t))
	default:
		l.observe(b, t)
	}
}

// markNilElems records that the program put undef into one of a container's
// elements, which is what decides whether its element type needs a pointer.
//
// It is deliberately about the container rather than about the element: Perl
// has one array, and one slot holding undef makes every slot a slot that might.
func (l *Lowerer) markNilElems(b *Binding) {
	if b == nil || l.pass != 1 {
		return
	}
	if b.Sigil == '@' || b.Sigil == '%' {
		b.NilElems = true
	}
}

// markNilElemsFrom does the same for a list being assigned to a container,
// where undef can be any one of the values.
func (l *Lowerer) markNilElemsFrom(b *Binding, rhs ast.Expr) {
	if b == nil || l.pass != 1 || rhs == nil {
		return
	}
	for _, e := range flatten(rhs) {
		if isUndefLiteral(e) {
			l.markNilElems(b)
			return
		}
	}
}

// noteLength records what a whole-array assignment says about how long the
// array is from then on.
//
// The bound is a minimum and it is deliberately pessimistic: several
// assignments keep the smallest, and anything the converter cannot count
// wipes it out. Being wrong in that direction costs a growth call that turns
// out to be unnecessary; being wrong in the other direction costs a panic.
func (l *Lowerer) noteLength(b *Binding, n int) {
	if b == nil || l.pass != 1 || b.Sigil != '@' {
		return
	}
	if n < 0 {
		// The length is only known while the program runs, so nothing about
		// it can be relied on here.
		l.forgetLength(b)
		return
	}
	if !b.lenKnown || n < b.MinLen {
		b.MinLen = n
	}
	b.lenKnown = true
}

// forgetLength says the array may now be shorter than anything seen before,
// which pop, shift, splice and a write to $#a all make possible.
func (l *Lowerer) forgetLength(b *Binding) {
	if b == nil || l.pass != 1 || b.Sigil != '@' {
		return
	}
	b.MinLen = 0
	b.lenKnown = true
}

// assignedLen is what a whole-array assignment says about the new length: the
// element count when the right side is written out as a list, and whatever is
// known about the other array when one is copied into another.
func (l *Lowerer) assignedLen(rhs ast.Expr, lowered ir.Expr) int {
	if v, ok := rhs.(*ast.Var); ok && v.Sigil == '@' {
		if src, found := l.scope.lookup(varKey('@', v.Name)); found && src.lenKnown {
			return src.MinLen
		}
		return -1
	}
	return literalLen(lowered)
}

// literalLen is how many elements a lowered list has when it is written out as
// one, and -1 when its length is only known while the program runs.
func literalLen(x ir.Expr) int {
	if c, ok := x.(*ir.CompositeLit); ok && c.LitType != nil && c.LitType.Kind == ir.Slice {
		return len(c.Elems)
	}
	return -1
}

// staticIndex reads an index written as a plain non-negative number.
func staticIndex(e ast.Expr) (int, bool) {
	n, ok := e.(*ast.NumberLit)
	if !ok {
		return 0, false
	}
	return strconvAtoi(n.Text)
}

// withinLength reports whether an index is one the array provably already has,
// which is the only case where a plain Go index expression is safe.
func withinLength(b *Binding, idx ast.Expr) bool {
	if b == nil {
		return false
	}
	k, ok := staticIndex(idx)
	return ok && k < b.MinLen
}

// ident builds a reference to a binding and counts the read.
func (l *Lowerer) ident(b *Binding) ir.Expr {
	if b == nil {
		return ir.Nil(ir.TAny)
	}
	b.Reads++
	return l.identFor(b)
}

// identFor renders a binding without counting a read. A binding the converter
// rewrote (a loop variable that had to become an indexed element, for example)
// renders as that expression instead of as its name.
func (l *Lowerer) identFor(b *Binding) ir.Expr {
	if b == nil {
		return ir.Nil(ir.TAny)
	}
	if x, ok := l.aliases[b]; ok {
		return x
	}
	// The class name a constructor was called with is one name here, so the
	// variable holding it is that name and no variable is needed.
	if c := l.classVars[b]; c != nil {
		return ir.Str(quote(c.Perl))
	}
	return ir.NewIdent(b.Go, b.Type)
}

// ---------------------------------------------------------------------------
// Variable expressions

// varExpr lowers a variable reference.
func (l *Lowerer) varExpr(v *ast.Var) ir.Expr {
	switch v.Sigil {
	case '#':
		// $#array is the last index, which in Go is len(a) - 1.
		b := l.lookup('@', v.Name, v)
		out := ir.Bin("-", lenOf(l.ident(b)), ir.IntLit("1"), ir.TInt)
		l.note(out, "$#array is the last valid index. Go has no such shorthand: it "+
			"is len(slice) - 1, and it is -1 for an empty slice exactly as Perl's is.",
			"slices-not-arrays")
		return out
	case '&':
		if s, ok := l.findSub(v.Name); ok {
			out := ir.NewIdent(s.Go, l.subFuncType(s))
			l.note(out, "\\&name takes a reference to a sub. A Go function is already "+
				"a value, so its bare name is the reference: it can be stored, passed, "+
				"and called without any taking or dereferencing.",
				"closures-and-loop-capture")
			return out
		}
	case '*':
		if x := l.globHandle(v.Name, v); x != nil {
			return x
		}
	}

	if x := l.specialVar(v); x != nil {
		return x
	}

	b := l.lookup(v.Sigil, v.Name, v)
	return l.ident(b)
}

// globHandle lowers a glob that names a filehandle, which is the only part of
// a glob a script normally wants.
//
// A glob is an entry in a symbol table, and `\*STDOUT` is a reference to that
// entry. Go has no symbol table to point at, and the thing being passed around
// was always the handle, so the glob, the reference to it, and the handle all
// collapse into one value here: `os.Stdout`, or whatever the program opened
// under that name. It returns nil for a glob that names something else, which
// is left to the rules that handle symbol-table assignment.
func (l *Lowerer) globHandle(name string, at ast.Node) ir.Expr {
	var out ir.Expr
	switch name {
	case "STDIN":
		out = ir.Pkg("os", "os", "Stdin", fileType)
	case "STDOUT":
		out = ir.Pkg("os", "os", "Stdout", fileType)
	case "STDERR":
		out = ir.Pkg("os", "os", "Stderr", fileType)
	default:
		b := l.findHandle(name)
		if b == nil || !(b.Type.Equal(fileType) || b.Type.Equal(pipeType)) {
			return nil
		}
		b.Reads++
		out = l.ident(b)
	}
	l.note(out, "A glob is a name in a symbol table and a glob reference points at "+
		"that name, which is how a handle was passed around before lexical "+
		"filehandles existed. Go has no symbol table to point at: the handle is an "+
		"ordinary value, so the value itself is what gets passed, and the two "+
		"spellings mean the same thing here.",
		"pointers-vs-references", "io-reader-writer")
	l.inform(at, "P2G6046", "a glob naming a filehandle",
		"`*"+name+"` names a handle, and the handle itself is what the generated code "+
			"passes, because Go has no symbol table for a reference to point into")
	return out
}

// findHandle returns the binding a bareword filehandle name already has,
// without creating one. A glob only names a handle if the program opened one
// under that name.
func (l *Lowerer) findHandle(name string) *Binding {
	key := varKey('$', name)
	if b, ok := l.scope.lookup(key); ok {
		return b
	}
	if b, ok := l.globalSeen[key]; ok {
		return b
	}
	return nil
}

// specialVar maps Perl's own variables onto their Go counterparts. It returns
// nil for anything that is an ordinary variable.
func (l *Lowerer) specialVar(v *ast.Var) ir.Expr {
	name := string(v.Sigil) + v.Name

	// The File::Find package variables are the walk's own state, which the
	// generated walk keeps in named arguments rather than in globals.
	if v.Sigil == '$' {
		if x, ok := l.findGlobal(v.Name); ok {
			return x
		}
		if x, ok := l.programDir(v.Name); ok {
			return x
		}
	}

	// $& is the whole of the innermost match, which Go returns as group 0 of
	// the same submatch slice.
	// A capture read where no match is in scope has nothing to name. Perl
	// answers with whatever the last successful match anywhere left behind,
	// which is a global the Go program does not have.
	if v.Sigil == '$' && (v.Name == "&" || (isDigits(v.Name) && v.Name != "0")) &&
		len(l.captureStack) == 0 {
		return l.strayCapture(v)
	}
	if v.Sigil == '$' && v.Name == "&" {
		if x, ok := l.wholeMatch(); ok {
			l.note(x, "$& is the text the last successful match consumed, and like $1 it "+
				"is global and lasts until the next match anywhere. Go returns it as the "+
				"first element of this match's own submatch slice, so there is nothing "+
				"global to leak.",
				"submatch-and-named-groups")
			return x
		}
	}

	// $1, $2 and so on name the capture groups of the innermost match that is
	// still in scope.
	if v.Sigil == '$' && isDigits(v.Name) && v.Name != "0" {
		n, _ := strconvAtoi(v.Name)
		if x, ok := l.captureVar(n); ok {
			l.note(x, "Perl leaves capture groups in the global variables $1, $2 and "+
				"so on, where they stay until the next successful match anywhere in the "+
				"program. Go returns them in a slice belonging to this match, so there "+
				"is nothing global to leak and nothing to reset.",
				"submatch-and-named-groups")
			return x
		}
	}

	// @_ inside a sub is the argument slice.
	if v.Sigil == '@' && v.Name == "_" && l.curSub != nil && l.curSub.VarArgs != nil {
		b := l.curSub.VarArgs
		b.Reads++
		return ir.NewIdent(b.Go, b.Type)
	}

	switch name {
	case "$_":
		if len(l.topicStack) > 0 {
			return l.topicStack[len(l.topicStack)-1]
		}
		b := l.special("$_", '$', "topic", v)
		if b.Type == nil || b.Type.Kind == ir.Any {
			b.Type = ir.TString
		}
		return l.ident(b)

	case "@ARGV":
		return l.ident(l.argv(v))

	case "$0":
		return index(ir.Pkg("os", "os", "Args", ir.SliceOf(ir.TString)), ir.IntLit("0"), ir.TString)

	case "$$":
		return call("os", "os", "Getpid", ir.TInt)

	case "$?":
		return l.ident(l.childStatus(v))

	case "$^X":
		return l.todoExpr(v, "P2G6504", "$^X",
			"the interpreter that ran the original is not here",
			"$^X is the path to the perl binary running the script, and a script "+
				"usually uses it to run more Perl. This program is not run by an "+
				"interpreter at all: os.Executable() names this binary, which is a "+
				"different thing and would not understand Perl handed to it.",
			"Decide what the child process should be now. Where the script was "+
				"re-running itself, a function call replaces the whole thing; where it "+
				"was running a helper script, name the helper.",
			"os-exec")

	case "$^O":
		out := ir.Pkg("runtime", "runtime", "GOOS", ir.TString)
		l.note(out, "$^O is the operating system the program was built for, which Go "+
			"records in runtime.GOOS. The spellings differ: Perl says darwin and "+
			"linux too, but MSWin32 where Go says windows.")
		return out

	case "$@":
		return l.ident(l.errText(v))

	case "$!":
		// Inside a failure branch, $! is the error the call actually returned.
		// Everywhere else there is nothing for it to name: Go keeps no global
		// record of the last failure.
		if l.errVar == "" && l.errFallback != "" {
			// The last call whose failure the program kept as a value, which
			// is what $! named at this point in the original too.
			out := l.helperCall(hErrnoText, ir.TString, ir.NewIdent(l.errFallback, ir.TError))
			l.note(out, "$! is a global holding the last system error, and the last "+
				"call that could have set it is the one above. Go has no such global: "+
				"the error is the value that call returned, still in scope here, so "+
				"this reads it by name.",
				"errors-are-values")
			return out
		}
		if l.errVar != "" {
			// $! held the errno and nothing else, while a Go error names the
			// operation and the file too, so the helper renders the one out of
			// the other and a message built around $! comes out reading as it
			// did before.
			out := l.helperCall(hErrnoText, ir.TString, ir.NewIdent(l.errVar, ir.TError))
			l.note(out, "$! is a global holding the last system error, which any "+
				"later call can overwrite before you read it. Go hands the error back "+
				"from the call that failed, so this one belongs to this call and to "+
				"nothing else. It also says more than $! did: the operation and the "+
				"path are in the error text, which is why the helper takes them back "+
				"out to keep this message the length it was.",
				"errors-are-values")
			return out
		}
		return l.todoExpr(v, "P2G6017", "$!",
			"the last system error is not available here",
			"$! holds the error from the most recent failed system call, globally "+
				"and until something else overwrites it. Go has no such variable: an "+
				"error is a value returned by the call that produced it, so outside the "+
				"branch that handled a failure there is nothing to read.",
			"Use the error returned by the call you are checking. Where the value "+
				"has to travel, wrap it with fmt.Errorf and %w so the original survives.",
			"errors-are-values", "error-wrapping")

	case "$.":
		b := l.lineCounter(v)
		out := l.ident(b)
		l.note(out, "$. is a global that every read updates, so it always refers to "+
			"whichever handle was read last. The counter here is an ordinary variable "+
			"the line-reading loops keep up to date, which says out loud what $. left "+
			"implicit.")
		l.approximate(v, "P2G6015", "$.",
			"the line counter is an ordinary variable",
			"$. is global and follows whichever handle was read most recently, so two "+
				"loops reading two files share it and it keeps its value after a loop "+
				"ends. The generated counter is one variable that every read loop "+
				"increments, which matches the common case of one loop at a time and not "+
				"the case of two interleaved reads.",
			"Where two handles are read in turn, give each loop its own counter.")
		return out

	case "$;", "$,", "$\\", "$/", "$\"", "$|":
		return l.todoExpr(v, "P2G6016", name,
			"this output formatting variable is not implemented",
			"Perl has global variables that change how print and split behave: the "+
				"output field separator, the record separator, the list separator, and "+
				"output buffering. Go has no global state of that kind; each call says "+
				"what it does.",
			"Pass the separator to the call that needs it. strings.Join takes one, "+
				"and bufio.Writer with an explicit Flush replaces $| entirely.")
	}

	// %+ is every named capture of the innermost match still in scope. Go
	// keeps the groups in a slice, so the hash is built from the names the
	// pattern declared.
	if v.Sigil == '%' && v.Name == "+" {
		for i := len(l.captureStack) - 1; i >= 0; i-- {
			frame := l.captureStack[i]
			if len(frame.Named) == 0 {
				continue
			}
			names := make([]string, 0, len(frame.Named))
			for name := range frame.Named {
				names = append(names, name)
			}
			sort.Strings(names)
			var keys, vals []ir.Expr
			for _, name := range names {
				keys = append(keys, ir.Str(quote(name)))
				vals = append(vals, l.helperCall(hAt, ir.TString,
					ir.NewIdent(frame.Name, ir.SliceOf(ir.TString)), ir.IntLit(itoa(frame.Named[name]))))
			}
			out := composite(ir.MapOf(ir.TString), keys, vals)
			l.note(out, "%+ holds the named captures of the last match, and it is global "+
				"and short-lived. Go returns the groups in a slice belonging to this "+
				"match, so the names have to be written down: the pattern declared them, "+
				"and they are known at conversion time.",
				"submatch-and-named-groups")
			return out
		}
	}
	return nil
}

// strayCapture stands in for $1, $2 or $& read where no match is in scope.
//
// Perl's capture variables are globals that survive the match that filled
// them, until the end of the enclosing block or the next successful match
// anywhere in the program. Go has nowhere to keep that: the submatch slice
// belongs to the call that produced it and is a local of the block that call
// was in.
func (l *Lowerer) strayCapture(v *ast.Var) ir.Expr {
	return l.todoExpr(v, "P2G4110", "$"+v.Name+" outside its match",
		"this capture is read where no match is in scope",
		"Perl leaves the captures in globals that outlive the match, so $"+v.Name+
			" can be read after the block that matched, and a failed match leaves the "+
			"previous values standing. Go returns the groups from the call that made "+
			"them, and that slice is a local of the block it was declared in.",
		"Keep what the match found in a variable of your own, declared before the "+
			"block that matches, and assign to it inside. That is what the original "+
			"was relying on the interpreter to do.",
		"submatch-and-named-groups", "nil-vs-undef")
}

// strconvAtoi is a tiny decimal parser, used where the input is known to be
// digits already.
func strconvAtoi(s string) (int, bool) {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, true
}

// myExpr lowers a bare `my` used as an expression, which happens when it is
// not the left side of an assignment: `my $x;` on its own.
func (l *Lowerer) myExpr(n *ast.My) ir.Expr {
	// The statement layer handles the declaring forms. Reaching here means the
	// declaration was used for its value, which is always undef.
	for _, v := range n.Vars {
		if vv, ok := v.(*ast.Var); ok {
			l.declare(vv, KindLocal)
		}
	}
	return ir.Nil(ir.TAny)
}
