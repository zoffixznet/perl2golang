package lower

import "perl2go/internal/ir"

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

// join returns the least type that can hold both a and b.
func join(a, b *ir.Type) *ir.Type {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case a.Equal(b):
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
		return ir.PointerTo(join(a.Elem, b.Elem))
	}

	// A void or invalid contribution carries no information.
	if a.Kind == ir.Void || a.Kind == ir.Invalid {
		return b
	}
	if b.Kind == ir.Void || b.Kind == ir.Invalid {
		return a
	}

	return ir.TAny
}

// joinAll folds join over a list of observations.
func joinAll(ts []*ir.Type) *ir.Type {
	var out *ir.Type
	for _, t := range ts {
		out = join(out, t)
	}
	return out
}

// isDynamic reports whether the type is the `any` fallback, which is what the
// scorecard counts.
func isDynamic(t *ir.Type) bool { return t == nil || t.Kind == ir.Any }

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
