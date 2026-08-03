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
			return &ir.Type{Kind: ir.Func, Params: a.Params, Results: a.Results, Variadic: a.Variadic}
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
	if a.Kind == ir.Pointer && b.Kind == ir.Pointer {
		// Two pointers to different named types have no pointer type in
		// common: *any is a pointer to an interface, which nothing assigns to.
		elem := join(a.Elem, b.Elem)
		if elem == nil || elem.Kind == ir.Any {
			return unresolved
		}
		return ir.PointerTo(elem)
	}

	return unresolved
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
