package lower

import (
	"strings"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// This file holds the builtins that ask a value what it is at run time. Perl
// needs them because a scalar has no type; Go usually does not, because the
// compiler already knows. Where the converter worked out the type it answers
// the question at compile time, and where it did not it falls back to a helper
// that asks reflect, which is the honest translation of a question Perl could
// only answer at run time either.

// refCall lowers ref.
func (l *Lowerer) refCall(n *ast.Call) ir.Expr {
	x := l.argExpr(n, 0)
	node := l.argNode(n, 0)

	if kind, ok := staticRefKind(typeOrAny(x)); ok {
		out := ir.Str(quote(kind))
		l.note(out, "The converter already knows what this value is, so the question "+
			"ref asks is answered here rather than at run time. That is the usual "+
			"case in Go: a variable's type is part of its declaration.",
			"static-types-and-zero-values")
		l.approximate(n, "P2G7020", "ref",
			"ref is answered from the declared type",
			"ref reports what kind of thing a reference points at, because a Perl "+
				"scalar can hold any of them. This value's Go type is known, so the "+
				"answer is a constant.",
			"If the value really can be several different things, declare it as an "+
				"interface and use a type switch, which is Go's version of this question.",
			"type-assertions-and-switches")
		return out
	}

	out := l.helperCall(hRefKind, ir.TString, x)
	l.note(out, "The value is held in an any, so what it refers to is only known "+
		"while the program runs. The helper asks reflect and answers in Perl's "+
		"vocabulary. A type switch is the Go way to ask the same question, and it "+
		"gives you the typed value along with the answer.",
		"type-assertions-and-switches")
	l.approximate(n, "P2G7020", "ref",
		"ref is answered by inspecting the value at run time",
		"ref reports what kind of thing a reference points at. Go decides types at "+
			"compile time, so there is no built-in equivalent; the generated code uses "+
			"reflect to answer in the same words Perl uses.",
		"A type switch is the idiomatic form, and it hands you the typed value "+
			"rather than a string naming its kind. Perl's blessed-object answer, the "+
			"class name, has no equivalent here at all: this reports ARRAY, HASH, CODE, "+
			"SCALAR, REF, or the empty string.",
		"type-assertions-and-switches")
	_ = node
	return out
}

// undefCall lowers undef in both its forms: the bare value, and `undef $x`,
// which clears a variable.
//
// Perl's undef is a value a scalar can hold, distinct from 0 and from the empty
// string. Go has no such value: a variable of type string always holds a
// string, and the nearest thing is the type's zero value. Where the type is not
// known the value lands in an any, and there nil really does mean absent.
func (l *Lowerer) undefCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 0 {
		out := ir.Nil(ir.TAny)
		l.note(out, "Perl's undef is a value in its own right. The Go equivalent "+
			"depends on the type: nil for anything held in an any, and the zero value "+
			"for a variable with a concrete type. A string variable cannot be undef, "+
			"only empty.",
			"nil-vs-undef", "static-types-and-zero-values")
		l.concept("nil-vs-undef")
		return out
	}

	// `undef $x` clears the variable, which in Go is assigning its zero value.
	target := l.expr(args[0])
	t := typeOrAny(target)
	st := assign("=", []ir.Expr{target}, []ir.Expr{zeroOf(t)})
	l.setProv(st, n)
	l.note(st, "`undef $x` puts the variable back to holding nothing. Go has no "+
		"nothing to put there, so the variable goes back to its zero value: the "+
		"empty string, 0, or nil, depending on what it was declared as.",
		"nil-vs-undef", "static-types-and-zero-values")
	if t.Kind != ir.Any && t.Kind != ir.Slice && t.Kind != ir.Map && t.Kind != ir.Pointer {
		l.approximate(n, "P2G2115", "undef",
			"clearing a variable leaves its zero value, not undef",
			"undef makes the variable hold nothing at all, which `defined` can see. A "+
				"Go variable of type "+t.String()+" always holds a "+t.String()+", so the "+
				"cleared value is "+t.Zero(nil)+" and nothing can tell it apart from that "+
				"value having been assigned.",
			"If the difference matters, declare the variable as a pointer, where nil "+
				"means absent and the value has to be dereferenced to be read.",
			"nil-vs-undef")
	}
	l.emit(st)
	return ir.Nil(ir.TAny)
}

// localStmts lowers `local X` and `local X = VALUE`.
//
// local swaps a package variable's value for the duration of the enclosing
// block, and everything called from inside sees the new one. Go has only
// lexical scoping, so the swap is written out: keep the old value, install the
// new one, and put it back on the way out with defer. The one difference worth
// knowing is when "on the way out" happens, and it is reported.
func (l *Lowerer) localStmts(targets []ast.Expr, value ast.Expr, n ast.Node) []ir.Stmt {
	if len(targets) == 0 {
		return nil
	}
	if len(targets) > 1 {
		// Several at once is several localisations, and only the last of them
		// can take the value.
		var out []ir.Stmt
		for i, t := range targets {
			var v ast.Expr
			if i == len(targets)-1 {
				v = value
			}
			out = append(out, l.localStmts([]ast.Expr{t}, v, n)...)
		}
		return out
	}

	target := l.expr(targets[0])
	if !assignableTarget(target) {
		return []ir.Stmt{l.todoStmt(n, "P2G2001", "local",
			"local's dynamic scoping is not implemented",
			"local temporarily replaces a package variable's value for the duration "+
				"of the enclosing block, and everything called from inside sees the new "+
				"value. What is being localised here has no Go variable behind it, so "+
				"there is nothing to save and put back.",
			"Pass the value as an argument instead, which is what the code is really "+
				"doing.",
			"defer-timing")}
	}

	t := typeOrAny(target)
	saved := l.tmp("saved" + capitalise(defaultName(targets[0])))
	keep := assign(":=", []ir.Expr{ir.NewIdent(saved, t)}, []ir.Expr{target})
	l.setProv(keep, n)

	newValue := ir.Expr(zeroOf(t))
	if value != nil {
		newValue = l.assignable(l.expr(value), t, value)
	}
	install := assign("=", []ir.Expr{target}, []ir.Expr{newValue})

	restore := &ir.Defer{Call: ir.CallOf(
		funcLit(nil, nil, &ir.Block{Stmts: []ir.Stmt{
			assign("=", []ir.Expr{target}, []ir.Expr{ir.NewIdent(saved, t)}),
		}}), ir.TVoid)}

	l.note(keep, "local is a save and a restore that Perl performs for you. Written "+
		"out, it is three lines: keep the old value, install the new one, and put the "+
		"old one back with defer.",
		"defer-timing")
	l.approximate(n, "P2G2001", "local",
		"the old value comes back when the function returns, not when the block ends",
		"local restores the previous value at the end of the enclosing block. Go's "+
			"defer runs when the enclosing function returns, so if this local sits "+
			"inside a loop or an if, the restore happens later than it did in Perl, and "+
			"a local inside a loop stacks up one deferred restore per turn.",
		"Where the block boundary matters, do the restore explicitly at the end of "+
			"the block instead of deferring it.",
		"defer-timing")
	return []ir.Stmt{keep, install, restore}
}

// localTargets reads what a `local` declaration is localising. Unlike `my`,
// which can only introduce variables, local can name a hash or array element,
// so the expressions come out as written.
func localTargets(my *ast.My) []ast.Expr {
	var out []ast.Expr
	for _, v := range my.Vars {
		out = append(out, flatten(v)...)
	}
	return out
}

// assignableTarget reports whether an IR expression can stand on the left of an
// assignment.
func assignableTarget(x ir.Expr) bool {
	switch x.(type) {
	case *ir.Ident, *ir.Index, *ir.Selector:
		return true
	}
	return false
}

// capitalise upper-cases the first letter, for building a readable temporary
// name out of another name.
func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// posCall lowers pos, which reads how far a global match has walked through a
// scalar.
func (l *Lowerer) posCall(n *ast.Call) ir.Expr {
	b, ok := l.posSubject(n)
	if !ok {
		return l.todoExpr(n, "P2G4531", "pos",
			"pos on something other than a variable is not implemented",
			"pos reads the match position Perl keeps on a scalar. The argument here "+
				"is not a variable, so there is nothing that position could belong to.",
			"Assign the string to a variable first, then scan that.")
	}
	p := l.matchPos(b, n)
	out := l.ident(p)
	l.note(out, "This is the position a global match left behind. Perl keeps it on "+
		"the scalar, which is why pos takes a variable rather than a string; here it "+
		"is an ordinary int.")
	l.approximate(n, "P2G4531", "pos",
		"an unstarted scan now reads as 0, not undef",
		"pos returns undef for a string no global match has walked yet, which is how "+
			"`defined pos($s)` tells \"not started\" from \"at the beginning\". An int "+
			"has no undef, so both read as 0.",
		"Keep a separate bool where the difference matters.",
		"nil-vs-undef")
	return out
}

// posTarget resolves `pos($x)` on the left of an assignment.
func (l *Lowerer) posTarget(n *ast.Call) (ir.Expr, bool) {
	b, ok := l.posSubject(n)
	if !ok {
		return nil, false
	}
	p := l.matchPos(b, n)
	out := l.identFor(p)
	l.note(out, "Assigning to pos moves the cursor a global match will start from. "+
		"With the position held in a variable of its own, that is an ordinary "+
		"assignment.")
	return out, true
}

// posSubject reads the scalar a call to pos is about.
func (l *Lowerer) posSubject(n *ast.Call) (*Binding, bool) {
	args := flatten(argList(n))
	if len(args) == 0 {
		return nil, false
	}
	v, ok := args[0].(*ast.Var)
	if !ok || v.Sigil != '$' {
		return nil, false
	}
	return l.lookup(v.Sigil, v.Name, v), true
}

// staticRefKind answers ref from a type the converter already resolved. It
// declines for any, where nothing is known until the program runs.
func staticRefKind(t *ir.Type) (string, bool) {
	if t == nil {
		return "", false
	}
	switch t.Kind {
	case ir.Slice:
		return "ARRAY", true
	case ir.Map:
		return "HASH", true
	case ir.Func:
		return "CODE", true
	case ir.Int, ir.Float, ir.String, ir.Bool:
		return "", true
	case ir.Pointer:
		if t.Elem != nil {
			switch t.Elem.Kind {
			case ir.Slice, ir.Map, ir.Func, ir.Pointer:
				return "REF", true
			}
		}
		return "SCALAR", true
	}
	return "", false
}
