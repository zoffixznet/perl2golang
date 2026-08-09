package lower

import "perl2golang/internal/ir"

// This file holds the type lattice.
//
// A Perl scalar has no type: the same variable holds 7, "7", and "seven" over
// its lifetime, and the operator decides how to read it. Go has no such thing,
// so the converter has to pick one Go type per variable and insert explicit
// conversions where Perl was converting implicitly. That choice is the single
// biggest determinant of whether the generated code reads like Go.
//
// The lattice is small on purpose:
//
//	int  <  float64
//	int, float64, string, bool  <  any
//
// Widening int to float64 is safe and invisible. Every other join lands on
// `any`, which is the documented fallback: correct, compiles, and reported as
// a place where the tool did not understand the program well enough.

// unresolved is the `any` that means two observations disagreed, as opposed to
// the `any` that means nothing said anything.
//
// The difference matters when the disagreement is inside a container. A slice
// seen holding both a function and a number is a slice of `any` for good, and
// folding the next number into it must not narrow it back to a slice of
// numbers. Both spell `any` in the generated code; only the fold tells them
// apart.
var unresolved = &ir.Type{Kind: ir.Any}

// isUnresolved reports whether a type is the settled kind of `any`. It is the
// identity of the value that carries the distinction, not anything written
// down in it, so the two spell the same Go and compare equal everywhere else.
func isUnresolved(t *ir.Type) bool { return t == unresolved }

// join returns the least type that can hold both a and b.
func join(a, b *ir.Type) *ir.Type {
	// A disagreement already reached is final: nothing later can resolve it.
	if isUnresolved(a) || isUnresolved(b) {
		return unresolved
	}
	// A void or invalid contribution carries no information at all, so it is
	// treated as no observation rather than as a type to reconcile.
	//
	// `any` is treated the same way, and that is the single most important
	// rule here. It is not a type the program was seen holding: it is what
	// the converter says when it did not work out the type of an expression.
	// Joining it with a real observation the way a lattice would would let
	// one unresolved expression erase everything the rest of the file said,
	// and a nested structure has plenty of those. A variable really holding
	// two different kinds of thing still lands on `any`, because joining two
	// incompatible real observations is what produces it.
	if a != nil && (a.Kind == ir.Void || a.Kind == ir.Invalid || a.Kind == ir.Any) {
		a = nil
	}
	if b != nil && (b.Kind == ir.Void || b.Kind == ir.Invalid || b.Kind == ir.Any) {
		b = nil
	}
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case a.Equal(b):
		if a.Kind == ir.Func {
			// A function literal's type is refreshed in place once its
			// signature settles, because whatever holds a single literal was
			// inferred from that very object. A join is the other case: the
			// answer describes a collection of several literals and must not
			// go on tracking any one of them.
			return &ir.Type{Kind: ir.Func, Params: a.Params, Results: a.Results,
				Variadic: a.Variadic, Group: mergeSigGroups(a.Group, b.Group)}
		}
		return a
	}

	// Numeric widening is the one lossless direction.
	if a.Kind == ir.Int && b.Kind == ir.Float || a.Kind == ir.Float && b.Kind == ir.Int {
		return ir.TFloat
	}

	// Two collections of the same shape join elementwise.
	if a.Kind == ir.Slice && b.Kind == ir.Slice {
		return ir.SliceOf(join(a.Elem, b.Elem))
	}
	if a.Kind == ir.Map && b.Kind == ir.Map {
		return ir.MapOf(join(a.Elem, b.Elem))
	}
	// A scalar seen both as a value and as something that might be absent is
	// something that might be absent: `my $seen = $h{k}` copies out a *int and
	// `$seen = 0` puts a number in it, and only the pointer holds both.
	if isNullable(a) && isScalarKind(b) {
		if elem := join(a.Elem, b); elem != nil && !isUnresolved(elem) {
			return nullable(elem)
		}
	}
	if isNullable(b) && isScalarKind(a) {
		if elem := join(a, b.Elem); elem != nil && !isUnresolved(elem) {
			return nullable(elem)
		}
	}
	if a.Kind == ir.Pointer && b.Kind == ir.Pointer {
		// Two pointers to different named types have no pointer type in
		// common: *any is a pointer to an interface, which nothing assigns to.
		elem := join(a.Elem, b.Elem)
		if elem == nil || elem.Kind == ir.Any {
			return unresolved
		}
		return ir.PointerTo(elem)
	}

	// Two function types of the same shape are the same function seen at two
	// moments, not two functions. A sub's signature is discovered from its
	// body, so an earlier round of inference can see it as returning nothing,
	// or as taking and returning values whose own types had not settled, and a
	// later round can see the answer. Calling that a conflict throws away
	// everything the later round worked out, and leaves whatever holds the
	// closure as `any`, which does not compile at the call.
	if a.Kind == ir.Func && b.Kind == ir.Func {
		// Meeting in a join means the two functions share a slot, so their
		// signature groups merge whether or not their shapes reconcile
		// today: the group is what makes them reconcile tomorrow.
		g := mergeSigGroups(a.Group, b.Group)
		if t := joinFunc(a, b); t != nil {
			t.Group = g
			return t
		}
	}

	return unresolved
}

// joinFunc reconciles two function types, or reports that it cannot.
//
// Every position is joined on its own, with the same rule the top level uses:
// an `any` on one side is an unfinished answer rather than an observation, so
// the other side wins. Two unfinished answers stay unfinished.
func joinFunc(a, b *ir.Type) *ir.Type {
	if a.Variadic != b.Variadic || len(a.Params) != len(b.Params) {
		return nil
	}
	switch {
	case len(a.Results) == 0 && len(b.Results) > 0:
		return b
	case len(b.Results) == 0 && len(a.Results) > 0:
		return a
	case len(a.Results) != len(b.Results):
		return nil
	}
	fold := func(as, bs []*ir.Type) ([]*ir.Type, bool) {
		out := make([]*ir.Type, len(as))
		for i := range as {
			t := join(as[i], bs[i])
			switch {
			case t == nil:
				// Neither side had anything to say about this position.
				t = ir.TAny
			case isUnresolved(t):
				return nil, false
			}
			out[i] = t
		}
		return out, true
	}
	params, ok := fold(a.Params, b.Params)
	if !ok {
		return nil
	}
	results, ok := fold(a.Results, b.Results)
	if !ok {
		return nil
	}
	return &ir.Type{Kind: ir.Func, Params: params, Results: results, Variadic: a.Variadic}
}

// sameParams reports whether two function types take the same arguments.
func sameParams(a, b *ir.Type) bool {
	if len(a.Params) != len(b.Params) || a.Variadic != b.Variadic {
		return false
	}
	for i := range a.Params {
		if !a.Params[i].Equal(b.Params[i]) {
			return false
		}
	}
	return true
}

// joinAll folds join over a list of observations.
//
// The fold is not a plain reduction, because the two ways of arriving at `any`
// have to be kept apart. An observation that is itself `any` says nothing and
// is skipped. Two real observations that no single Go type covers are a
// genuine conflict, and that answer is final: folding further would let the
// next observation resurrect a type the file has already disproved, and a
// nested structure has plenty of observations to resurrect one with.
func joinAll(ts []*ir.Type) *ir.Type {
	var out *ir.Type
	for _, t := range ts {
		if t == nil || t.Kind == ir.Void || t.Kind == ir.Invalid || t.Kind == ir.Any {
			continue
		}
		if out == nil {
			out = t
			continue
		}
		next := join(out, t)
		if next == nil || next.Kind == ir.Any {
			return unresolved
		}
		out = next
	}
	return out
}

// isDynamic reports whether the type is the `any` fallback, which is what the
// scorecard counts.
func isDynamic(t *ir.Type) bool { return t == nil || t.Kind == ir.Any }

// stringlyResolve answers for the joins that failed only because text met
// numbers. Every Perl scalar has a faithful string form, so a slot seen
// holding "localhost" here and 8080 there is representable as a string with
// the numbers written in at their sources, which is exactly what Perl was
// doing silently. That is a far better answer than `any`: the reader gets a
// real type, truthiness and arithmetic already go through the Perl-faithful
// conversions, and nothing needs an assertion.
//
// It answers nil when the disagreement was about anything more than scalar
// kinds: a slot holding both a number and a hash, or a function and text,
// has no honest string form, and those stay dynamic.
func stringlyResolve(ts []*ir.Type) *ir.Type {
	scalars, slices, maps := 0, 0, 0
	var elems []*ir.Type
	for _, t := range ts {
		if t == nil || t.Kind == ir.Void || t.Kind == ir.Invalid || t.Kind == ir.Any {
			continue
		}
		switch t.Kind {
		case ir.Int, ir.Float, ir.String, ir.Bool:
			scalars++
		case ir.Slice:
			slices++
			elems = append(elems, t.Elem)
		case ir.Map:
			maps++
			elems = append(elems, t.Elem)
		default:
			return nil
		}
	}
	switch {
	case scalars > 0 && slices == 0 && maps == 0:
		return ir.TString
	case slices > 0 && scalars == 0 && maps == 0:
		if e := stringlyElem(elems); e != nil {
			return ir.SliceOf(e)
		}
	case maps > 0 && scalars == 0 && slices == 0:
		if e := stringlyElem(elems); e != nil {
			return ir.MapOf(e)
		}
	}
	return nil
}

// stringlyElem settles the element type of a run of containers whose
// top-level shapes agreed: the ordinary join when it holds, the string form
// when only scalar kinds disagreed, nothing otherwise.
func stringlyElem(elems []*ir.Type) *ir.Type {
	j := joinAll(elems)
	if j != nil && !isUnresolved(j) {
		return j
	}
	if j == nil {
		return nil
	}
	return stringlyResolve(elems)
}

// stringlyRescue applies stringlyResolve to a settled type that came out
// dynamic: at the top when the join itself failed, and inside a container
// whose element slot was the conflict.
func stringlyRescue(t *ir.Type, evidence []*ir.Type) *ir.Type {
	switch {
	case isUnresolved(t):
		if s := stringlyResolve(evidence); s != nil {
			return s
		}
	case t != nil && (t.Kind == ir.Slice || t.Kind == ir.Map) && isUnresolved(t.Elem):
		var elems []*ir.Type
		for _, ev := range evidence {
			if ev != nil && ev.Kind == t.Kind {
				elems = append(elems, ev.Elem)
			}
		}
		if e := stringlyResolve(elems); e != nil {
			if t.Kind == ir.Slice {
				return ir.SliceOf(e)
			}
			return ir.MapOf(e)
		}
	}
	return t
}

// isScalarKind reports whether a type is one of the four Go types a Perl scalar
// settles on. Those are the ones a pointer can usefully make optional: a slice,
// a map and an interface already have a nil of their own.
func isScalarKind(t *ir.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case ir.Int, ir.Float, ir.String, ir.Bool:
		return true
	}
	return false
}

// nullable is *T for a scalar T, and T unchanged for everything else.
//
// It is how a container that the program has put undef into spells its element
// type. Perl's undef is a value a hash or an array element can hold, and no Go
// scalar type has room for it: 0 in a map[string]int is a stored zero and
// nothing else. A map[string]*int has room, because nil is not 0.
func nullable(t *ir.Type) *ir.Type {
	if !isScalarKind(t) {
		return t
	}
	return ir.PointerTo(t)
}

// isNullable reports whether a type is the pointer-to-scalar shape nullable
// produces. A pointer to a named type is an object reference and not this.
func isNullable(t *ir.Type) bool {
	return t != nil && t.Kind == ir.Pointer && isScalarKind(t.Elem)
}

// isOrdered reports whether Go's < applies to the type, which is what the
// generic ordering functions in slices and cmp require.
func isOrdered(t *ir.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case ir.Int, ir.Float, ir.String:
		return true
	}
	return false
}

// isNum reports whether Go arithmetic applies to the type directly.
func isNum(t *ir.Type) bool { return t != nil && (t.Kind == ir.Int || t.Kind == ir.Float) }

// arithmeticResult is what an arithmetic operator leaves behind, given the type
// of the other operand.
//
// It exists because Perl's `+`, `-` and `*` are numeric operators and nothing
// else: `$total += $field` reads a number out of $field whatever $field was
// spelled as, and the total is a number either way. Recording the operand's own
// type as evidence for the total is what used to leave the accumulator in the
// most common loop in Perl holding `any`, on the grounds that it had been seen
// holding text, which it never had.
//
// Two whole numbers give a whole number and anything else gives a
// floating-point number, since text can carry a fraction and nothing rules that
// out. An operand whose own type has not resolved yet says nothing at all: the
// first discovery round sees most variables as untyped, and a guess made there
// would be evidence the later rounds could never take back.
func arithmeticResult(operand *ir.Type) *ir.Type {
	if operand == nil {
		return nil
	}
	switch operand.Kind {
	case ir.Int:
		return ir.TInt
	case ir.Any, ir.Invalid, ir.Void:
		return nil
	}
	return ir.TFloat
}

// isStr reports whether the type is a Go string.
func isStr(t *ir.Type) bool { return t != nil && t.Kind == ir.String }

// elemOf returns the element type of a slice or map, or any.
func elemOf(t *ir.Type) *ir.Type {
	if t != nil && (t.Kind == ir.Slice || t.Kind == ir.Map || t.Kind == ir.Pointer) && t.Elem != nil {
		return t.Elem
	}
	return ir.TAny
}

// defaultFor gives a starting type for a binding whose sigil is known but
// whose contents are not: an untyped scalar, a slice of dynamic values, or a
// map of dynamic values.
func defaultFor(sigil rune) *ir.Type {
	switch sigil {
	case '@':
		return ir.SliceOf(ir.TAny)
	case '%':
		return ir.MapOf(ir.TAny)
	default:
		return ir.TAny
	}
}

// scalarKind names the type for a diagnostic or a note, in words a Perl
// developer reads rather than Go syntax.
func typeWords(t *ir.Type) string {
	if t == nil {
		return "a value of unknown type"
	}
	switch t.Kind {
	case ir.Int:
		return "a whole number"
	case ir.Float:
		return "a floating-point number"
	case ir.String:
		return "text"
	case ir.Bool:
		return "a true or false value"
	case ir.Slice:
		return "a list of " + typeWords(t.Elem)
	case ir.Map:
		return "a map from text to " + typeWords(t.Elem)
	case ir.Any:
		return "a value whose type varies"
	case ir.Error:
		return "an error"
	}
	return t.String()
}
