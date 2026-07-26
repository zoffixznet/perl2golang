package lower

import (
	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// This file lowers the ways Perl reaches inside a container: array and hash
// element access, slices, dereferences, and reference construction.

// indexParts resolves an array element access into the container, the index,
// and the element type. It returns a nil container when the shape is not one
// the converter understands.
func (l *Lowerer) indexParts(n *ast.Index) (base ir.Expr, idx ir.Expr, elem *ir.Type) {
	if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' {
		// $a[0] reads element 0 of @a: the sigil describes the element, not
		// the container, which is one of the first things to unlearn.
		b := l.lookup('@', v.Name, v)
		base = l.ident(b)
	} else {
		base = l.expr(n.Base)
	}
	if base == nil {
		return nil, nil, ir.TAny
	}
	elem = elemOf(typeOrAny(base))
	idx = l.toInt(l.expr(n.Idx), n.Idx)
	return base, idx, elem
}

// indexExpr lowers $a[i] and $ref->[i].
func (l *Lowerer) indexExpr(n *ast.Index) ir.Expr {
	base, idx, elem := l.indexParts(n)
	if base == nil {
		return ir.Nil(ir.TAny)
	}

	// A literal negative index counts from the end, which Go spells out.
	if lit, ok := idx.(*ir.Lit); ok && lit.Kind == ir.LitInt && len(lit.Value) > 0 && lit.Value[0] == '-' {
		off := ir.Bin("-", lenOf(base), ir.IntLit(lit.Value[1:]), ir.TInt)
		out := index(base, off, elem)
		l.note(out, "A negative Perl index counts back from the end. Go has no such "+
			"rule, so the arithmetic is written out. Note that Perl returns undef for "+
			"an index that is out of range while Go panics, which is louder and, on "+
			"balance, kinder.",
			"slices-not-arrays")
		return out
	}

	out := index(base, idx, elem)
	if _, ok := n.Idx.(*ast.NumberLit); !ok {
		l.note(out, "Reading past the end of a Perl array gives undef. Go panics with "+
			"an index out of range instead, which turns a silent wrong answer into an "+
			"immediate stack trace.",
			"slices-not-arrays")
	}
	return out
}

// hashParts resolves a hash element access into the map, the key, and the
// value type.
func (l *Lowerer) hashParts(n *ast.HashIndex) (m ir.Expr, key ir.Expr, elem *ir.Type) {
	if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' && v.Name == "+" {
		// $+{name} reads a named capture group.
		if k, ok := staticString(n.Key); ok {
			if x, found := l.namedCapture(k); found {
				l.note(x, "A named capture is still just a numbered group in Go. "+
					"regexp.SubexpIndex looks the number up by name when the pattern is "+
					"not known at conversion time.",
					"submatch-and-named-groups")
				return x, nil, ir.TString
			}
		}
	}
	if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' {
		if v.Name == "ENV" {
			k := l.toStr(l.expr(n.Key), n.Key)
			out := call("os", "os", "Getenv", ir.TString, k)
			l.note(out, "Perl's %ENV is a hash view of the environment. Go reads one "+
				"variable at a time with os.Getenv, which returns the empty string when "+
				"the name is not set.")
			return out, nil, ir.TString
		}
		b := l.lookup('%', v.Name, v)
		m = l.ident(b)
	} else {
		m = l.expr(n.Base)
	}
	if m == nil {
		return nil, nil, ir.TAny
	}
	elem = elemOf(typeOrAny(m))
	key = l.toStr(l.expr(n.Key), n.Key)
	return m, key, elem
}

// hashExpr lowers $h{k} and $ref->{k}.
func (l *Lowerer) hashExpr(n *ast.HashIndex) ir.Expr {
	m, key, elem := l.hashParts(n)
	if m == nil {
		return ir.Nil(ir.TAny)
	}
	if key == nil {
		// %ENV, already fully lowered.
		return m
	}
	out := index(m, key, elem)
	l.note(out, "Reading a key that is not in a Go map returns the value type's zero "+
		"value rather than undef, and it does not create the key the way Perl's "+
		"autovivification can. Use the two-result form when you need to tell a "+
		"missing key from a stored zero.",
		"comma-ok-idiom", "nil-slices-vs-nil-maps")
	return out
}

// sliceExpr lowers @a[...] and @h{...}.
func (l *Lowerer) sliceExpr(n *ast.Slice) ir.Expr {
	var container ir.Expr
	if v, ok := n.Base.(*ast.Var); ok {
		sig := '@'
		if n.Hash {
			sig = '%'
		}
		container = l.ident(l.lookup(sig, v.Name, v))
	} else {
		container = l.expr(n.Base)
	}
	if container == nil {
		return ir.Nil(ir.TAny)
	}
	elem := elemOf(typeOrAny(container))

	var elems []ir.Expr
	for _, ie := range n.Idx {
		for _, one := range flatten(ie) {
			if n.Hash {
				elems = append(elems, index(container, l.toStr(l.expr(one), one), elem))
				continue
			}
			elems = append(elems, index(container, l.toInt(l.expr(one), one), elem))
		}
	}
	out := composite(ir.SliceOf(elem), nil, elems)
	l.note(out, "A Perl slice picks several elements at once and yields a list. Go "+
		"has no such syntax for scattered indices, so the elements are gathered into "+
		"a slice literal.",
		"slices-not-arrays")
	return out
}

// derefExpr lowers $$r, @$r, %$r and their braced forms.
//
// Perl needs an explicit dereference because a reference is a distinct kind of
// scalar. Go's slices and maps are already reference-like, so most of these
// disappear entirely, which is worth saying out loud.
func (l *Lowerer) derefExpr(n *ast.Deref) ir.Expr {
	x := l.expr(n.X)
	if x == nil {
		return ir.Nil(ir.TAny)
	}
	t := typeOrAny(x)
	switch n.Sigil {
	case '@', '%':
		if t.Kind == ir.Slice || t.Kind == ir.Map {
			l.note(x, "Perl needs @{...} or %{...} to get at what a reference points "+
				"to. A Go slice or map value already refers to its data, so the "+
				"dereference has nothing to do and disappears.",
				"pointers-vs-references")
			return x
		}
	case '$':
		if t.Kind == ir.Pointer {
			out := ir.Un("*", x, t.Elem)
			l.note(out, "Go spells a pointer dereference with a leading *, and it is "+
				"only needed for real pointers. Slices and maps do not need one.",
				"pointers-vs-references")
			return out
		}
		return x
	case '#':
		return ir.Bin("-", lenOf(x), ir.IntLit("1"), ir.TInt)
	case '&':
		return x
	}
	return x
}

// refGen lowers \EXPR.
func (l *Lowerer) refGen(n *ast.RefGen) ir.Expr {
	switch inner := n.X.(type) {
	case *ast.Var:
		switch inner.Sigil {
		case '@', '%':
			x := l.expr(inner)
			l.note(x, "Perl takes a reference here because an array or hash flattens "+
				"when it is passed or nested. Go slices and maps already carry a "+
				"reference to their data, so passing the value itself is enough, and "+
				"the callee sees the same elements.",
				"pointers-vs-references", "slice-aliasing-and-copy")
			return x
		case '&':
			if s, ok := l.subs[inner.Name]; ok {
				out := ir.NewIdent(s.Go, nil)
				l.note(out, "A code reference is just the function value in Go. "+
					"Functions are ordinary values that can be stored, passed and "+
					"returned.",
					"variadic-and-no-defaults")
				return out
			}
		case '$':
			b := l.lookup('$', inner.Name, inner)
			out := ir.Un("&", l.ident(b), ir.PointerTo(b.Type))
			l.note(out, "Go's & takes the address of a variable, which is the closest "+
				"thing to a scalar reference. Unlike Perl, the pointer type is part of "+
				"the variable's type, and nil is a possible value.",
				"pointers-vs-references", "nil-vs-undef")
			return out
		}
	}
	x := l.expr(n.X)
	if x == nil {
		return ir.Nil(ir.TAny)
	}
	if t := typeOrAny(x); t.Kind == ir.Slice || t.Kind == ir.Map {
		return x
	}
	return ir.Un("&", x, ir.PointerTo(typeOrAny(x)))
}

// anonSub lowers `sub { ... }`.
func (l *Lowerer) anonSub(n *ast.AnonSub) ir.Expr {
	saved := l.scope
	l.scope = newScope(saved)
	body := &ir.Block{Stmts: l.stmts(n.Body)}
	l.scope = saved

	out := funcLit(nil, nil, body)
	l.note(out, "An anonymous sub becomes a Go function literal. It closes over the "+
		"variables in scope exactly as Perl's does, and it is an ordinary value: it "+
		"can be stored in a variable, put in a map, or passed to another function.",
		"closures-and-loop-capture")
	return out
}

// callRef lowers $code->(...) and &$code(...).
func (l *Lowerer) callRef(n *ast.FuncCallRef) ir.Expr {
	fn := l.expr(n.Ref)
	args, _ := l.listParts(n.Args)
	out := ir.CallOf(fn, ir.TAny, args...)
	l.note(out, "Calling through a code reference needs no arrow in Go: a variable "+
		"holding a function is called like any other function.")
	return out
}

// fileHandleExpr lowers a bareword filehandle used as a value.
func (l *Lowerer) fileHandleExpr(n *ast.FileHandle) ir.Expr {
	switch n.Name {
	case "STDOUT":
		return ir.Pkg("os", "os", "Stdout", ir.NamedType("*os.File", "os"))
	case "STDERR":
		return ir.Pkg("os", "os", "Stderr", ir.NamedType("*os.File", "os"))
	case "STDIN":
		return ir.Pkg("os", "os", "Stdin", ir.NamedType("*os.File", "os"))
	}
	b := l.lookup('$', n.Name, n)
	return l.ident(b)
}

// existsExpr lowers `exists`.
func (l *Lowerer) existsExpr(x ast.Expr, at ast.Node) ir.Expr {
	switch n := x.(type) {
	case *ast.HashIndex:
		m, key, elem := l.hashParts(n)
		if m != nil && key != nil {
			okName := l.tmp("ok")
			st := assign(":=", []ir.Expr{ir.NewIdent("_", nil), ir.NewIdent(okName, ir.TBool)},
				[]ir.Expr{indexComma(m, key, elem)})
			l.note(st, "Go's map index has a two-result form: the value, and whether "+
				"the key was present. That second result is the whole answer to exists, "+
				"and it is the idiom to reach for whenever a zero value would be "+
				"ambiguous.",
				"comma-ok-idiom")
			l.emit(st)
			return ir.NewIdent(okName, ir.TBool)
		}
	case *ast.Index:
		if base, idx, _ := l.indexParts(n); base != nil {
			return ir.Bin("<", idx, lenOf(base), ir.TBool)
		}
	}
	return l.toBool(l.expr(x), x)
}
