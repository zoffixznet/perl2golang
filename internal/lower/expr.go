package lower

import (
	"strconv"
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
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
		l.noteNumericEdge(n)
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
		return l.methodCall(n)
	case *ast.BacktickCmd:
		return l.backtickCmd(n, false)
	case *ast.GlobExpr:
		return l.todoExpr(n, "P2G6022", "glob",
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
		// reverse read for one value reverses characters rather than
		// elements, which is the one list operator that does not answer
		// with a count.
		if n.Name == "reverse" {
			return l.reverseText(n)
		}
		// A time split read for one value is the readable stamp, not the
		// length of the list it would have given.
		if n.Name == "gmtime" || n.Name == "localtime" {
			return l.timeStamp(n, n.Name == "localtime")
		}
		// splice in scalar context is the last element it removed, not how
		// many, which is the one list operator that does not answer with a
		// count.
		if n.Name == "splice" {
			removed := l.expr(e)
			rt := typeOrAny(removed)
			if rt.Kind != ir.Slice {
				return removed
			}
			// The call has to run once, so it is named before the last element
			// is read out of it.
			name := l.tmp("removed")
			decl := assign(":=", []ir.Expr{ir.NewIdent(name, rt)}, []ir.Expr{removed})
			l.setProv(decl, n)
			l.emit(decl)
			removed = ir.NewIdent(name, rt)
			out := l.helperCall(hAt, elemOf(rt), removed,
				ir.Bin("-", lenOf(removed), ir.IntLit("1"), ir.TInt))
			l.note(out, "splice in scalar context is the last element it removed, "+
				"where every other list operator would answer with a count. Go has no "+
				"context at all, so the choice is made here and written out.",
				"context-is-gone")
			return out
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
	case *ast.Slice:
		// A slice read where one value is wanted yields its last element,
		// which for the common one-index form is simply that element.
		x := l.expr(e)
		if lit, ok := x.(*ir.CompositeLit); ok && len(lit.Elems) > 0 {
			out := lit.Elems[len(lit.Elems)-1]
			l.note(out, "A slice read for one value yields its last element rather "+
				"than a list or a count. Go has no context, so the choice is made here "+
				"and the element is named directly.",
				"context-is-gone")
			return out
		}
		if typeOrAny(x).Kind == ir.Slice {
			return l.helperCall(hAt, elemOf(typeOrAny(x)), x,
				ir.Bin("-", lenOf(x), ir.IntLit("1"), ir.TInt))
		}
		return x
	case *ast.List:
		if len(n.Elems) == 0 {
			return ir.Nil(ir.TAny)
		}
		// A comma expression in scalar context yields its last element.
		for _, el := range n.Elems[:len(n.Elems)-1] {
			l.evalForEffect(el)
		}
		return l.scalar(n.Elems[len(n.Elems)-1])
	case *ast.Readline:
		if x, ok := l.readlineScalar(n); ok {
			return x
		}
	case *ast.FuncCallRef:
		// A sub called through a reference where one value is wanted: Perl
		// hands the context into the sub, so a body ending in split answers
		// with how many pieces there were. Which sub runs is only known when
		// the program does, so when the result's type is unresolved the
		// choice between a value and a count is made at run time too.
		x := l.expr(e)
		if typeOrAny(x).Kind == ir.Any {
			out := l.helperCall(hScalarOf, ir.TAny, x)
			l.note(out, "This call reaches its sub through a value, so which sub "+
				"answers is decided while the program runs, and so is what its "+
				"result is worth as one value: a list answers with its length, "+
				"anything else is already one value. Perl decides this by handing "+
				"the caller's context into the sub; Go has no context, so the "+
				"helper asks the value instead.",
				"context-is-gone")
			return out
		}
		return x
	case *ast.Deref:
		// `scalar @{ $ref }` asks the list behind the reference how long it
		// is, exactly as a named array would be asked.
		if n.Sigil == '@' {
			x := l.expr(e)
			if typeOrAny(x).Kind == ir.Slice {
				out := lenOf(x)
				l.note(out, "A list in scalar context is its element count in Perl, and "+
					"a dereferenced array reference is no different. Go asks for that "+
					"with the built-in len.",
					"context-is-gone")
				return out
			}
			return x
		}
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
	x, _ := l.listWithShape(e)
	return x
}

// listPart describes one part of a lowered list: the type it had before it
// was fitted to the list's element type, and whether it stands for a run of
// values rather than one.
//
// The shape is what a list assignment needs to type its targets by position.
// The first target takes the first part, not the join of everything on the
// right: `($best, @pair) = ($score, @names)` gives $best a number however
// text-like the names are.
type listPart struct {
	t       *ir.Type
	spliced bool
}

// listWithShape lowers an expression in list context and also reports the
// shape of the parts the list was built from.
func (l *Lowerer) listWithShape(e ast.Expr) (ir.Expr, []listPart) {
	parts, t := l.listParts([]ast.Expr{e})
	elem := usableElem(t, argTypes(parts))
	shape := make([]listPart, len(parts))
	for i, p := range parts {
		pt := typeOrAny(p)
		shape[i] = listPart{
			t: pt,
			spliced: pt.Kind == ir.Slice &&
				(l.spliced[p] || (pt.Elem != nil && elem != nil && pt.Elem.Equal(elem))),
		}
	}
	if len(parts) == 1 {
		p := parts[0]
		if p.Type() != nil && p.Type().Kind == ir.Slice {
			// A single part that is already a slice is the whole list, and it
			// stands for however many values it holds.
			shape[0].spliced = true
			return p, shape
		}
		// A call or a conditional whose type did not resolve may be handing
		// back a list, and neither its spelling nor its Go type can say.
		// Treating it as a single element is the only thing that compiles,
		// and it is not necessarily right. A scalar variable is different: it
		// holds exactly one value however unresolved its type, so wrapping it
		// as one element is simply correct and nothing needs reporting.
		if typeOrAny(p).Kind == ir.Any && flattensInList(e) {
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
	return l.listValue(parts, t), shape
}

// containerList lowers a list that is about to become an array's contents,
// which is the one place where undef among the values changes the element type
// rather than one value.
func (l *Lowerer) containerList(rhs ast.Expr) ir.Expr {
	if !listHasUndef(rhs) {
		return l.list(rhs)
	}
	parts, t := l.listParts([]ast.Expr{rhs})
	if isScalarKind(t) {
		t = nullable(t)
	}
	return l.listValue(parts, t)
}

// listHasUndef reports whether a list written out in the source has a bare
// undef among its elements.
func listHasUndef(e ast.Expr) bool {
	switch n := e.(type) {
	case nil:
		return false
	case *ast.List:
		for _, el := range n.Elems {
			if listHasUndef(el) {
				return true
			}
		}
		return false
	case *ast.BinOp:
		if n.Op == "," || n.Op == "=>" {
			return listHasUndef(n.L) || listHasUndef(n.R)
		}
		return false
	}
	return isUndefLiteral(e)
}

// listValue builds the Go slice a lowered Perl list produces.
//
// Perl lists are flat: `(1, @rest, 2)` is one list of however many elements
// @rest holds, not a list of three things. Go has no such rule, so an array in
// the middle of a list has to be spliced in explicitly, which slices.Concat
// does in one call.
func (l *Lowerer) listValue(parts []ir.Expr, elem *ir.Type) ir.Expr {
	elem = usableElem(elem, argTypes(parts))
	sliceT := ir.SliceOf(elem)
	splices := func(p ir.Expr) bool {
		pt := typeOrAny(p)
		if pt.Kind != ir.Slice {
			return false
		}
		// A part marked when the list was taken apart is known to stand for
		// several values whatever its element type; for the rest, an element
		// type that matches the list's is the evidence, and a mismatch means
		// the slice is a reference standing as one element.
		return l.spliced[p] || pt.Elem.Equal(elem)
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
			// A spliced-in slice whose elements are narrower than the list's
			// is copied across, because Go has no conversion between a slice
			// of one type and a slice of another.
			groups = append(groups, l.assignable(p, sliceT, nil))
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
	// Several function literals in one list share a slot the same way they do
	// in a hash, and a slice has one element type too.
	saved := l.uniformFn
	l.uniformFn = l.uniformFn || closureList(es)
	defer func() { l.uniformFn = saved }()

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
				l.spliced[x] = true
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
		case *ast.BacktickCmd:
			// A command read where a list is wanted hands back one string per
			// line, which is a different call from the one that hands back
			// the whole output.
			x := l.backtickCmd(n, true)
			l.spliced[x] = true
			out = append(out, x)
			seen = append(seen, elemOf(typeOrAny(x)))
			return
		case *ast.FuncCallRef:
			// A sub reached through a reference flattens what it returns into
			// the surrounding list, exactly as a named sub would; which sub
			// answers is only known while the program runs, so what its result
			// holds is asked then too.
			x, ft, direct := l.codeRefCall(n)
			if direct && len(ft.Results) > 1 {
				// The signature says how many values come back, so each one
				// takes its own place in the list, by position.
				names := make([]ir.Expr, len(ft.Results))
				for i, rt := range ft.Results {
					names[i] = ir.NewIdent(l.tmp("r"), rt)
				}
				st := assign(":=", names, []ir.Expr{x})
				l.setProv(st, n)
				l.note(st, "A Go function that returns several values can only be "+
					"called where all of them are taken at once, so the call gets its "+
					"own statement and the results join the list by position.",
					"multiple-return-values")
				l.emit(st)
				out = append(out, names...)
				seen = append(seen, ft.Results...)
				return
			}
			if l.pass == 2 && typeOrAny(x).Kind == ir.Any {
				flat := l.helperCall(hAsList, ir.SliceOf(ir.TAny), x)
				l.note(flat, "This call reaches its sub through a value, so how many "+
					"values come back is only known while the program runs. The helper "+
					"splices in whatever list the call produced, or the single value as "+
					"a list of one.",
					"type-assertions-and-switches", "context-is-gone")
				l.spliced[flat] = true
				out = append(out, flat)
				seen = append(seen, ir.TAny)
				return
			}
			if x != nil {
				out = append(out, x)
				if xt := x.Type(); xt != nil && xt.Kind == ir.Slice {
					l.spliced[x] = true
					seen = append(seen, xt.Elem)
					return
				}
				seen = append(seen, x.Type())
			}
			return
		case *ast.Call:
			if x, ok := l.multiResultCall(n, true); ok {
				l.spliced[x] = true
				out = append(out, x)
				seen = append(seen, elemOf(typeOrAny(x)))
				return
			}
			// A sub that returns nothing contributes nothing to a list. Perl's
			// bare `return;` is the empty list here and undef in scalar
			// context, and the difference is load bearing: one element of a
			// hash constructor's list shifts every pair after it.
			if sub, known := l.findSub(n.Name); known && len(sub.Results) == 0 && n.Block == nil {
				if x := l.expr(e); x != nil {
					_ = x
				}
				return
			}
			// An eval block in a list hands the list on: its value is the
			// block's last expression evaluated in the caller's context, and
			// a die inside leaves the empty list, not one undef.
			if n.Name == "eval" && n.Block != nil {
				x := l.evalListCall(n)
				if xt := typeOrAny(x); xt.Kind == ir.Slice {
					l.spliced[x] = true
					out = append(out, x)
					seen = append(seen, xt.Elem)
					return
				}
				out = append(out, x)
				seen = append(seen, typeOrAny(x))
				return
			}
		}
		x := l.expr(e)
		if x == nil {
			return
		}
		// A keyed collection read where a list is wanted lays itself out as
		// key, value, key, value. When the values are text the whole flat
		// list is text, keys included, and keeping that fact keeps typed
		// whatever the list lands in.
		if xt := typeOrAny(x); xt.Kind == ir.Map && flattensInList(e) {
			helper, elem := hFlatPairs, ir.TAny
			if xt.Elem != nil && xt.Elem.Kind == ir.String {
				helper, elem = hFlatPairsText, ir.TString
			}
			flat := l.helperCall(helper, ir.SliceOf(elem), x)
			l.note(flat, "A hash used where a list is wanted becomes its pairs, laid "+
				"out one after another. Go has no such flattening and no ordering "+
				"either, so the pairs come out sorted by key, which is stable where "+
				"ranging over the map would not be.",
				"map-iteration-order", "context-is-gone")
			l.spliced[flat] = true
			out = append(out, flat)
			seen = append(seen, elem)
			return
		}
		// A dereferenced array whose type did not resolve still flattens: the
		// syntax proves it holds a list even though the compiled type cannot.
		// The helper asks the value for its elements while the program runs,
		// which is when the answer exists. Only the pass that emits real code
		// wraps: the discovery pass must keep seeing the plain value, or the
		// wrapper's []any becomes evidence and freezes the type it was only
		// standing in for.
		if l.pass == 2 && typeOrAny(x).Kind == ir.Any && certainlyList(e) {
			flat := l.helperCall(hAsList, ir.SliceOf(ir.TAny), x)
			l.note(flat, "This value's type did not resolve, so it is compiled as "+
				"`any`, but the way it is written proves it stands for a list. The "+
				"helper splices in whatever elements it holds when the program runs, "+
				"which is the moment the answer becomes knowable.",
				"type-assertions-and-switches", "context-is-gone")
			if l.pass == 2 {
				l.approximate(e, "P2G3010", "a dynamic value used as a list",
					"the value's elements are spliced in while the program runs",
					"This value's type did not resolve, so it is declared as `any`. "+
						"Perl flattens what it holds into the surrounding list; the "+
						"generated code does the same through a helper that asks the "+
						"value for its elements at run time instead of at compile time.",
					"Give the variable a concrete slice type at its declaration. The "+
						"helper call then disappears, along with the conversions around it.",
					"type-assertions-and-switches", "static-types-and-zero-values")
			}
			l.spliced[flat] = true
			out = append(out, flat)
			seen = append(seen, ir.TAny)
			return
		}
		out = append(out, x)
		if xt := x.Type(); xt != nil && xt.Kind == ir.Slice && flattensInList(e) {
			l.spliced[x] = true
			seen = append(seen, xt.Elem)
			return
		}
		seen = append(seen, x.Type())
	}
	for _, e := range es {
		add(e)
	}
	// The parts decide the element type together, and two that no single Go
	// type covers settle it at any for good. Two exceptions keep the
	// collection typed: several classes together have an interface in
	// common, and text beside numbers has the string form every Perl scalar
	// carries.
	t := joinAll(seen)
	if t == nil || t.Kind == ir.Any {
		if iface, ok := l.commonInterface(l.classesOf(seen)); ok {
			return out, iface
		}
	}
	if isUnresolved(t) {
		if s := stringlyResolve(seen); s != nil {
			t = s
		}
	}
	if t == nil {
		t = ir.TAny
	}
	return out, t
}

// classesOf reads the classes out of a list of observed types, reporting
// nothing when any of them is not a class at all.
func (l *Lowerer) classesOf(ts []*ir.Type) []*Class {
	var out []*Class
	for _, t := range ts {
		c := l.classOf(t)
		if c == nil {
			return nil
		}
		out = append(out, c)
	}
	return out
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

// certainlyList reports whether an expression's spelling alone proves it
// stands for a list of values, whatever type inference made of it. `@$ref`
// can only ever be a list; `f()` might return one value or several; `$x` is
// always exactly one. Only the first kind is safe to splice when the type is
// unknown, because a slice-typed value behind a scalar is a reference and a
// reference contributes itself.
func certainlyList(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.Var:
		return n.Sigil == '@'
	case *ast.Deref:
		return n.Sigil == '@'
	case *ast.Slice:
		return true
	}
	return false
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

// noteNumericEdge reports a whole-number literal at or past the edge of what
// int64 holds. Perl walks a number through signed, unsigned and double
// representations as it grows, silently and without wrapping; Go's int stays
// 64-bit and wraps, so arithmetic on a value this size behaves differently
// and the reader deserves to hear it where the number is written.
func (l *Lowerer) noteNumericEdge(n *ast.NumberLit) {
	clean := strings.ReplaceAll(n.Text, "_", "")
	if strings.ContainsAny(clean, ".eE") && !strings.HasPrefix(strings.ToLower(clean), "0x") {
		return
	}
	if v, err := strconv.ParseInt(clean, 0, 64); err == nil {
		if v >= 1<<62 || v <= -(1<<62) {
			l.approximate(n, "P2G3020", "a whole number at int64's edge",
				"arithmetic near this size wraps in Go",
				"Perl promotes a growing number through unsigned and then float "+
					"representations, so adding 1 to the largest signed value stays "+
					"exact. Go's int is 64-bit and overflows by wrapping: the same "+
					"addition lands at the most negative value, silently, and no "+
					"check anywhere reports the overflow.",
				"Where values genuinely reach this size, use math/big.Int, or "+
					"uint64 when the range is one bit short.",
				"explicit-conversions-no-coercion", "static-types-and-zero-values")
		}
		return
	}
	if _, err := strconv.ParseFloat(clean, 64); err == nil {
		l.approximate(n, "P2G3020", "a whole number past int64",
			"the number became a float64",
			"This whole number does not fit in 64 signed bits, which is where Perl "+
				"silently switches to a double and starts rounding: past 2^53 a "+
				"float64 cannot hold every whole number, so nearby values collapse "+
				"into one.",
			"Where the exact value matters, use math/big.Int; a float64 here keeps "+
				"the original's behaviour, rounding included.",
			"explicit-conversions-no-coercion")
	}
}

// listLit lowers a parenthesised list reached as a plain expression.
//
// Everything that genuinely consumes a list, an array being filled, a sub
// being called, a loop being fed, takes the list apart itself, so reaching
// here means the parentheses stand in an ordinary expression position.
func (l *Lowerer) listLit(n *ast.List) ir.Expr {
	parts, t := l.listParts(n.Elems)
	// One part means the parentheses were grouping, not a container: Perl
	// flattens, so (@a) is @a, and ($x) in a value position is $x. Wrapping
	// the part in a slice literal would invent a container Perl never built,
	// and a one-element slice where a number was wanted reads as 0.
	if len(parts) == 1 {
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
	if c := l.recordFor(n, l.recordHint()); c != nil {
		return l.recordLit(c, n)
	}
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
	// A table of callbacks is the one case where several function literals
	// share a slot, and Go's slot has one type. They are lowered to one
	// signature so the map can hold them and so calling through it needs no
	// assertion.
	saved := l.uniformFn
	l.uniformFn = l.uniformFn || closureTable(flat)
	defer func() { l.uniformFn = saved }()

	var seen []*ir.Type
	sawUndef := false
	for i := 0; i+1 < len(flat); i += 2 {
		k := l.expr(flat[i])
		v := l.expr(flat[i+1])
		if k == nil || v == nil {
			continue
		}
		keys = append(keys, l.toStr(k, flat[i]))
		vals = append(vals, v)
		if isUndefLiteral(flat[i+1]) {
			// undef says the values can be absent, and says nothing about
			// what the present ones are. Counting it as evidence would settle
			// the whole map at `any` on the strength of the one key that
			// holds nothing.
			sawUndef = true
			continue
		}
		seen = append(seen, v.Type())
	}
	// The values decide the map's value type together, and two that no single
	// Go type covers settle it at any for good.
	//
	// A value whose own type did not resolve settles it too, which is where a
	// literal differs from a variable. Inference treats `any` as an absence of
	// evidence because a variable goes on being used and something later may
	// pin it down. A literal has all its evidence here: `{ path => $path, secs
	// => $secs + 0 }` with $path unresolved is not a map of numbers, and
	// deciding that it is means writing the path through a numeric conversion
	// and storing a zero.
	t = joinAll(seen)
	for _, one := range seen {
		if one == nil || one.Kind == ir.Any {
			t = ir.TAny
			break
		}
	}
	// Values that are all scalars, text here and numbers there, share the
	// string form every Perl scalar carries: the numbers are written in at
	// their sources, which is what Perl did silently.
	if isUnresolved(t) {
		if s := stringlyResolve(seen); s != nil {
			t = s
		}
	}
	t = usableElem(t, seen)
	if t == nil {
		t = ir.TAny
	}
	// A literal with undef among its values needs a value type with room for
	// nothing, which for a Go scalar means a pointer.
	if sawUndef {
		t = nullable(t)
	}
	for i, v := range vals {
		vals[i] = l.assignable(v, t, nil)
	}
	return keys, vals, t
}

// closureTable reports whether a flat key/value list is a table of
// callbacks: more than one of its values is written as a sub.
//
// One closure on its own keeps the signature its body implies, which is the
// better Go. Two or more in one collection cannot, because a map has a single
// value type and the two rarely agree about what they take or return.
// closureList reports whether a list holds more than one function literal.
func closureList(es []ast.Expr) bool {
	subs := 0
	var count func(ast.Expr)
	count = func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.AnonSub:
			subs++
		case *ast.List:
			for _, el := range n.Elems {
				count(el)
			}
		case *ast.BinOp:
			if n.Op == "," {
				count(n.L)
				count(n.R)
			}
		}
	}
	for _, e := range es {
		count(e)
	}
	return subs > 1
}

func closureTable(flat []ast.Expr) bool {
	subs := 0
	for i := 1; i < len(flat); i += 2 {
		if _, ok := flat[i].(*ast.AnonSub); ok {
			subs++
		}
	}
	return subs > 1
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
		expr := l.unwrapNil(p.expr)
		p.expr = expr
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

// todoExpr records a refusal and produces the expression that stands in for it.
func (l *Lowerer) todoExpr(n ast.Node, code, construct, short, message, advice string, concepts ...string) ir.Expr {
	todo := l.refuse(n, code, construct, short, message, advice, concepts...)
	return l.stub(todo, ir.TAny)
}

// stub builds the stand-in for a refused expression: one call, naming the
// diagnostic code and the missing behaviour, that yields the zero value of the
// type the position wanted.
//
// Three things had to be true of it. It has to read as code, because the line
// it sits in usually converted well and a reader is there to learn from it: an
// inline `func() any { panic(...) }()` at every refusal turned a statement with
// three method calls in it into something nobody could follow. It has to carry
// the diagnostic code, so the line ties back to the entry in the report that
// explains it. And it must not stop the program, because a refusal near the top
// of a file would then make every converted line below it dead code, which is
// the whole of what a partial conversion has to offer.
func (l *Lowerer) stub(todo ir.Todo, t *ir.Type) ir.Expr {
	t = stubType(t)
	fn := ir.NewIdent(l.use(hNotImplemented)+"["+t.Go(nil)+"]", nil)
	x := ir.CallOf(fn, t, ir.Str(strconv.Quote(todo.Code)), ir.Str(strconv.Quote(todoWording(todo))))
	todo.Spelled = true
	ir.MetaOf(x).Todo = &todo
	return x
}

// stubType picks the type to instantiate the stand-in at. A type that needs an
// import cannot be spelled in the helper's type argument without the emitter
// being told about that import, and one that has no spelling at all cannot be
// written down anywhere, so both degrade to any: the same type the refusal
// would have produced before it knew any better.
func stubType(t *ir.Type) *ir.Type {
	if spellableType(t) {
		return t
	}
	return ir.TAny
}

// spellableType reports whether a type can be written into a type argument
// without an import.
func spellableType(t *ir.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case ir.Invalid, ir.Void, ir.Func:
		return false
	case ir.Named:
		return t.Import == "" && t.Name != ""
	case ir.Slice, ir.Pointer:
		return spellableType(t.Elem)
	case ir.Map:
		return spellableType(t.Elem) && (t.Key == nil || spellableType(t.Key))
	}
	return true
}

// todoWording is the sentence the stand-in prints at run time. It is the clean
// wording, so the two renderings of the program say exactly the same thing.
func todoWording(t ir.Todo) string {
	if t.Short != "" {
		return t.Short
	}
	return "not implemented"
}

// usableElem widens a collection's element type when the values cannot all be
// stored in it.
//
// Two function types that differ are the case this exists for. Go converts
// between numbers and between named types with the same shape, but there is no
// conversion at all between func() and func() string: a collection holding
// both is a collection of `any`, and saying otherwise produces a literal that
// does not compile.
func usableElem(t *ir.Type, seen []*ir.Type) *ir.Type {
	if t == nil || t.Kind != ir.Func {
		return t
	}
	for _, one := range seen {
		if one != nil && one.Kind == ir.Func && !one.Equal(t) {
			return ir.TAny
		}
	}
	return t
}
