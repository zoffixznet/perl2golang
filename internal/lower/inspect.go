package lower

import (
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
