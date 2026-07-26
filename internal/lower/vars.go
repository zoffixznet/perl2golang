package lower

import (
	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
	"perl2go/internal/perl/token"
)

// declare creates (pass 1) or recovers (pass 2) the binding for a declaration
// site. Keying on the AST node means both passes agree about which record a
// name refers to, which is the whole reason the two-pass design works.
func (l *Lowerer) declare(v *ast.Var, kind Kind) *Binding {
	if b, ok := l.decls[v]; ok {
		l.scope.define(varKey(v.Sigil, v.Name), b)
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
	b := &Binding{
		Perl:  key,
		Sigil: sigil,
		Kind:  KindGlobal,
		Line:  posLine(at),
		Type:  defaultFor(sigil),
	}
	b.Go = l.names.take(goName(name))
	l.globalSeen[key] = b
	l.globals = append(l.globals, b)
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

// observe records a type the binding was seen holding. Only pass 1 collects;
// pass 2 reads the settled answer.
func (l *Lowerer) observe(b *Binding, t *ir.Type) {
	if b == nil || t == nil || l.pass != 1 {
		return
	}
	if t.Kind == ir.Void || t.Kind == ir.Invalid {
		return
	}
	b.Evidence = append(b.Evidence, t)
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
		if s, ok := l.subs[v.Name]; ok {
			return ir.NewIdent(s.Go, nil)
		}
	}

	if x := l.specialVar(v); x != nil {
		return x
	}

	b := l.lookup(v.Sigil, v.Name, v)
	return l.ident(b)
}

// specialVar maps Perl's own variables onto their Go counterparts. It returns
// nil for anything that is an ordinary variable.
func (l *Lowerer) specialVar(v *ast.Var) ir.Expr {
	name := string(v.Sigil) + v.Name

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
		b := l.lookup('$', "_topic", v)
		b.Perl = "$_"
		if b.Type == nil || b.Type.Kind == ir.Any {
			b.Type = ir.TString
		}
		return l.ident(b)

	case "@ARGV":
		// @ARGV is an ordinary array in Perl: scripts shift it, sort it and
		// assign to it. os.Args[1:] is an expression rather than a variable,
		// so the arguments get a real variable of their own.
		b := l.lookup('@', "args", v)
		b.Perl = "@ARGV"
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
		return l.ident(b)

	case "$0":
		return index(ir.Pkg("os", "os", "Args", ir.SliceOf(ir.TString)), ir.IntLit("0"), ir.TString)

	case "$$":
		return call("os", "os", "Getpid", ir.TInt)

	case "$@":
		b := l.lookup('$', "_err", v)
		b.Perl = "$@"
		b.Type = ir.TString
		return l.ident(b)

	case "$!":
		// Inside a failure branch, $! is the error the call actually returned.
		// Everywhere else there is nothing for it to name: Go keeps no global
		// record of the last failure.
		if l.errVar != "" {
			out := ir.NewIdent(l.errVar, ir.TError)
			l.note(out, "$! is a global holding the last system error, which any "+
				"later call can overwrite before you read it. Go hands the error back "+
				"from the call that failed, so this one belongs to this open and to "+
				"nothing else.",
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

	if v.Sigil == '%' && v.Name == "ENV" {
		return nil
	}
	return nil
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
