package lower

import (
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file lowers the ways Perl reaches inside a container: array and hash
// element access, slices, dereferences, and reference construction.

// indexParts resolves an array element access into the container, the index,
// and the element type. It returns a nil container when the shape is not one
// the converter understands.
func (l *Lowerer) indexParts(n *ast.Index) (base ir.Expr, idx ir.Expr, elem *ir.Type) {
	// $_[n] inside a sub reads the argument list, which is a parameter here
	// and not a variable the file declared.
	if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' && v.Name == "_" &&
		l.curSub != nil && l.curSub.VarArgs != nil {
		b := l.curSub.VarArgs
		b.Reads++
		base = ir.NewIdent(b.Go, b.Type)
		elem = elemOf(typeOrAny(base))
		idx = l.toInt(l.expr(n.Idx), n.Idx)
		return base, idx, elem
	}
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
	base = l.asSlice(base, n)
	elem = elemOf(typeOrAny(base))
	idx = l.toInt(l.expr(n.Idx), n.Idx)
	return base, idx, elem
}

// indexExpr lowers $a[i] and $ref->[i].
func (l *Lowerer) indexExpr(n *ast.Index) ir.Expr {
	// $_[0] inside a method is the object the method was called on, which the
	// receiver already names.
	if isFirstArg(n) && l.curSub != nil && l.curSub.Recv != nil {
		out := l.ident(l.curSub.Recv)
		l.note(out, "A Perl method finds its object in $_[0], because a method call is "+
			"a sub call with the object prepended. Go names it in the receiver, before "+
			"the method name, so there is nothing to index.",
			"methods-and-receivers")
		return out
	}
	base, idx, elem := l.indexParts(n)
	if base == nil {
		return ir.Nil(ir.TAny)
	}

	// A literal negative index counts from the end, which Go spells out.
	if text, neg := negativeLiteral(n.Idx); neg {
		off := ir.Bin("-", lenOf(base), ir.IntLit(text), ir.TInt)
		out := index(base, off, elem)
		l.note(out, "A negative Perl index counts back from the end. Go has no such "+
			"rule, so the arithmetic is written out. Note that Perl returns undef for "+
			"an index that is out of range while Go panics, which is louder and, on "+
			"balance, kinder.",
			"slices-not-arrays")
		return out
	}

	// Reading an argument that was not passed is not an error in Perl, and a
	// sub that takes its arguments as a list is written expecting that, so
	// the read goes through the helper that answers with a zero value.
	if v, ok := n.Base.(*ast.Var); ok && v.Name == "_" && l.curSub != nil && l.curSub.VarArgs != nil {
		out := l.helperCall(hAt, elem, base, idx)
		l.note(out, "A caller may pass fewer arguments than the body reads, which Perl "+
			"answers with undef and Go answers by panicking. The helper gives the zero "+
			"value instead, which is what the original did.",
			"slices-not-arrays", "variadic-and-no-defaults")
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

// negativeLiteral reports whether an index is a negative constant, and returns
// its magnitude. Go has no negative index, so the arithmetic has to be written
// out, and Go rejects a negative constant index outright rather than wrapping.
func negativeLiteral(e ast.Expr) (string, bool) {
	switch n := e.(type) {
	case *ast.NumberLit:
		if strings.HasPrefix(n.Text, "-") {
			return n.Text[1:], true
		}
	case *ast.UnOp:
		if n.Op == "-" {
			if inner, ok := n.X.(*ast.NumberLit); ok {
				return inner.Text, true
			}
		}
	}
	return "", false
}

// hashParts resolves a hash element access into the map, the key, and the
// value type.
func (l *Lowerer) hashParts(n *ast.HashIndex) (m ir.Expr, key ir.Expr, elem *ir.Type) {
	m, key, elem, _ = l.hashPartsField(n)
	return m, key, elem
}

// hashPartsField is hashParts plus the struct field it resolved, when the
// container turned out to be an object rather than a hash. A nil key means the
// place is the whole expression: a field, a parameter, or an environment
// variable, none of which is a lookup inside a container.
func (l *Lowerer) hashPartsField(n *ast.HashIndex) (m ir.Expr, key ir.Expr, elem *ir.Type, field *ClassField) {
	// $args{name} inside a constructor is the parameter that named argument
	// turned into, and never a hash at all.
	if b, ok := l.namedArgRead(n); ok {
		return l.ident(b), nil, b.Type, nil
	}
	if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' && v.Name == "+" {
		// $+{name} reads a named capture group.
		if k, ok := staticString(n.Key); ok {
			if x, found := l.namedCapture(k); found {
				l.note(x, "A named capture is still just a numbered group in Go. "+
					"regexp.SubexpIndex looks the number up by name when the pattern is "+
					"not known at conversion time.",
					"submatch-and-named-groups")
				return x, nil, ir.TString, nil
			}
		}
	}
	switch {
	case isFirstArg(n.Base) && l.curSub != nil && l.curSub.Recv != nil:
		// $_[0]{field} is the compact way a method reaches its own object.
		m = l.ident(l.curSub.Recv)
	default:
		v, isVar := n.Base.(*ast.Var)
		if isVar && !n.Arrow && v.Sigil == '$' {
			if v.Name == "ENV" {
				k := l.toStr(l.expr(n.Key), n.Key)
				out := call("os", "os", "Getenv", ir.TString, k)
				l.note(out, "Perl's %ENV is a hash view of the environment. Go reads one "+
					"variable at a time with os.Getenv, which returns the empty string when "+
					"the name is not set.")
				return out, nil, ir.TString, nil
			}
			b := l.lookup('%', v.Name, v)
			m = l.ident(b)
		} else {
			m = l.expr(n.Base)
		}
	}
	if m == nil {
		return nil, nil, ir.TAny, nil
	}
	// The container is lowered before anything decides what it is, because a
	// blessed hash reference and a plain one look the same in the source and
	// only the value's type tells them apart.
	if c := l.classOf(typeOrAny(m)); c != nil {
		if k, ok := staticString(n.Key); ok {
			f := c.field(k)
			if f == nil && l.pass == 1 {
				f = l.declareField(c, k, n)
			}
			if f != nil {
				l.fieldAt[n] = f
				return selector(m, f.Go, f.Type), nil, f.Type, f
			}
			return nil, nil, ir.TAny, nil
		}
		// A struct field cannot be reached by a computed name, so the type
		// gets a reader that switches over the names it has. That is what a
		// Go program writes instead of reaching for reflection.
		if c.Record {
			fn := l.recordFieldByName(c)
			out := ir.CallOf(ir.NewIdent(fn, nil), ir.TAny, m, l.toStr(l.expr(n.Key), n.Key))
			l.approximate(n, "P2G7048", "a computed key on a record",
				"the field name is worked out at run time",
				"The record's field names are written into the program, and this read "+
					"names one of them with a value. Go has no way to reach a field by a "+
					"computed name, so the type carries a reader that switches over the "+
					"names it has.",
				"Reading a field by name loses its type: what comes back is `any`, "+
					"because the fields do not all hold the same kind of thing.",
				"structs-and-embedding", "compile-time-mindset")
			return out, nil, ir.TAny, nil
		}
		if l.pass == 2 {
			l.approximate(n, "P2G7048", "a computed key on an object",
				"the field name is worked out at run time",
				"Perl objects are hashes, so a field can be reached by a name the "+
					"program computes. A Go struct's fields are fixed when it is compiled.",
				"Where the set of names is small, a switch over them reads well. Where it "+
					"is open-ended, that part of the object wants to be a map field.",
				"structs-and-embedding")
		}
		return nil, nil, ir.TAny, nil
	}
	m = l.asMap(m, n)
	elem = elemOf(typeOrAny(m))
	key = l.toStr(l.expr(n.Key), n.Key)
	l.noteDeepRead(n)
	return m, key, elem, nil
}

// noteDeepRead reports the side effect a nested read has in Perl and does not
// have here.
//
// `if ($h{a}{b}{c})` looks like a question and is also an answer: perl creates
// $h{a} and $h{a}{b} on the way through, so `exists $h{a}` is true afterwards
// even though nothing was ever assigned. A Go map read creates nothing. The Go
// behaviour is nearly always the one the author wanted, and it is still a
// difference a program can see.
func (l *Lowerer) noteDeepRead(n *ast.HashIndex) {
	if l.pass != 2 {
		return
	}
	switch n.Base.(type) {
	case *ast.HashIndex, *ast.Index:
	default:
		return
	}
	l.approximate(n, "P2G2510", "a nested read",
		"reading a nested key no longer creates the levels above it",
		"Reading a deep path in Perl creates every level on the way to it, so a test "+
			"that only looked at a key leaves that key behind. Go's map read creates "+
			"nothing at all, so a later `exists` on the same path answers no where the "+
			"original answered yes.",
		"Nothing to change unless the program relied on it. Where it did, assign the "+
			"intermediate levels explicitly, which also says out loud what was happening.",
		"nil-slices-vs-nil-maps", "comma-ok-idiom")
}

// asMap makes a value usable as a map.
//
// A nested Perl structure often leaves inference with nothing to go on, and
// the value lands as `any`. Go will not index an interface value, so the type
// has to be asserted. The assertion panics when the value is something else,
// which is exactly the moment the developer wants to hear about it.
func (l *Lowerer) asMap(x ir.Expr, at ast.Node) ir.Expr {
	if typeOrAny(x).Kind != ir.Any {
		return x
	}
	want := ir.MapOf(ir.TAny)
	out := &ir.TypeAssert{X: x, Assert: want}
	out.T = want
	l.note(out, "This value's type did not resolve, so it is held as `any`. Go will "+
		"not index an interface value: the assertion says what it is expected to be, "+
		"and panics on the spot if it turns out to be something else. The two-result "+
		"form of an assertion asks the same question without panicking.",
		"type-assertions-and-switches")
	return out
}

// asSlice makes a value usable as a slice, for the same reason as asMap.
func (l *Lowerer) asSlice(x ir.Expr, at ast.Node) ir.Expr {
	if typeOrAny(x).Kind != ir.Any {
		return x
	}
	want := ir.SliceOf(ir.TAny)
	out := &ir.TypeAssert{X: x, Assert: want}
	out.T = want
	l.note(out, "A value declared as `any` cannot be indexed directly. The assertion "+
		"states what it should be; if the program is right, it costs nothing, and if "+
		"it is wrong, it stops here rather than further along.",
		"type-assertions-and-switches")
	return out
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
	// A slice of a struct's fields is not a lookup at all: each name picks a
	// field, and the fields have different types, so what comes back is a
	// list of values rather than a slice of one element type.
	if n.Hash {
		if x, ok := l.fieldSlice(n, container); ok {
			return x
		}
	}
	if n.Hash {
		container = l.asMap(container, n)
	} else {
		container = l.asSlice(container, n)
	}
	elem := elemOf(typeOrAny(container))

	// An index that is itself a list, such as the range in @a[0 .. $n] or the
	// array in @h{@wanted}, decides how many elements the slice has at run
	// time. That cannot be written out as a literal, so those go through a
	// helper that walks the index list instead.
	var indexes []ast.Expr
	for _, ie := range n.Idx {
		indexes = append(indexes, flatten(ie)...)
	}
	if parts, dynamic := l.sliceIndexes(indexes, n.Hash); dynamic {
		return l.pickElements(container, parts, elem, n.Hash)
	}

	var elems []ir.Expr
	for _, one := range indexes {
		if n.Hash {
			elems = append(elems, index(container, l.toStr(l.expr(one), one), elem))
			continue
		}
		if text, neg := negativeLiteral(one); neg {
			elems = append(elems, index(container,
				ir.Bin("-", lenOf(container), ir.IntLit(text), ir.TInt), elem))
			continue
		}
		elems = append(elems, index(container, l.toInt(l.expr(one), one), elem))
	}
	out := composite(ir.SliceOf(elem), nil, elems)
	l.note(out, "A Perl slice picks several elements at once and yields a list. Go "+
		"has no such syntax for scattered indices, so the elements are gathered into "+
		"a slice literal.",
		"slices-not-arrays")
	return out
}

// sliceIndexes lowers the index expressions of a slice into values of the one
// type an index has, reporting whether any of them is a list rather than a
// single index.
func (l *Lowerer) sliceIndexes(indexes []ast.Expr, hash bool) ([]ir.Expr, bool) {
	want := ir.TInt
	if hash {
		want = ir.TString
	}
	var parts []ir.Expr
	dynamic := false
	for _, one := range indexes {
		x := l.expr(one)
		if x == nil {
			parts = append(parts, ir.Nil(want))
			continue
		}
		if t := typeOrAny(x); t.Kind == ir.Slice {
			dynamic = true
			if !elemOf(t).Equal(want) {
				helper := hIntList
				if hash {
					helper = hStrList
				}
				x = l.helperCall(helper, ir.SliceOf(want), x)
			}
			parts = append(parts, x)
			continue
		}
		if hash {
			parts = append(parts, l.toStr(x, one))
			continue
		}
		parts = append(parts, l.toInt(x, one))
	}
	return parts, dynamic
}

// pickElements builds the slice whose indices are only known at run time.
func (l *Lowerer) pickElements(container ir.Expr, parts []ir.Expr, elem *ir.Type, hash bool) ir.Expr {
	want, helper := ir.TInt, hPick
	if hash {
		want, helper = ir.TString, hPickKeys
	}
	out := l.helperCall(helper, ir.SliceOf(elem), container, l.listValue(parts, want))
	l.note(out, "A Perl slice picks several elements at once and yields a list. When "+
		"the indices are themselves a list, as a range or an array is, how many "+
		"elements come back is not known until the program runs, so Go walks the "+
		"index list rather than writing the elements out.",
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
			if s, ok := l.findSub(inner.Name); ok {
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
	return l.addressOf(x, n)
}

// addressOf takes the address of a value, giving it a name first when it does
// not already have one.
//
// Perl takes a reference to anything, a literal included. Go's & needs
// something that lives somewhere: a constant has no address, so `\'x'` becomes
// a variable holding "x" and a pointer to that.
func (l *Lowerer) addressOf(x ir.Expr, at ast.Node) ir.Expr {
	switch x.(type) {
	case *ir.Ident, *ir.Selector, *ir.Index, *ir.CompositeLit:
		return ir.Un("&", x, ir.PointerTo(typeOrAny(x)))
	}
	t := typeOrAny(x)
	name := l.tmp("held")
	decl := &ir.DeclStmt{Names: []string{name}, Type: t, Values: []ir.Expr{x}}
	l.setProv(decl, at)
	l.note(decl, "Go's & needs something that lives somewhere, and a constant does "+
		"not: only a variable, a field or an element has an address. Perl takes a "+
		"reference to anything, so the value is given a name first.",
		"pointers-vs-references")
	l.emit(decl)
	return ir.Un("&", ir.NewIdent(name, t), ir.PointerTo(t))
}

// anonSub lowers `sub { ... }`.
//
// An anonymous sub gets the same treatment a named one does: its parameters are
// recovered from the `my (...) = @_` at the top, and its result types are
// gathered on the first pass and committed to on the second. Without that a
// function literal has no signature worth the name, and everything that holds
// one, a variable, a map of callbacks, a returned closure, ends up as `any`.
func (l *Lowerer) anonSub(n *ast.AnonSub) ir.Expr {
	s := l.anonSubs[n]
	if s == nil {
		s = &Sub{Name: "anon@" + itoa(posLine(n)), Line: posLine(n)}
		s.Go = s.Name
		if l.anonSubs == nil {
			l.anonSubs = map[*ast.AnonSub]*Sub{}
		}
		l.anonSubs[n] = s
		l.anonOrd = append(l.anonOrd, s)
	}

	savedScope, savedSub := l.scope, l.curSub
	l.scope = newScope(savedScope)
	l.scope.fn = s
	l.curSub = s
	// The body may hold literals of its own, and they decide for themselves
	// whether they hold a table of callbacks.
	savedUniform := l.uniformFn
	l.uniformFn = false
	if savedUniform {
		s.Uniform = true
	}
	if s.Uniform {
		l.uniformFn = true
	}
	params, rest := l.recoverParams(s, valueTail(n.Body))
	l.uniformFn = false
	if s.Uniform {
		// A closure sharing a slot answers with one value of no fixed type,
		// whatever its body happens to produce, or the collection could not
		// hold it beside the others.
		s.Results = []*ir.Type{ir.TAny}
	}
	body := l.markUnused(&ir.Block{Stmts: l.stmts(rest)})
	l.implicitReturn(s, body)
	l.ensureReturn(s, body)
	l.scope, l.curSub = savedScope, savedSub
	l.uniformFn = savedUniform

	// The signature the previous round settled is used from the second round
	// of discovery onwards, so that whatever holds the literal is inferred
	// from the shape the literal will actually have.
	out := funcLit(params, s.Results, body)
	if l.pass == 1 {
		s.LitType = out.Type()
	}
	l.note(out, "An anonymous sub becomes a Go function literal. It closes over the "+
		"variables in scope exactly as Perl's does, and it is an ordinary value: it "+
		"can be stored in a variable, put in a map, or passed to another function. "+
		"Unlike Perl's, it has a written-down signature, so the compiler checks every "+
		"call to it.",
		"closures-and-loop-capture")
	return out
}

// callRef lowers $code->(...) and &$code(...).
func (l *Lowerer) callRef(n *ast.FuncCallRef) ir.Expr {
	fn := l.expr(n.Ref)
	args, _ := l.listParts(n.Args)
	t := typeOrAny(fn)

	if t.Kind != ir.Func {
		// The value's type did not resolve to a function. Go will not call an
		// interface value, and an assertion would have to name the whole
		// signature, which is exactly what a table of callbacks does not
		// have in common. Asking the value what it takes is the only thing
		// that works for all of them.
		spread, ellipsis := l.spreadArgs(args, ir.TAny, n)
		out := l.helperCall(hCallFn, ir.TAny,
			append([]ir.Expr{l.assignable(fn, ir.TAny, n)}, spread...)...)
		out.Ellipsis = ellipsis
		l.approximate(n, "P2G7030", "call through a code reference",
			"the call goes through reflection because the signature is not known",
			"Nothing in the file pinned down what this reference points at, so it is "+
				"held in a value of no fixed type. Go will not call one, and an "+
				"assertion would have to name the whole signature, which a collection "+
				"of callbacks that differ in one argument does not have.",
			"Give the collection a function type where it is created. Every call "+
				"through it then becomes a direct one, checked when the program is "+
				"compiled and several times faster.",
			"type-assertions-and-switches", "closures-and-loop-capture")
		l.note(out, "Reflection is the escape hatch, not the idiom. It is here because "+
			"the value's type is not known; the moment it is, the call is written "+
			"straight and the compiler checks it.",
			"type-assertions-and-switches")
		return out
	}

	result := ir.TVoid
	if len(t.Results) == 1 {
		result = t.Results[0]
	}
	ellipsis := false
	if t.Variadic && len(t.Params) > 0 {
		args, ellipsis = l.spreadArgs(args, t.Params[len(t.Params)-1], n)
	}
	out := ir.CallOf(fn, result, args...)
	out.Ellipsis = ellipsis
	l.note(out, "Calling through a code reference needs no arrow in Go: a variable "+
		"holding a function is called like any other function.")
	return out
}

// spreadArgs prepares an argument list for a call whose target takes its
// arguments as one variadic slice, which every code reference does.
//
// Perl flattens every array in an argument list into @_ before the sub sees
// it. Go spreads exactly one slice and will not mix that with other arguments,
// so `$code->(@args)` becomes a spread, `$code->($x, @rest)` has its list
// built first, and a call with no arrays in it is passed as it stands.
func (l *Lowerer) spreadArgs(args []ir.Expr, elem *ir.Type, at ast.Node) ([]ir.Expr, bool) {
	lists := 0
	for _, a := range args {
		if typeOrAny(a).Kind == ir.Slice {
			lists++
		}
	}
	switch {
	case lists == 0:
		out := make([]ir.Expr, len(args))
		for i, a := range args {
			out[i] = l.assignable(a, elem, at)
		}
		return out, false
	case len(args) == 1:
		return []ir.Expr{l.assignable(args[0], ir.SliceOf(elem), at)}, true
	default:
		return []ir.Expr{l.flattenTail(args, elem, at)}, true
	}
}

// argTypes reads the types of an argument list, for building the function type
// a code reference has to be asserted to.
func argTypes(args []ir.Expr) []*ir.Type {
	out := make([]*ir.Type, len(args))
	for i, a := range args {
		out[i] = typeOrAny(a)
	}
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
