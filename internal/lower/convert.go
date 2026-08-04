package lower

import (
	"strconv"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file holds the coercions.
//
// Perl converts between text and numbers silently, at every operator, in both
// directions. Go converts nowhere. Every place the two disagree, an explicit
// conversion appears in the generated code, and that is the single most
// visible difference a Perl developer meets on their first day of Go. Each
// conversion the converter inserts therefore carries a note explaining what it
// is standing in for.

// call builds a call to a package-level function, registering the import.
func call(pkgPath, pkgName, sel string, ret *ir.Type, args ...ir.Expr) *ir.Call {
	return ir.CallOf(ir.Pkg(pkgPath, pkgName, sel, nil), ret, args...)
}

// helperCall builds a call to a runtime support function, registering it.
func (l *Lowerer) helperCall(name string, ret *ir.Type, args ...ir.Expr) *ir.Call {
	return ir.CallOf(ir.NewIdent(l.use(name), nil), ret, args...)
}

// unwrapNil reads a value that may be absent as the value behind it.
//
// A slot with room for undef holds a pointer, and every conversion below wants
// the value rather than the address. Perl read undef as 0, as the empty string
// and as false depending on the operator, which is exactly what the zero value
// behind a nil pointer gives, so the unwrapping is faithful and not an
// approximation.
func (l *Lowerer) unwrapNil(x ir.Expr) ir.Expr {
	if x == nil || !isNullable(x.Type()) {
		return x
	}
	return l.helperCall(hDeref, x.Type().Elem, x)
}

// toStr renders any expression as a Go string.
func (l *Lowerer) toStr(x ir.Expr, n ast.Node) ir.Expr {
	if x == nil {
		return ir.Str(`""`)
	}
	x = l.unwrapNil(x)
	t := x.Type()
	switch {
	case t == nil:
		return l.helperCall(hToText, ir.TString, x)
	case t.Kind == ir.String:
		return x
	case t.Kind == ir.Int:
		out := call("strconv", "strconv", "Itoa", ir.TString, x)
		l.note(out, "Go will not turn a number into text on its own. strconv.Itoa "+
			"is the explicit conversion Perl was doing silently every time a number "+
			"met the . operator or a double-quoted string.",
			"explicit-conversions-no-coercion")
		return out
	case t.Kind == ir.Float:
		out := l.helperCall(hFormatNum, ir.TString, x)
		l.note(out, "Perl prints a float with fifteen significant digits and trims "+
			"the trailing zeros, so 0.1 + 0.2 shows as 0.3. Go's default float "+
			"formatting shows the shortest text that reads back to the same value, "+
			"which is 0.30000000000000004. "+hFormatNum+" reproduces Perl's rule.",
			"explicit-conversions-no-coercion")
		return out
	case t.Kind == ir.Bool:
		out := l.helperCall(hToText, ir.TString, x)
		l.note(out, "A Perl comparison yields 1 for true and the empty string for "+
			"false, so printing one prints 1 or nothing. Go comparisons yield a bool, "+
			"which prints as true or false.")
		return out
	case t.Kind == ir.Slice:
		sep := l.seps.listSep()
		out := l.helperCall(hJoinList, ir.TString, x, ir.Str(quote(sep)))
		if sep == " " {
			l.note(out, "Interpolating an array into a string joins its elements with a "+
				"space. Go has no interpolation, so the join is written out.")
		} else {
			l.note(out, "Interpolating an array into a string joins its elements with "+
				"whatever $\" holds. Go has neither interpolation nor a global anyone can "+
				"reach in and change, so the separator appears in the call.")
		}
		return out
	}
	return l.helperCall(hToText, ir.TString, x)
}

// toFloat renders any expression as a float64.
func (l *Lowerer) toFloat(x ir.Expr, n ast.Node) ir.Expr {
	if x == nil {
		return ir.FloatLit("0")
	}
	x = l.unwrapNil(x)
	t := x.Type()
	switch {
	case t == nil:
		return l.helperCall(hToNum, ir.TFloat, x)
	case t.Kind == ir.Float:
		return x
	case t.Kind == ir.Int:
		if lit, ok := x.(*ir.Lit); ok && lit.Kind == ir.LitInt {
			return ir.FloatLit(lit.Value)
		}
		return conversion(ir.TFloat, x)
	case t.Kind == ir.String:
		out := l.helperCall(hParseNum, ir.TFloat, x)
		l.note(out, "Perl reads the longest numeric prefix of a string and calls the "+
			"rest zero, so \"12abc\" + 0 is 12 and \"abc\" + 0 is 0. Go's "+
			"strconv.ParseFloat returns an error instead. "+hParseNum+" keeps Perl's rule; "+
			"if the input is really always a number, strconv.ParseFloat with a checked "+
			"error is the better Go.",
			"strconv-parsing", "explicit-conversions-no-coercion")
		return out
	}
	return l.helperCall(hToNum, ir.TFloat, x)
}

// toInt renders any expression as an int.
func (l *Lowerer) toInt(x ir.Expr, n ast.Node) ir.Expr {
	if x == nil {
		return ir.IntLit("0")
	}
	x = l.unwrapNil(x)
	t := x.Type()
	switch {
	case t != nil && t.Kind == ir.Bool:
		// Perl's true is the number 1 and its false is the empty string,
		// which prints as nothing and counts as zero. Go has no arithmetic on
		// a bool at all, so the choice is written out.
		if lit, ok := x.(*ir.Lit); ok && lit.Kind == ir.LitBool {
			if lit.Value == "true" {
				return ir.IntLit("1")
			}
			return ir.IntLit("0")
		}
		name := l.tmp("n")
		decl := &ir.DeclStmt{Names: []string{name}, Type: ir.TInt}
		set := &ir.If{Cond: x, Then: &ir.Block{Stmts: []ir.Stmt{
			assign("=", []ir.Expr{ir.NewIdent(name, ir.TInt)}, []ir.Expr{ir.IntLit("1")}),
		}}}
		l.note(decl, "Perl's true is the number 1 and its false is the empty string, so "+
			"a flag prints as 1 or as nothing and counts as 1 or 0. Go has no arithmetic "+
			"on a bool and no conversion from one, so the choice is written out.",
			"explicit-conversions-no-coercion")
		l.emit(decl)
		l.emit(set)
		return ir.NewIdent(name, ir.TInt)
	case t == nil:
		return conversion(ir.TInt, l.helperCall(hToNum, ir.TFloat, x))
	case t.Kind == ir.Int:
		return x
	case t.Kind == ir.Float:
		// Go refuses to convert an untyped float constant to int, because the
		// truncation would be silent. Doing it here is the same answer.
		if text, ok := floatConstant(x); ok {
			if f, err := strconv.ParseFloat(text, 64); err == nil {
				return ir.IntLit(strconv.FormatInt(int64(f), 10))
			}
		}
		out := conversion(ir.TInt, x)
		l.note(out, "Converting a float to an int in Go truncates towards zero, which "+
			"is what Perl's int() does too.")
		return out
	case t.Kind == ir.String:
		return conversion(ir.TInt, l.helperCall(hParseNum, ir.TFloat, x))
	}
	return conversion(ir.TInt, l.helperCall(hToNum, ir.TFloat, x))
}

// floatConstant reports whether an expression is a floating-point constant,
// possibly negated, and returns its text.
func floatConstant(x ir.Expr) (string, bool) {
	switch n := x.(type) {
	case *ir.Lit:
		if n.Kind == ir.LitFloat {
			return n.Value, true
		}
	case *ir.Unary:
		if n.Op == "-" {
			if text, ok := floatConstant(n.X); ok {
				return "-" + text, true
			}
		}
	}
	return "", false
}

// toNum renders an expression as whatever numeric type suits it: int when the
// expression is already integral, float64 otherwise. Keeping integers as int
// matters, because `int` is what a Go developer writes for a counter.
func (l *Lowerer) toNum(x ir.Expr, n ast.Node) ir.Expr {
	if x != nil && x.Type() != nil && x.Type().Kind == ir.Int {
		return x
	}
	return l.toFloat(x, n)
}

// toBool renders an expression as a Go bool using Perl's truth rule: zero, the
// empty string, and the one-character string "0" are false, everything else is
// true. Getting this wrong is silent and common, so each shape gets its own
// note rather than a blanket helper call.
func (l *Lowerer) toBool(x ir.Expr, n ast.Node) ir.Expr {
	if x == nil {
		return ir.BoolLit(false)
	}
	x = l.unwrapNil(x)
	t := x.Type()
	switch {
	case t == nil:
		return l.helperCall(hIsTrue, ir.TBool, x)
	case t.Kind == ir.Bool:
		return x
	case t.Kind == ir.Int:
		if lit, ok := x.(*ir.Lit); ok && lit.Kind == ir.LitInt {
			// A written-out number is true or false here and now, and saying
			// so reads better than comparing a constant with zero.
			return ir.BoolLit(lit.Value != "0")
		}
		out := ir.Bin("!=", x, ir.IntLit("0"), ir.TBool)
		l.note(out, "Go has no truthiness: only a bool can be a condition. A Perl "+
			"number is false when it is zero, so the test is written out.",
			"static-types-and-zero-values")
		return out
	case t.Kind == ir.Float:
		return ir.Bin("!=", x, ir.FloatLit("0"), ir.TBool)
	case t.Kind == ir.String:
		out := l.helperCall(hTruthy, ir.TBool, x)
		l.note(out, "Perl calls a string false when it is empty or is exactly \"0\", "+
			"so a line of text reading \"0\" is false while \"0.0\" and \" \" are true. "+
			"Go has no such rule, so the test is spelled out.",
			"static-types-and-zero-values")
		return out
	case t.Kind == ir.Slice, t.Kind == ir.Map:
		out := ir.Bin(">", ir.CallOf(ir.NewIdent("len", nil), ir.TInt, x), ir.IntLit("0"), ir.TBool)
		l.note(out, "An array or hash in a Perl condition is true when it has "+
			"elements. Go asks for the length explicitly.",
			"nil-slices-vs-nil-maps")
		return out
	case t.Kind == ir.Pointer, t.Kind == ir.Error:
		return ir.Bin("!=", x, ir.Nil(t), ir.TBool)
	}
	return l.helperCall(hIsTrue, ir.TBool, x)
}

// assignable converts x so it can be stored in a variable of type want.
func (l *Lowerer) assignable(x ir.Expr, want *ir.Type, n ast.Node) ir.Expr {
	if x == nil || want == nil {
		return x
	}
	have := x.Type()
	if have != nil && have.Equal(want) {
		return x
	}
	// A slot that has room for undef holds *T, and the two directions across
	// that boundary are the whole point of the pointer: nothing going in stays
	// nil, and a value coming out has to be read through the pointer.
	if isNullable(want) {
		if lit, ok := x.(*ir.Lit); ok && lit.Kind == ir.LitNil {
			return ir.Nil(want)
		}
		if isNullable(have) {
			return x
		}
		return l.helperCall(hPtr, want, l.assignable(x, want.Elem, n))
	}
	if isNullable(have) && want.Kind != ir.Any {
		x = l.helperCall(hDeref, have.Elem, x)
		have = have.Elem
		if have.Equal(want) {
			return x
		}
	}
	switch want.Kind {
	case ir.String:
		return l.toStr(x, n)
	case ir.Int:
		return l.toInt(x, n)
	case ir.Float:
		return l.toFloat(x, n)
	case ir.Bool:
		return l.toBool(x, n)
	case ir.Any:
		// A collection literal stored where anything can go is read back by
		// asserting it to the collection of anything, because that is the
		// only shape the reader can name. Writing the literal with its own
		// element type would make that assertion fail at run time, and a
		// literal has no other owner to be aliased from, so it is simply
		// written as the wider type.
		if lit, ok := x.(*ir.CompositeLit); ok && lit.LitType != nil &&
			(lit.LitType.Kind == ir.Slice || lit.LitType.Kind == ir.Map) &&
			lit.LitType.Elem != nil && lit.LitType.Elem.Kind != ir.Any {
			wider := ir.SliceOf(ir.TAny)
			if lit.LitType.Kind == ir.Map {
				wider = ir.MapOf(ir.TAny)
			}
			return l.assignable(x, wider, n)
		}
		return x
	case ir.Func, ir.Named, ir.Pointer:
		return l.assertTo(x, want)
	case ir.Slice, ir.Map:
		if have != nil && have.Kind == ir.Any {
			return l.assertTo(x, want)
		}
		// A literal is written where it stands, so it can simply be written
		// with the type it is going into. An empty one has no elements to
		// infer from at all, which is the `@a = ()` case; a non-empty one
		// worked out its own element type, and the surrounding declaration is
		// the better answer whenever the two differ.
		// Retyping a non-empty literal is a second-pass move only: on the
		// first pass the literal's own type is the evidence the declaration
		// is being inferred from, and overwriting it would erase exactly the
		// observation being collected.
		if lit, ok := x.(*ir.CompositeLit); ok && (len(lit.Elems) == 0 || l.pass == 2) &&
			(have == nil || have.Kind == want.Kind) {
			lit.LitType = want
			lit.T = want
			for i, el := range lit.Elems {
				lit.Elems[i] = l.assignable(el, want.Elem, n)
			}
			return lit
		}
		// A collection of one element type is not a collection of any: Go has
		// no conversion between them, so the values are copied across.
		if have != nil && have.Kind == want.Kind && want.Elem != nil && want.Elem.Kind == ir.Any &&
			have.Elem != nil && have.Elem.Kind != ir.Any && l.pass == 2 {
			helper := hAnyList
			if want.Kind == ir.Map {
				helper = hAnyMap
			}
			out := l.helperCall(helper, want, x)
			l.approximate(n, "P2G3021", "a typed collection stored where anything can go",
				"the values are copied into a new collection",
				"The destination holds values of no fixed type, and the source holds "+
					"one particular type. Go has no conversion between the two, because "+
					"they are laid out differently in memory, so the elements are copied "+
					"one at a time.",
				"Give the destination the same element type as the source. The copy then "+
					"disappears, and so does the difference between changing one and "+
					"changing the other.",
				"slice-aliasing-and-copy", "static-types-and-zero-values")
			return out
		}
		// The same gap in the other direction: the source holds values of no
		// fixed type and the destination wants one. Each value has to be
		// asserted on the way across, which is what a type switch would do one
		// value at a time.
		if have != nil && have.Kind == want.Kind && want.Elem != nil && want.Elem.Kind != ir.Any &&
			have.Elem != nil && have.Elem.Kind == ir.Any && l.pass == 2 {
			helper := hTypedList
			if want.Kind == ir.Map {
				helper = hTypedMap
			}
			out := l.helperCall(helper, want, x)
			out.TypeArgs = typeArgsFor(want)
			l.approximate(n, "P2G3022", "a collection of anything stored where one type is wanted",
				"the values are asserted one at a time into a new collection",
				"The source holds values of no fixed type and the destination wants one "+
					"particular type. Go has no conversion between the two, so the values "+
					"are copied across with an assertion on each; a value that is not of "+
					"that type lands as the type's zero value.",
				"Give the source the same element type as the destination. The copy then "+
					"disappears, and so does the chance of a value quietly turning into a "+
					"zero.",
				"collections-hold-one-type", "type-assertions-and-switches")
			return out
		}
		// Two slices whose element types are both known but different. Go has
		// no conversion between them either: []int and []float64 are laid out
		// differently, so the elements are read across one at a time.
		if have != nil && have.Kind == ir.Slice && want.Kind == ir.Slice && l.pass == 2 {
			helper := ""
			switch want.Elem.Kind {
			case ir.Int:
				helper = hIntList
			case ir.Float:
				helper = hFloatList
			case ir.String:
				helper = hStrList
			}
			if helper != "" {
				out := l.helperCall(helper, want, x)
				l.approximate(n, "P2G3021", "a list read as a different element type",
					"the elements are read across into a new list",
					"A Go slice holds one element type and there is no conversion "+
						"between two of them, because they are laid out differently in "+
						"memory. The elements are read across one at a time instead.",
					"Declare both with the same element type; the copy then disappears, "+
						"and so does the difference between changing one and changing the "+
						"other.",
					"collections-hold-one-type", "explicit-conversions-no-coercion")
				return out
			}
		}
		return x
	}
	return x
}

// typeArgsFor spells out the type parameters a typed-collection helper cannot
// infer from its argument, since the element type appears only in its result.
func typeArgsFor(want *ir.Type) []*ir.Type {
	if want == nil {
		return nil
	}
	if want.Kind == ir.Map {
		key := want.Key
		if key == nil {
			key = ir.TString
		}
		return []*ir.Type{key, want.Elem}
	}
	return []*ir.Type{want.Elem}
}

// assertTo states what a value held in an any really is, so that Go will let
// it be used as that type. A value that already has the type is left alone.
func (l *Lowerer) assertTo(x ir.Expr, want *ir.Type) ir.Expr {
	if typeOrAny(x).Kind != ir.Any || want == nil {
		return x
	}
	// An untyped nil is not an interface value and has nothing to assert; the
	// type's own zero value is what was meant.
	if lit, ok := x.(*ir.Lit); ok && lit.Kind == ir.LitNil {
		return zeroOf(want)
	}
	// A bare `x.(T)` stops the program the moment the guess is wrong, and a
	// value of no fixed type is exactly where the guess can be wrong: this is
	// the type inference saying it did not work the value out. The helper is
	// the two-result form, which asks rather than insists, so a wrong guess
	// leaves an empty collection or a nil record and the lines below still
	// run. Where the type is known, neither shape is emitted at all.
	out := l.tolerantAs(x, want)
	l.note(out, "This value is held in an any, so Go needs to be told what it is "+
		"before it can be used as one. A plain assertion, x.(T), panics when the "+
		"value turns out to be something else; this is the two-result form, which "+
		"asks instead of insisting and gives the zero value when the answer is no.",
		"type-assertions-and-switches", "comma-ok-idiom")
	return out
}

// zeroOf builds the zero value of a type as an expression.
func zeroOf(t *ir.Type) ir.Expr {
	if t == nil {
		return ir.Nil(ir.TAny)
	}
	switch t.Kind {
	case ir.String:
		return ir.Str(`""`)
	case ir.Int:
		return ir.IntLit("0")
	case ir.Float:
		return ir.FloatLit("0")
	case ir.Bool:
		return ir.BoolLit(false)
	case ir.Map:
		return &ir.CompositeLit{LitType: t}
	}
	return ir.Nil(t)
}
