package lower

import (
	"strconv"
	"strings"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// expr lowers an expression in its natural context: an array yields a Go
// slice, a hash yields a map, a scalar yields a single value.
func (l *Lowerer) expr(e ast.Expr) ir.Expr {
	x := l.exprInner(e)
	if x != nil {
		l.setProv(x, e)
	}
	return x
}

func (l *Lowerer) exprInner(e ast.Expr) ir.Expr {
	switch n := e.(type) {
	case nil:
		return nil
	case *ast.NumberLit:
		return numberLit(n.Text)
	case *ast.StrLit:
		return ir.Str(strconv.Quote(n.Value))
	case *ast.InterpLit:
		return l.interp(n)
	case *ast.QwLit:
		elems := make([]ir.Expr, len(n.Words))
		for i, w := range n.Words {
			elems[i] = ir.Str(strconv.Quote(w))
		}
		out := composite(ir.SliceOf(ir.TString), nil, elems)
		l.note(out, "qw() is a list of words with the quoting left out. Go spells "+
			"the same thing as a slice literal, quotes and all.")
		return out
	case *ast.Var:
		return l.varExpr(n)
	case *ast.My:
		return l.myExpr(n)
	case *ast.Assign:
		return l.assignExpr(n)
	case *ast.BinOp:
		return l.binop(n)
	case *ast.UnOp:
		return l.unop(n)
	case *ast.Ternary:
		return l.ternary(n)
	case *ast.List:
		return l.listLit(n)
	case *ast.Call:
		return l.callExpr(n)
	case *ast.FuncCallRef:
		return l.callRef(n)
	case *ast.Index:
		return l.indexExpr(n)
	case *ast.HashIndex:
		return l.hashExpr(n)
	case *ast.Slice:
		return l.sliceExpr(n)
	case *ast.Deref:
		return l.derefExpr(n)
	case *ast.RefGen:
		return l.refGen(n)
	case *ast.AnonArray:
		return l.anonArray(n)
	case *ast.AnonHash:
		return l.anonHash(n)
	case *ast.AnonSub:
		return l.anonSub(n)
	case *ast.Match:
		return l.matchExpr(n, false)
	case *ast.Subst:
		return l.substExpr(n)
	case *ast.Trans:
		return l.transExpr(n)
	case *ast.QrExpr:
		return l.qrExpr(n)
	case *ast.Readline:
		return l.readlineExpr(n)
	case *ast.FileTest:
		return l.fileTest(n)
	case *ast.FileHandle:
		return l.fileHandleExpr(n)
	case *ast.MethodCall:
		return l.todoExpr(n, "P2G7001", "method call",
			"object method calls are not implemented",
			"This calls a method on an object. Perl objects are blessed references "+
				"whose class is decided at run time, and method resolution walks @ISA. "+
				"Go has methods, but they are declared on a named type at compile time "+
				"and there is no inheritance.",
			"Declare a struct type for the class, turn each sub in the package into a "+
				"method with a receiver, and replace inheritance with embedding or with "+
				"an interface the concrete types satisfy.",
			"methods-and-receivers", "structs-and-embedding", "implicit-interfaces")
	case *ast.BacktickCmd:
		return l.todoExpr(n, "P2G6501", "backticks",
			"running an external command is not implemented",
			"Backticks run a shell command and capture its output. Go can do this with "+
				"os/exec, but the shape is different enough that a mechanical translation "+
				"would hide the error handling Go insists on.",
			"Use exec.Command(name, args...).Output(), check the returned error, and "+
				"note that Go does not involve a shell unless you ask for one explicitly.",
			"os-exec", "errors-are-values")
	case *ast.GlobExpr:
		return l.todoExpr(n, "P2G6020", "glob",
			"filename globbing is not implemented",
			"The glob operator expands a shell-style pattern into filenames.",
			"Use path/filepath.Glob, which returns the matches and an error.",
			"errors-are-values")
	case *ast.BadExpr:
		return l.todoExpr(n, "P2G1520", "unparsed expression",
			"this expression was not understood",
			"The parser could not make sense of this expression: "+n.Reason+".",
			"Translate it by hand. The original is quoted above.")
	}
	return l.todoExpr(e, "P2G3599", "expression",
		"this expression is not implemented",
		"The converter has no rule for this construct yet.",
		"Translate it by hand.")
}

// scalar lowers an expression in scalar context, where Perl asks a list how
// long it is rather than what is in it.
func (l *Lowerer) scalar(e ast.Expr) ir.Expr {
	switch n := e.(type) {
	case *ast.Var:
		if n.Sigil == '@' || n.Sigil == '%' {
			x := l.expr(e)
			out := ir.CallOf(ir.NewIdent("len", nil), ir.TInt, x)
			l.note(out, "An array in scalar context is its element count in Perl. Go "+
				"asks for that with the built-in len.",
				"context-is-gone")
			return out
		}
	case *ast.Call:
		if x, ok := l.multiResultCall(n, false); ok {
			return x
		}
		if isListBuiltin(n.Name) {
			x := l.expr(e)
			if typeOrAny(x).Kind == ir.Slice {
				out := lenOf(x)
				l.note(out, "In scalar context a Perl list operator reports how many "+
					"values it produced. Go asks for that with len.")
				return out
			}
			return x
		}
	case *ast.List:
		if len(n.Elems) == 0 {
			return ir.Nil(ir.TAny)
		}
		// A comma expression in scalar context yields its last element.
		for _, el := range n.Elems[:len(n.Elems)-1] {
			l.evalForEffect(el)
		}
		return l.scalar(n.Elems[len(n.Elems)-1])
	}
	x := l.expr(e)
	if x != nil && x.Type() != nil && x.Type().Kind == ir.Slice {
		if _, isVar := e.(*ast.Var); !isVar {
			return x
		}
	}
	return x
}

// list lowers an expression in list context and always yields a slice.
func (l *Lowerer) list(e ast.Expr) ir.Expr {
	parts, t := l.listParts([]ast.Expr{e})
	if len(parts) == 1 {
		p := parts[0]
		if p.Type() != nil && p.Type().Kind == ir.Slice {
			return p
		}
		// A value whose type inference did not resolve may be holding a list
		// at run time, and Go cannot tell. Treating it as a single element is
		// the only thing that compiles, and it is not necessarily right.
		if typeOrAny(p).Kind == ir.Any {
			l.approximate(e, "P2G3010", "a dynamic value used as a list",
				"a value of unknown type is being treated as one element",
				"This value's type did not resolve, so it is declared as `any`. Perl "+
					"would flatten it into the surrounding list if it held one; Go cannot "+
					"know whether it does, so it is used as a single element.",
				"Give the variable a concrete type at its declaration. Where the value "+
					"really is a list, declaring it as a slice makes this line correct and "+
					"removes the need for the conversions around it.",
				"type-assertions-and-switches", "static-types-and-zero-values")
		}
	}
	return l.listValue(parts, t)
}

// listValue builds the Go slice a lowered Perl list produces.
//
// Perl lists are flat: `(1, @rest, 2)` is one list of however many elements
// @rest holds, not a list of three things. Go has no such rule, so an array in
// the middle of a list has to be spliced in explicitly, which slices.Concat
// does in one call.
func (l *Lowerer) listValue(parts []ir.Expr, elem *ir.Type) ir.Expr {
	sliceT := ir.SliceOf(elem)
	splices := func(p ir.Expr) bool {
		pt := typeOrAny(p)
		return pt.Kind == ir.Slice && pt.Elem.Equal(elem)
	}

	any := false
	for _, p := range parts {
		if splices(p) {
			any = true
			break
		}
	}
	fit := func(ps []ir.Expr) []ir.Expr {
		for i, p := range ps {
			ps[i] = l.assignable(p, elem, nil)
		}
		return ps
	}
	if !any {
		return composite(sliceT, nil, fit(parts))
	}

	var groups []ir.Expr
	var run []ir.Expr
	flush := func() {
		if len(run) > 0 {
			groups = append(groups, composite(sliceT, nil, fit(run)))
			run = nil
		}
	}
	for _, p := range parts {
		if splices(p) {
			flush()
			groups = append(groups, p)
			continue
		}
		run = append(run, p)
	}
	flush()
	if len(groups) == 1 {
		return groups[0]
	}
	out := call("slices", "slices", "Concat", sliceT, groups...)
	l.note(out, "A Perl list is flat, so an array written inside one contributes its "+
		"elements rather than itself. Go keeps them apart, and slices.Concat is how "+
		"several slices become one.",
		"slices-not-arrays")
	return out
}

// listParts flattens a Perl list into Go expressions, reporting the joined
// element type. Perl lists are flat: a list inside a list is spliced in, and
// an array contributes its elements rather than itself.
func (l *Lowerer) listParts(es []ast.Expr) ([]ir.Expr, *ir.Type) {
	var out []ir.Expr
	var seen []*ir.Type
	var add func(e ast.Expr)
	add = func(e ast.Expr) {
		switch n := e.(type) {
		case nil:
			return
		case *ast.List:
			for _, el := range n.Elems {
				add(el)
			}
			return
		case *ast.BinOp:
			if n.Op == "," {
				add(n.L)
				add(n.R)
				return
			}
		case *ast.Match:
			// A global match in list context yields every match, not a truth
			// value, which is a different Go call entirely.
			if x, ok := l.globalMatch(n); ok {
				out = append(out, x)
				seen = append(seen, elemOf(typeOrAny(x)))
				return
			}
			// A plain match in list context yields its capture groups, which
			// is what `my ($a, $b) = $s =~ /(x)(y)/` is reading.
			if xs, ok := l.captureList(n); ok {
				out = append(out, xs...)
				for _, x := range xs {
					seen = append(seen, typeOrAny(x))
				}
				return
			}
		case *ast.Call:
			if x, ok := l.multiResultCall(n, true); ok {
				out = append(out, x)
				seen = append(seen, elemOf(typeOrAny(x)))
				return
			}
		}
		x := l.expr(e)
		if x == nil {
			return
		}
		out = append(out, x)
		if xt := x.Type(); xt != nil && xt.Kind == ir.Slice && flattensInList(e) {
			seen = append(seen, xt.Elem)
			return
		}
		seen = append(seen, x.Type())
	}
	for _, e := range es {
		add(e)
	}
	// The parts decide the element type together, and two that no single Go
	// type covers settle it at any for good.
	t := joinAll(seen)
	if t == nil {
		t = ir.TAny
	}
	return out, t
}

// flattensInList reports whether an expression contributes its elements to a
// surrounding list rather than itself.
//
// This is the difference between `(1, @rest)`, which is one flat list, and
// `(1, [@rest])`, which is a list of two things, the second being a reference.
// Everything that builds a reference keeps its contents to itself; everything
// else in list context spills.
func flattensInList(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.AnonArray, *ast.AnonHash, *ast.AnonSub, *ast.RefGen:
		return false
	case *ast.Index, *ast.HashIndex:
		// An element of a container is one value, even when that value is a
		// reference to a list.
		return false
	case *ast.Var:
		return n.Sigil != '$'
	case *ast.Deref:
		return n.Sigil != '$'
	}
	return true
}

// flatten returns the elements of a Perl list expression without lowering
// them, so callers can inspect the shape first.
func flatten(e ast.Expr) []ast.Expr {
	switch n := e.(type) {
	case nil:
		return nil
	case *ast.List:
		var out []ast.Expr
		for _, el := range n.Elems {
			out = append(out, flatten(el)...)
		}
		return out
	case *ast.BinOp:
		if n.Op == "," {
			return append(flatten(n.L), flatten(n.R)...)
		}
	}
	return []ast.Expr{e}
}

// ---------------------------------------------------------------------------
// Literals

// numberLit turns Perl's numeric spelling into a Go literal, keeping integers
// integral. Perl has one numeric type; Go has int and float64, and choosing
// int wherever it is honest is what makes the output read like Go.
func numberLit(text string) ir.Expr {
	clean := strings.ReplaceAll(text, "_", "")
	if strings.ContainsAny(clean, ".eE") && !strings.HasPrefix(strings.ToLower(clean), "0x") {
		if _, err := strconv.ParseFloat(clean, 64); err == nil {
			return ir.FloatLit(clean)
		}
	}
	if _, err := strconv.ParseInt(clean, 0, 64); err == nil {
		// Perl's leading-zero octal spelling is 0NNN; Go 1.13 and later spell
		// it 0oNNN, and the bare form is a compile error.
		if len(clean) > 1 && clean[0] == '0' && clean[1] >= '0' && clean[1] <= '9' {
			return ir.IntLit("0o" + clean[1:])
		}
		return ir.IntLit(clean)
	}
	if f, err := strconv.ParseFloat(clean, 64); err == nil {
		return ir.FloatLit(strconv.FormatFloat(f, 'g', -1, 64))
	}
	return ir.IntLit("0")
}

// listLit lowers a parenthesised list.
func (l *Lowerer) listLit(n *ast.List) ir.Expr {
	parts, t := l.listParts(n.Elems)
	// A list holding one thing that is already a list is that list: Perl
	// flattens, so (@a) and @a are the same. Wrapping it in a slice literal
	// would build a slice of slices.
	if len(parts) == 1 && typeOrAny(parts[0]).Kind == ir.Slice {
		return parts[0]
	}
	return l.listValue(parts, t)
}

// anonArray lowers [ ... ]. A Perl array reference is a pointer to an array;
// a Go slice is already a reference-like value, so the two line up without a
// pointer.
func (l *Lowerer) anonArray(n *ast.AnonArray) ir.Expr {
	parts, t := l.listParts(n.Elems)
	out := l.listValue(parts, t)
	l.note(out, "Perl's [ ... ] builds an array reference because arrays flatten "+
		"when nested. A Go slice already behaves like a reference: copying the "+
		"slice value copies a small header that points at the same elements, so "+
		"no extra indirection is needed.",
		"pointers-vs-references", "slice-aliasing-and-copy")
	return out
}

// anonHash lowers { k => v, ... }.
func (l *Lowerer) anonHash(n *ast.AnonHash) ir.Expr {
	keys, vals, t := l.pairs(n.Elems)
	out := composite(ir.MapOf(t), keys, vals)
	l.note(out, "Perl's { ... } builds a hash reference. A Go map is already a "+
		"reference type, so the map value itself is what gets passed around.",
		"pointers-vs-references", "nil-slices-vs-nil-maps")
	return out
}

// pairs splits a flat key, value, key, value list into two slices and reports
// the joined value type.
func (l *Lowerer) pairs(elems []ast.Expr) (keys, vals []ir.Expr, t *ir.Type) {
	flat := make([]ast.Expr, 0, len(elems))
	for _, e := range elems {
		flat = append(flat, flatten(e)...)
	}
	var seen []*ir.Type
	for i := 0; i+1 < len(flat); i += 2 {
		k := l.expr(flat[i])
		v := l.expr(flat[i+1])
		if k == nil || v == nil {
			continue
		}
		keys = append(keys, l.toStr(k, flat[i]))
		vals = append(vals, v)
		seen = append(seen, v.Type())
	}
	// The values decide the map's value type together, and two that no single
	// Go type covers settle it at any for good.
	t = joinAll(seen)
	if t == nil {
		t = ir.TAny
	}
	for i, v := range vals {
		vals[i] = l.assignable(v, t, nil)
	}
	return keys, vals, t
}

// ---------------------------------------------------------------------------
// String interpolation

// interp lowers a double-quoted string.
//
// Two renderings are possible and the choice matters for readability: when
// every interpolated piece is already text, Go's + reads best; when numbers
// are involved, fmt.Sprintf with explicit verbs is what a Go developer writes.
func (l *Lowerer) interp(n *ast.InterpLit) ir.Expr {
	var pieces []interpPiece
	allText := true
	interps := 0
	for _, p := range n.Parts {
		if s, ok := p.(*ast.StrLit); ok {
			pieces = append(pieces, interpPiece{text: s.Value})
			continue
		}
		x := l.expr(p)
		if x == nil {
			continue
		}
		interps++
		if x.Type() == nil || x.Type().Kind != ir.String {
			allText = false
		}
		pieces = append(pieces, interpPiece{expr: x, node: p})
	}

	if interps == 0 {
		var sb strings.Builder
		for _, p := range pieces {
			sb.WriteString(p.text)
		}
		return ir.Str(strconv.Quote(sb.String()))
	}

	if allText {
		var out ir.Expr
		add := func(x ir.Expr) {
			if out == nil {
				out = x
				return
			}
			out = ir.Bin("+", out, x, ir.TString)
		}
		for _, p := range pieces {
			if p.expr != nil {
				add(p.expr)
				continue
			}
			if p.text == "" {
				continue
			}
			add(ir.Str(strconv.Quote(p.text)))
		}
		l.note(out, "Go has no string interpolation. Joining text with + is the "+
			"direct equivalent when every piece is already a string; fmt.Sprintf is "+
			"the choice when some are not.")
		return out
	}

	format, args := l.sprintfParts(pieces)
	out := call("fmt", "fmt", "Sprintf", ir.TString, append([]ir.Expr{ir.Str(strconv.Quote(format))}, args...)...)
	l.note(out, "Go has no string interpolation. fmt.Sprintf takes a format string "+
		"with one verb per value: %s for text, %d for a whole number. The verbs are "+
		"checked by go vet, which catches the mismatches Perl would have silently "+
		"stringified.",
		"vet-and-staticcheck")
	return out
}

// interpPiece is one span of an interpolating string: either literal text or
// an embedded expression.
type interpPiece struct {
	text string
	expr ir.Expr
	node ast.Expr
}

// sprintfParts builds a Go format string and its arguments from interpolation
// pieces, choosing the verb from each value's type.
func (l *Lowerer) sprintfParts(pieces []interpPiece) (string, []ir.Expr) {
	var format strings.Builder
	var args []ir.Expr
	for _, p := range pieces {
		if p.expr == nil {
			format.WriteString(strings.ReplaceAll(p.text, "%", "%%"))
			continue
		}
		t := p.expr.Type()
		switch {
		case t == nil || t.Kind == ir.Any:
			format.WriteString("%s")
			args = append(args, l.toStr(p.expr, p.node))
		case t.Kind == ir.String:
			format.WriteString("%s")
			args = append(args, p.expr)
		case t.Kind == ir.Int:
			format.WriteString("%d")
			args = append(args, p.expr)
		case t.Kind == ir.Float, t.Kind == ir.Bool, t.Kind == ir.Slice, t.Kind == ir.Map:
			format.WriteString("%s")
			args = append(args, l.toStr(p.expr, p.node))
		default:
			format.WriteString("%v")
			args = append(args, p.expr)
		}
	}
	return format.String(), args
}

// ---------------------------------------------------------------------------
// Fallback

// todoExpr records a refusal and produces an expression that is honest about
// it: the generated program panics with the original Perl rather than doing
// something plausible and wrong.
func (l *Lowerer) todoExpr(n ast.Node, code, construct, short, message, advice string, concepts ...string) ir.Expr {
	todo := l.refuse(n, code, construct, short, message, advice, concepts...)
	// Go's panic is a statement and yields nothing, so in a position that
	// wants a value it has to be wrapped. The program stops here rather than
	// carrying on with something plausible and wrong.
	x := ir.Raw("func() any { panic("+strconv.Quote(short)+") }()", ir.TAny)
	m := ir.MetaOf(x)
	m.Todo = &todo
	return x
}
