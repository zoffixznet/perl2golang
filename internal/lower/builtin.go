package lower

import (
	"strings"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// callExpr lowers a call used for its value.
func (l *Lowerer) callExpr(n *ast.Call) ir.Expr {
	if s, ok := l.subs[n.Name]; ok && !isBuiltinName(n.Name) {
		return l.callSub(s, n)
	}
	if x := l.builtin(n); x != nil {
		return x
	}
	if s, ok := l.subs[n.Name]; ok {
		return l.callSub(s, n)
	}
	return l.todoExpr(n, "P2G3599", n.Name,
		"the "+n.Name+" function is not implemented",
		"There is no rule for "+n.Name+" yet, and guessing at one would produce Go "+
			"that looks right and behaves differently.",
		"Write the equivalent by hand. `go doc` over the standard library is the "+
			"fastest way to find what corresponds.")
}

// callStatement lowers a call whose value is discarded, which is where the
// list-modifying builtins belong.
func (l *Lowerer) callStatement(n *ast.Call) []ir.Stmt {
	if sts, ok := l.statementFormOK(n); ok {
		return sts
	}
	x := l.callExpr(n)
	if x == nil {
		return nil
	}
	// A call whose value is thrown away is a statement in Go only if it is a
	// call; anything else would not compile.
	if _, isCall := x.(*ir.Call); !isCall {
		return nil
	}
	st := exprStmt(x)
	l.setProv(st, n)
	return []ir.Stmt{st}
}

// statementForm lowers the builtins that only make sense as statements.
func (l *Lowerer) statementForm(n *ast.Call) []ir.Stmt {
	sts, _ := l.statementFormOK(n)
	return sts
}

// statementFormOK reports whether a name has a statement form, and lowers it.
func (l *Lowerer) statementFormOK(n *ast.Call) ([]ir.Stmt, bool) {
	switch n.Name {
	case "print", "say", "printf", "push", "unshift", "chomp", "die", "warn",
		"exit", "delete", "close", "open", "return", "opendir", "closedir":
		return l.statementOnly(n), true
	}
	return nil, false
}

// statementOnly dispatches the statement-shaped builtins.
func (l *Lowerer) statementOnly(n *ast.Call) []ir.Stmt {
	switch n.Name {
	case "print", "say":
		return l.printCall(n, n.Name == "say")
	case "printf":
		return l.printfCall(n)
	case "push":
		return l.pushCall(n)
	case "unshift":
		return l.unshiftCall(n)
	case "chomp":
		return l.chompCall(n)
	case "die":
		return l.dieCall(n)
	case "warn":
		return l.warnCall(n)
	case "exit":
		return l.exitCall(n)
	case "delete":
		return l.deleteCall(n)
	case "close":
		return l.closeCall(n)
	case "open":
		return l.openCall(n)
	case "opendir":
		return l.opendirCall(n)
	case "closedir":
		return l.closedirCall(n)
	}
	return nil
}

// builtin dispatches the value-producing builtins. It returns nil when the
// name is not one it handles.
func (l *Lowerer) builtin(n *ast.Call) ir.Expr {
	switch n.Name {
	case "sprintf":
		return l.sprintfCall(n)
	case "join":
		return l.joinCall(n)
	case "split":
		return l.splitCall(n)
	case "scalar":
		if len(n.Args) == 1 {
			return l.scalar(n.Args[0])
		}
	case "length":
		return l.lengthCall(n)
	case "defined":
		if len(n.Args) == 1 {
			return l.definedExpr(n.Args[0], n)
		}
		return l.definedExpr(nil, n)
	case "exists":
		if len(n.Args) == 1 {
			return l.existsExpr(n.Args[0], n)
		}
	case "keys":
		return l.keysCall(n, false)
	case "values":
		return l.keysCall(n, true)
	case "sort":
		return l.sortCall(n)
	case "reverse":
		return l.reverseCall(n)
	case "map":
		return l.mapCall(n)
	case "grep":
		return l.grepCall(n)
	case "pop":
		return l.popCall(n, false)
	case "shift":
		return l.popCall(n, true)
	case "splice":
		return l.spliceCall(n)
	case "uc":
		return call("strings", "strings", "ToUpper", ir.TString, l.argStr(n, 0))
	case "lc":
		return call("strings", "strings", "ToLower", ir.TString, l.argStr(n, 0))
	case "ucfirst":
		return l.helperCall(hUcFirst, ir.TString, l.argStr(n, 0))
	case "lcfirst":
		return l.helperCall(hLcFirst, ir.TString, l.argStr(n, 0))
	case "substr":
		return l.substrCall(n)
	case "index":
		return l.indexCall(n, false)
	case "rindex":
		return l.indexCall(n, true)
	case "abs":
		return l.absCall(n)
	case "int":
		return l.toInt(l.argExpr(n, 0), n)
	case "sqrt":
		return call("math", "math", "Sqrt", ir.TFloat, l.argFloat(n, 0))
	case "chr":
		return conversion(ir.TString, conversion(ir.NamedType("rune", ""), l.argInt(n, 0)))
	case "ord":
		return l.helperCall(hOrd, ir.TInt, l.argStr(n, 0))
	case "hex":
		return l.radixCall(n, 16)
	case "oct":
		return l.radixCall(n, 8)
	case "time":
		return call("time", "time", "Now", ir.NamedType("time.Time", "time"))
	case "chomp", "chop":
		return l.chompExpr(n)

	case "close", "open", "print", "printf", "say", "die", "warn", "exit",
		"push", "unshift", "delete":
		// These reach expression position because of the `X or die` idiom.
		// Perl's answer there is a truth value, so the statements run first
		// and the value they produce is success. Only names the statement
		// layer handles belong here, or the two would call each other.
		for _, st := range l.statementForm(n) {
			l.emit(st)
		}
		return ir.BoolLit(true)

	case "binmode":
		l.inform(n, "P2G6060", "binmode",
			"binmode sets the encoding layer on a handle. Go reads and writes bytes "+
				"and leaves decoding to the caller, so there is no layer to set: text is "+
				"already UTF-8 and golang.org/x/text/encoding handles anything else.")
		return ir.BoolLit(true)

	case "eof":
		return ir.BoolLit(false)

	case "sleep":
		if argCount(n) > 0 {
			out := call("time", "time", "Sleep", ir.TVoid,
				ir.Bin("*", conversion(ir.NamedType("time.Duration", "time"), l.argInt(n, 0)),
					ir.Pkg("time", "time", "Second", nil), ir.NamedType("time.Duration", "time")))
			l.note(out, "A Go duration is its own type rather than a number of seconds, "+
				"so the unit is part of the value and time.Sleep(2) will not compile.")
			return out
		}
		return nil
	case "wantarray":
		return l.todoExpr(n, "P2G2031", "wantarray",
			"wantarray has no Go equivalent",
			"wantarray reports whether the caller wanted a list or a scalar, so one "+
				"sub can return two different shapes. Go has no calling context at all: "+
				"a function's result types are fixed by its signature.",
			"Split the sub into two functions with clear names, or return the list "+
				"and let the caller take its length.",
			"multiple-return-values")
	case "ref":
		return l.refCall(n)
	case "undef":
		return l.undefCall(n)
	case "readdir":
		return l.readdirCall(n)
	case "unlink":
		return l.unlinkCall(n)
	case "bless":
		return l.todoExpr(n, "P2G7001", "bless",
			"bless has no Go equivalent",
			"bless marks a reference as belonging to a class, which is how Perl "+
				"builds objects. Go has no such operation: methods are declared on a "+
				"named type, and a value's type never changes.",
			"Declare a struct type and give it methods with a receiver. The "+
				"constructor becomes an ordinary function returning that type.",
			"methods-and-receivers", "structs-and-embedding")
	case "eval":
		return l.evalCall(n)
	case "local":
		return l.todoExpr(n, "P2G2001", "local",
			"local's dynamic scoping is not implemented",
			"local temporarily replaces a package variable's value for the duration "+
				"of the enclosing block, and everything called from inside sees the new "+
				"value. Go has only lexical scoping.",
			"Pass the value as an argument, or save and restore it around the block "+
				"with defer.",
			"defer-timing")
	case "each":
		return l.todoExpr(n, "P2G5570", "each",
			"each keeps hidden iterator state",
			"each walks a hash one pair at a time using an iterator stored inside the "+
				"hash itself, which is why a nested or abandoned each loop misbehaves. Go "+
				"has no such state.",
			"Range over the map directly with `for k, v := range m`, which gives both "+
				"halves and keeps no state between loops.",
			"map-iteration-order")
	case "sum", "sum0", "max", "min", "first", "uniq", "shuffle", "reduce":
		return l.listUtil(n)
	case "floor", "ceil", "fmod", "strftime":
		return l.posix(n)
	case "basename", "dirname":
		return l.pathCall(n)
	}
	return nil
}

// isBuiltinName reports whether a name is one of the builtins, so a sub that
// shadows one is still called.
func isBuiltinName(name string) bool {
	switch name {
	case "print", "printf", "say", "sprintf", "join", "split", "push", "pop",
		"shift", "unshift", "splice", "reverse", "sort", "map", "grep", "keys",
		"values", "each", "exists", "delete", "defined", "scalar", "length",
		"substr", "index", "rindex", "uc", "lc", "ucfirst", "lcfirst", "chomp",
		"chop", "abs", "int", "sqrt", "hex", "oct", "ord", "chr", "die", "warn",
		"exit", "open", "close", "eof", "ref", "bless", "wantarray", "local",
		"eval", "time", "sleep":
		return true
	}
	return false
}

// isListBuiltin reports whether a call produces a list, which decides what
// scalar context does to it.
func isListBuiltin(name string) bool {
	switch name {
	case "split", "keys", "values", "sort", "map", "grep", "reverse", "uniq", "shuffle":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Argument helpers

func (l *Lowerer) argExpr(n *ast.Call, i int) ir.Expr {
	args := flatten(argList(n))
	if i >= len(args) {
		if len(l.topicStack) > 0 {
			return l.topicStack[len(l.topicStack)-1]
		}
		return ir.Str(`""`)
	}
	return l.expr(args[i])
}

func (l *Lowerer) argNode(n *ast.Call, i int) ast.Expr {
	args := flatten(argList(n))
	if i >= len(args) {
		return nil
	}
	return args[i]
}

func (l *Lowerer) argStr(n *ast.Call, i int) ir.Expr {
	return l.toStr(l.argExpr(n, i), l.argNode(n, i))
}

func (l *Lowerer) argInt(n *ast.Call, i int) ir.Expr {
	return l.toInt(l.argExpr(n, i), l.argNode(n, i))
}

func (l *Lowerer) argFloat(n *ast.Call, i int) ir.Expr {
	return l.toFloat(l.argExpr(n, i), l.argNode(n, i))
}

func argList(n *ast.Call) ast.Expr {
	if len(n.Args) == 1 {
		return n.Args[0]
	}
	return &ast.List{Elems: n.Args}
}

func argCount(n *ast.Call) int { return len(flatten(argList(n))) }

// ---------------------------------------------------------------------------
// Output

// printCall lowers print and say.
func (l *Lowerer) printCall(n *ast.Call, newline bool) []ir.Stmt {
	args := flatten(argList(n))
	var dest ir.Expr
	if len(args) > 0 {
		if fh, ok := args[0].(*ast.FileHandle); ok {
			dest = l.fileHandleExpr(fh)
			args = args[1:]
		}
	}
	if len(args) == 0 {
		args = []ast.Expr{&ast.Var{Sigil: '$', Name: "_"}}
	}

	// One interpolated string is the overwhelmingly common case, and it maps
	// straight onto Printf, which is what a Go developer writes.
	if len(args) == 1 {
		if il, ok := args[0].(*ast.InterpLit); ok {
			if st, done := l.printfFromInterp(il, dest, newline, n); done {
				return st
			}
		}
	}

	var parts []ir.Expr
	for _, a := range args {
		x := l.expr(a)
		if x == nil {
			continue
		}
		if t := typeOrAny(x); t.Kind == ir.Float || t.Kind == ir.Bool || t.Kind == ir.Slice || t.Kind == ir.Any {
			x = l.toStr(x, a)
		}
		parts = append(parts, x)
	}
	if newline {
		parts = append(parts, ir.Str(`"\n"`))
	}

	fn := "Print"
	var callArgs []ir.Expr
	if dest != nil {
		fn = "Fprint"
		callArgs = append(callArgs, dest)
	}
	callArgs = append(callArgs, parts...)
	out := call("fmt", "fmt", fn, ir.TVoid, callArgs...)
	st := exprStmt(out)
	l.setProv(st, n)
	l.note(st, "fmt.Print writes its arguments one after another with no separator "+
		"unless two of them are both non-strings, which is why the newline is passed "+
		"explicitly. fmt.Println would add spaces and a newline of its own.")
	return []ir.Stmt{st}
}

// printfFromInterp turns `print "text $x\n"` into fmt.Printf, which keeps the
// generated line looking like the original and reads better than concatenation.
func (l *Lowerer) printfFromInterp(il *ast.InterpLit, dest ir.Expr, newline bool, at ast.Node) ([]ir.Stmt, bool) {
	var pieces []interpPiece
	for _, p := range il.Parts {
		if s, ok := p.(*ast.StrLit); ok {
			pieces = append(pieces, interpPiece{text: s.Value})
			continue
		}
		x := l.expr(p)
		if x == nil {
			return nil, false
		}
		pieces = append(pieces, interpPiece{expr: x, node: p})
	}
	format, args := l.sprintfParts(pieces)
	if newline {
		format += "\n"
	}

	fn := "Printf"
	var callArgs []ir.Expr
	if dest != nil {
		fn = "Fprintf"
		callArgs = append(callArgs, dest)
	}
	if len(args) == 0 {
		fn = strings.TrimSuffix(fn, "f")
		callArgs = append(callArgs, ir.Str(quote(format)))
		out := call("fmt", "fmt", fn, ir.TVoid, callArgs...)
		st := exprStmt(out)
		l.setProv(st, at)
		return []ir.Stmt{st}, true
	}
	callArgs = append(callArgs, ir.Str(quote(format)))
	callArgs = append(callArgs, args...)
	out := call("fmt", "fmt", fn, ir.TVoid, callArgs...)
	st := exprStmt(out)
	l.setProv(st, at)
	l.note(st, "A double-quoted Perl string interpolates its variables. Go has no "+
		"interpolation, so the text becomes a format string and the values become "+
		"arguments. %s takes text, %d takes a whole number, and go vet checks that "+
		"they line up.",
		"vet-and-staticcheck")
	return []ir.Stmt{st}, true
}

// printfCall lowers printf.
func (l *Lowerer) printfCall(n *ast.Call) []ir.Stmt {
	args := flatten(argList(n))
	var dest ir.Expr
	if len(args) > 0 {
		if fh, ok := args[0].(*ast.FileHandle); ok {
			dest = l.fileHandleExpr(fh)
			args = args[1:]
		}
	}
	if len(args) == 0 {
		return nil
	}
	format, ok := staticString(args[0])
	var callArgs []ir.Expr
	fn := "Printf"
	if dest != nil {
		fn = "Fprintf"
		callArgs = append(callArgs, dest)
	}
	if !ok {
		l.approximate(n, "P2G5020", "printf with a computed format",
			"the format string is built at run time",
			"The format is not a literal, so it cannot be translated at conversion "+
				"time. Perl and Go agree on most of printf, but not all of it.",
			"Check the format for Perl-only features such as %v, and for %s applied "+
				"to a number, which Go will not stringify on its own.")
		callArgs = append(callArgs, l.toStr(l.expr(args[0]), args[0]))
		for _, a := range args[1:] {
			callArgs = append(callArgs, l.expr(a))
		}
	} else {
		var vals []ir.Expr
		for _, a := range args[1:] {
			for _, one := range flatten(a) {
				vals = append(vals, l.expr(one))
			}
		}
		goFormat, goArgs := l.perlFormat(format, vals, n)
		callArgs = append(callArgs, ir.Str(quote(goFormat)))
		callArgs = append(callArgs, goArgs...)
	}
	out := call("fmt", "fmt", fn, ir.TVoid, callArgs...)
	st := exprStmt(out)
	l.setProv(st, n)
	l.note(st, "printf carries over almost unchanged. The difference is that Go "+
		"checks the verbs against the argument types, so %s given a number is a "+
		"reported mistake rather than a silent stringification.",
		"vet-and-staticcheck")
	return []ir.Stmt{st}
}

// sprintfCall lowers sprintf.
func (l *Lowerer) sprintfCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 0 {
		return ir.Str(`""`)
	}
	format, ok := staticString(args[0])
	if !ok {
		var vals []ir.Expr
		for _, a := range args {
			vals = append(vals, l.expr(a))
		}
		return l.helperCall(hSprintf, ir.TString, vals...)
	}
	var vals []ir.Expr
	for _, a := range args[1:] {
		for _, one := range flatten(a) {
			vals = append(vals, l.expr(one))
		}
	}
	goFormat, goArgs := l.perlFormat(format, vals, n)
	return call("fmt", "fmt", "Sprintf", ir.TString,
		append([]ir.Expr{ir.Str(quote(goFormat))}, goArgs...)...)
}

// ---------------------------------------------------------------------------
// Lists

func (l *Lowerer) joinCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) < 2 {
		return ir.Str(`""`)
	}
	sep := l.toStr(l.expr(args[0]), args[0])
	var list ir.Expr
	if len(args) == 2 {
		list = l.list(args[1])
	} else {
		parts, t := l.listParts(args[1:])
		list = composite(ir.SliceOf(t), nil, parts)
	}
	out := l.stringsJoin(list, sep)
	l.note(out, "strings.Join is join with the arguments the other way round: the "+
		"slice comes first, the separator second.")
	return out
}

func (l *Lowerer) lengthCall(n *ast.Call) ir.Expr {
	s := l.argStr(n, 0)
	out := lenOf(s)
	l.note(out, "Go's len on a string counts bytes, and so does Perl's length "+
		"without `use utf8`. For text that is not ASCII the two agree only while both "+
		"are counting bytes; utf8.RuneCountInString counts characters.",
		"strings-are-bytes")
	return out
}

func (l *Lowerer) keysCall(n *ast.Call, wantValues bool) ir.Expr {
	m := l.argExpr(n, 0)
	t := typeOrAny(m)
	if t.Kind != ir.Map {
		return composite(ir.SliceOf(ir.TString), nil, nil)
	}
	fn := "Keys"
	elem := ir.TString
	if wantValues {
		fn = "Values"
		elem = elemOf(t)
	}
	iter := call("maps", "maps", fn, nil, m)
	out := call("slices", "slices", "Collect", ir.SliceOf(elem), iter)
	l.note(out, "maps.Keys hands back an iterator rather than a slice, because most "+
		"loops never need the slice. slices.Collect materialises it when, as here, a "+
		"list is what was wanted. The order is deliberately randomised in Go, exactly "+
		"as it is in Perl, so sort it if the output has to be stable.",
		"map-iteration-order")
	return out
}

func (l *Lowerer) reverseCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 1 {
		if x := l.expr(args[0]); typeOrAny(x).Kind == ir.String {
			out := l.helperCall(hReverseStr, ir.TString, x)
			l.note(out, "reverse on a single string reverses its characters. Go has no "+
				"built-in for it, partly because reversing text is only meaningful "+
				"character by character, not byte by byte.",
				"strings-are-bytes")
			return out
		}
	}
	src := l.list(argList(n))
	name := l.tmp("reversed")
	clone := assign(":=", []ir.Expr{ir.NewIdent(name, typeOrAny(src))},
		[]ir.Expr{call("slices", "slices", "Clone", typeOrAny(src), src)})
	l.note(clone, "Perl's reverse returns a new list and leaves the original alone. "+
		"slices.Reverse works in place, so the slice is cloned first. Skipping the "+
		"clone would quietly reorder the caller's data, because a slice shares its "+
		"backing array.",
		"slice-aliasing-and-copy")
	l.emit(clone)
	l.emit(exprStmt(call("slices", "slices", "Reverse", ir.TVoid, ir.NewIdent(name, typeOrAny(src)))))
	return ir.NewIdent(name, typeOrAny(src))
}

func (l *Lowerer) pushCall(n *ast.Call) []ir.Stmt {
	args := flatten(argList(n))
	if len(args) < 2 {
		return nil
	}
	target := l.assignTarget(args[0])
	if target == nil {
		target = l.expr(args[0])
	}
	elem := elemOf(typeOrAny(target))
	var vals []ir.Expr
	for _, a := range args[1:] {
		x := l.expr(a)
		if x == nil {
			continue
		}
		if typeOrAny(x).Kind == ir.Slice && elem.Kind != ir.Slice {
			// Perl flattens an array into the push; Go spreads it with ... .
			c := appendTo(target, x)
			c.Ellipsis = true
			st := assign("=", []ir.Expr{target}, []ir.Expr{c})
			l.setProv(st, n)
			return []ir.Stmt{st}
		}
		vals = append(vals, l.assignable(x, elem, a))
	}
	if b := l.bindingOfTarget(args[0]); b != nil {
		for _, a := range args[1:] {
			l.observeElem(b, typeOrAny(l.expr(a)))
		}
	}
	st := assign("=", []ir.Expr{target}, []ir.Expr{appendTo(target, vals...)})
	l.setProv(st, n)
	l.note(st, "append returns a new slice header rather than modifying the old one, "+
		"so the result has to be assigned back. Forgetting that assignment is the "+
		"classic first Go bug, and the compiler cannot catch it.",
		"slice-aliasing-and-copy")
	return []ir.Stmt{st}
}

func (l *Lowerer) unshiftCall(n *ast.Call) []ir.Stmt {
	args := flatten(argList(n))
	if len(args) < 2 {
		return nil
	}
	target := l.assignTarget(args[0])
	if target == nil {
		return nil
	}
	t := typeOrAny(target)
	elem := elemOf(t)
	var vals []ir.Expr
	for _, a := range args[1:] {
		vals = append(vals, l.assignable(l.expr(a), elem, a))
	}
	head := composite(t, nil, vals)
	c := appendTo(head, target)
	c.Ellipsis = true
	st := assign("=", []ir.Expr{target}, []ir.Expr{c})
	l.setProv(st, n)
	l.note(st, "Go has no unshift. Prepending means building a new slice with the new "+
		"elements first and appending the old contents, which copies the whole slice. "+
		"A queue that is fed at the front is usually better served by a different "+
		"shape, such as a container/list or reversing the direction of the loop.",
		"slices-not-arrays")
	return []ir.Stmt{st}
}

// popCall lowers pop and shift, both of which return an element and shorten
// the array.
func (l *Lowerer) popCall(n *ast.Call, front bool) ir.Expr {
	var targetNode ast.Expr
	args := flatten(argList(n))
	if len(args) > 0 {
		targetNode = args[0]
	} else if l.curSub != nil && l.curSub.VarArgs != nil {
		targetNode = &ast.Var{Sigil: '@', Name: "args"}
	}
	var target ir.Expr
	if targetNode != nil {
		target = l.assignTarget(targetNode)
	}
	if target == nil && l.curSub != nil && l.curSub.VarArgs != nil {
		target = ir.NewIdent(l.curSub.VarArgs.Go, l.curSub.VarArgs.Type)
	}
	if target == nil {
		return ir.Nil(ir.TAny)
	}
	t := typeOrAny(target)
	elem := elemOf(t)

	name := l.tmp(valueName(front))
	var pick ir.Expr
	var rest ir.Expr
	if front {
		pick = index(target, ir.IntLit("0"), elem)
		rest = slicing(target, ir.IntLit("1"), nil, t)
	} else {
		last := ir.Bin("-", lenOf(target), ir.IntLit("1"), ir.TInt)
		pick = index(target, last, elem)
		rest = slicing(target, nil, ir.Bin("-", lenOf(target), ir.IntLit("1"), ir.TInt), t)
	}
	take := assign(":=", []ir.Expr{ir.NewIdent(name, elem)}, []ir.Expr{pick})
	l.setProv(take, n)
	if front {
		l.note(take, "shift takes the first element and shortens the array. Go has no "+
			"such operation: the element is read, then the slice is re-sliced past it. "+
			"Re-slicing does not copy, it just moves the start of the window.",
			"slices-not-arrays", "slice-aliasing-and-copy")
	} else {
		l.note(take, "pop takes the last element and shortens the array. In Go that is "+
			"a read of the last index followed by a re-slice.",
			"slices-not-arrays")
	}
	l.emit(take)
	l.emit(assign("=", []ir.Expr{target}, []ir.Expr{rest}))
	return ir.NewIdent(name, elem)
}

func valueName(front bool) string {
	if front {
		return "first"
	}
	return "last"
}

func (l *Lowerer) spliceCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 3 {
		target := l.assignTarget(args[0])
		if target != nil {
			t := typeOrAny(target)
			off := l.toInt(l.expr(args[1]), args[1])
			count := l.toInt(l.expr(args[2]), args[2])
			out := call("slices", "slices", "Delete", t, target,
				off, ir.Bin("+", off, count, ir.TInt))
			l.emit(assign("=", []ir.Expr{target}, []ir.Expr{out}))
			l.approximate(n, "P2G5580", "splice",
				"splice becomes a delete and returns nothing",
				"splice removes a run of elements and returns what it removed. "+
					"slices.Delete removes the run but returns the shortened slice, not the "+
					"removed part.",
				"If the removed elements are needed, copy them out with slices.Clone "+
					"before deleting.",
				"slice-aliasing-and-copy")
			return target
		}
	}
	return l.todoExpr(n, "P2G5581", "splice",
		"this form of splice is not implemented",
		"splice can insert, remove and replace in one call, with up to four "+
			"arguments and a meaningful return value in both contexts.",
		"Use slices.Delete, slices.Insert or slices.Replace, which each do one of "+
			"those jobs and are clearer at the call site.",
		"slices-not-arrays")
}

// ---------------------------------------------------------------------------
// Strings

func (l *Lowerer) substrCall(n *ast.Call) ir.Expr {
	switch argCount(n) {
	case 2:
		return l.helperCall(hSubstrFrom, ir.TString, l.argStr(n, 0), l.argInt(n, 1))
	case 3:
		out := l.helperCall(hSubstr, ir.TString, l.argStr(n, 0), l.argInt(n, 1), l.argInt(n, 2))
		l.note(out, "Go slices a string with s[from:to], counted in bytes, and panics "+
			"when the bounds are wrong. substr counts from the end for a negative "+
			"offset, clips instead of failing, and treats the third argument as a "+
			"length rather than an end. The helper keeps those rules.",
			"strings-are-bytes")
		return out
	case 4:
		return l.helperCall(hSubstrReplace, ir.TString,
			l.argStr(n, 0), l.argInt(n, 1), l.argInt(n, 2), l.argStr(n, 3))
	}
	return ir.Str(`""`)
}

func (l *Lowerer) indexCall(n *ast.Call, last bool) ir.Expr {
	if argCount(n) == 2 {
		fn := "Index"
		if last {
			fn = "LastIndex"
		}
		out := call("strings", "strings", fn, ir.TInt, l.argStr(n, 0), l.argStr(n, 1))
		l.note(out, "strings.Index returns the byte offset of the first match, or -1 "+
			"when there is none, which is the same answer index gives.")
		return out
	}
	helper := hIndexOf
	if last {
		helper = hLastIndexOf
	}
	return l.helperCall(helper, ir.TInt, l.argStr(n, 0), l.argStr(n, 1), l.argInt(n, 2))
}

func (l *Lowerer) absCall(n *ast.Call) ir.Expr {
	x := l.argExpr(n, 0)
	if typeOrAny(x).Kind == ir.Int {
		out := ir.CallOf(ir.NewIdent("max", nil), ir.TInt, x, ir.Un("-", x, ir.TInt))
		l.note(out, "Go's math.Abs works in float64 and there is no integer version. "+
			"For an int, max(x, -x) is the whole of it, using the built-in max added "+
			"in Go 1.21.")
		return out
	}
	return call("math", "math", "Abs", ir.TFloat, l.toFloat(x, l.argNode(n, 0)))
}

func (l *Lowerer) radixCall(n *ast.Call, base int) ir.Expr {
	s := l.argStr(n, 0)
	name := l.tmp("n")
	parsed := assign(":=", []ir.Expr{ir.NewIdent(name, ir.TInt), ir.NewIdent("_", nil)},
		[]ir.Expr{call("strconv", "strconv", "ParseInt", ir.TInt, s, ir.IntLit(itoa(base)), ir.IntLit("64"))})
	l.approximate(n, "P2G5050", "hex or oct",
		"the parse error is discarded here",
		"Perl's hex and oct return 0 for text they cannot read. Go's "+
			"strconv.ParseInt returns an error instead, which is the shape every "+
			"conversion in Go takes.",
		"Check the second result and decide what a bad value should mean, rather "+
			"than letting it become 0.",
		"errors-are-values", "strconv-parsing")
	l.emit(parsed)
	return conversion(ir.TInt, ir.NewIdent(name, ir.TInt))
}

func (l *Lowerer) chompCall(n *ast.Call) []ir.Stmt {
	target := l.assignTarget(l.chompTarget(n))
	if target == nil {
		return nil
	}

	// chomp on an array chomps every element, which in Go is a loop that
	// writes back through the index.
	if typeOrAny(target).Kind == ir.Slice {
		idx := l.tmp("i")
		loop := &ir.Range{
			Key:    ir.NewIdent(idx, ir.TInt),
			X:      target,
			Define: true,
			Body: &ir.Block{Stmts: []ir.Stmt{
				assign("=", []ir.Expr{index(target, ir.NewIdent(idx, ir.TInt), ir.TString)},
					[]ir.Expr{call("strings", "strings", "TrimSuffix", ir.TString,
						index(target, ir.NewIdent(idx, ir.TInt), ir.TString), ir.Str(`"\n"`))}),
			}},
		}
		l.setProv(loop, n)
		l.note(loop, "chomp applied to an array trims every element. Go has no "+
			"operation that reaches into a whole slice at once, so the loop writes "+
			"back through the index; assigning to the range variable would change a "+
			"copy and nothing else.",
			"range-is-not-foreach", "slice-aliasing-and-copy")
		return []ir.Stmt{loop}
	}

	st := assign("=", []ir.Expr{target},
		[]ir.Expr{call("strings", "strings", "TrimSuffix", ir.TString, target, ir.Str(`"\n"`))})
	l.setProv(st, n)
	l.note(st, "chomp removes one trailing newline and nothing else, which is exactly "+
		"strings.TrimSuffix. Note that bufio.Scanner has already removed the newline, "+
		"so a chomp after a Scan is unnecessary.",
		"bufio-scanner-limit")
	return []ir.Stmt{st}
}

func (l *Lowerer) chompTarget(n *ast.Call) ast.Expr {
	args := flatten(argList(n))
	if len(args) > 0 {
		return args[0]
	}
	return &ast.Var{Sigil: '$', Name: "_"}
}

// chompExpr lowers chomp used for its value, which is the number of characters
// it removed rather than the trimmed text.
func (l *Lowerer) chompExpr(n *ast.Call) ir.Expr {
	target := l.assignTarget(l.chompTarget(n))
	if target == nil {
		return ir.IntLit("0")
	}
	count := l.tmp("removed")
	decl := &ir.DeclStmt{Names: []string{count}, Type: ir.TInt}
	check := &ir.If{
		Cond: call("strings", "strings", "HasSuffix", ir.TBool, target, ir.Str(`"\n"`)),
		Then: &ir.Block{Stmts: []ir.Stmt{
			assign("=", []ir.Expr{ir.NewIdent(count, ir.TInt)}, []ir.Expr{ir.IntLit("1")}),
		}},
	}
	l.setProv(decl, n)
	l.note(decl, "chomp returns how many characters it removed, not the trimmed "+
		"text, so the count has to be worked out before the trim happens.")
	l.emit(decl)
	l.emit(check)
	for _, st := range l.chompCall(n) {
		l.emit(st)
	}
	return ir.NewIdent(count, ir.TInt)
}

// ---------------------------------------------------------------------------
// Program control

func (l *Lowerer) dieCall(n *ast.Call) []ir.Stmt {
	msg := l.dieMessage(n)
	l.usedExit = true
	var out []ir.Stmt
	out = append(out, exprStmt(l.writeTo(ir.Pkg("os", "os", "Stderr", nil), msg)))
	exit := exprStmt(call("os", "os", "Exit", ir.TVoid, ir.IntLit("255")))
	st := out[0]
	l.setProv(st, n)
	l.note(st, "die writes to standard error and ends the program with status 255. "+
		"Go has no die: a function reports failure by returning an error and lets the "+
		"caller decide. Inside main, printing and exiting is the honest equivalent; "+
		"inside a function, returning an error is what Go code does.",
		"errors-are-values", "panic-and-recover")
	out = append(out, exit)
	return out
}

// dieMessage builds the text die prints, adding the "at FILE line N." suffix
// Perl appends when the message does not end in a newline.
func (l *Lowerer) dieMessage(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 0 {
		return ir.Str(`"Died\n"`)
	}
	if text, ok := staticString(args[0]); ok && len(args) == 1 {
		if !strings.HasSuffix(text, "\n") {
			text += " at " + l.opts.File + " line " + itoa(posLine(n)) + ".\n"
			l.approximate(n, "P2G6520", "die without a trailing newline",
				"the file and line suffix is fixed at conversion time",
				"When a die message does not end in a newline, Perl appends \" at FILE "+
					"line N.\" using the line the die is on. The generated code has that "+
					"text baked in, so it will not follow the Go source if it moves.",
				"End the message with a newline to suppress the suffix, as Perl does.")
		}
		return ir.Str(quote(text))
	}
	var parts []ir.Expr
	for _, a := range args {
		parts = append(parts, l.toStr(l.expr(a), a))
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out = ir.Bin("+", out, p, ir.TString)
	}
	return out
}

// writeTo builds the call that writes one message to a stream, folding an
// fmt.Sprintf argument back into Fprintf rather than nesting the two.
func (l *Lowerer) writeTo(dest ir.Expr, msg ir.Expr) ir.Expr {
	if c, ok := msg.(*ir.Call); ok {
		if sel, ok := c.Fun.(*ir.Selector); ok && sel.Sel == "Sprintf" && sel.Import == "fmt" {
			args := append([]ir.Expr{dest}, c.Args...)
			return call("fmt", "fmt", "Fprintf", ir.TVoid, args...)
		}
	}
	return call("fmt", "fmt", "Fprint", ir.TVoid, dest, msg)
}

func (l *Lowerer) warnCall(n *ast.Call) []ir.Stmt {
	msg := l.dieMessage(n)
	st := exprStmt(l.writeTo(ir.Pkg("os", "os", "Stderr", nil), msg))
	l.setProv(st, n)
	l.note(st, "warn writes to standard error and carries on. In Go that is a plain "+
		"write to os.Stderr, or the log package when a timestamp and a prefix are "+
		"wanted.")
	return []ir.Stmt{st}
}

func (l *Lowerer) exitCall(n *ast.Call) []ir.Stmt {
	code := ir.Expr(ir.IntLit("0"))
	if argCount(n) > 0 {
		code = l.argInt(n, 0)
	}
	l.usedExit = true
	st := exprStmt(call("os", "os", "Exit", ir.TVoid, code))
	l.setProv(st, n)
	l.note(st, "os.Exit ends the program immediately. Deferred functions do not run, "+
		"and buffered output is not flushed, so anything that has to be written must "+
		"be written before this line.",
		"defer-timing")
	return []ir.Stmt{st}
}

func (l *Lowerer) deleteCall(n *ast.Call) []ir.Stmt {
	args := flatten(argList(n))
	if len(args) != 1 {
		return nil
	}
	hi, ok := args[0].(*ast.HashIndex)
	if !ok {
		return nil
	}
	m, key, _ := l.hashParts(hi)
	if m == nil || key == nil {
		return nil
	}
	st := exprStmt(ir.CallOf(ir.NewIdent("delete", nil), ir.TVoid, m, key))
	l.setProv(st, n)
	l.note(st, "Go's delete is a built-in taking the map and the key. Deleting a key "+
		"that is not there is not an error, and delete returns nothing, where Perl's "+
		"returns the removed value.")
	return []ir.Stmt{st}
}

func (l *Lowerer) evalCall(n *ast.Call) ir.Expr {
	return l.todoExpr(n, "P2G8001", "eval",
		"eval is not implemented",
		"eval either traps a die from the block inside it, or compiles a string as "+
			"Perl source at run time. Go has neither: there is no compiler at run time, "+
			"and errors travel as return values rather than by unwinding.",
		"For the block form, change the code inside to return an error and check "+
			"it. For the string form, there is no equivalent; a fixed set of "+
			"expressions becomes named functions selected from a map.",
		"errors-are-values", "panic-and-recover")
}

// ---------------------------------------------------------------------------
// Module functions

func (l *Lowerer) listUtil(n *ast.Call) ir.Expr {
	src := l.list(argList(n))
	t := typeOrAny(src)
	elem := elemOf(t)
	switch n.Name {
	case "max", "min":
		fn := "Max"
		if n.Name == "min" {
			fn = "Min"
		}
		out := call("slices", "slices", fn, elem, src)
		l.note(out, "slices.Max and slices.Min came into the standard library in Go "+
			"1.21. They panic on an empty slice, where List::Util returns undef, so "+
			"check the length first if that can happen.")
		return out
	case "sum", "sum0":
		name := l.tmp("total")
		acc := elem
		if !isNum(acc) {
			acc = ir.TFloat
		}
		decl := &ir.DeclStmt{Names: []string{name}, Type: acc}
		item := l.tmp("v")
		loop := &ir.Range{
			Key:    ir.NewIdent("_", ir.TInt),
			Value:  ir.NewIdent(item, elem),
			X:      src,
			Define: true,
			Body: &ir.Block{Stmts: []ir.Stmt{
				assign("+=", []ir.Expr{ir.NewIdent(name, acc)}, []ir.Expr{l.assignable(ir.NewIdent(item, elem), acc, nil)}),
			}},
		}
		l.note(decl, "Go has no sum function over a slice. The loop is the idiom, and "+
			"it is deliberately not hidden: the accumulator's type is visible, which "+
			"is where an overflow or a float rounding decision would be made.")
		l.emit(decl)
		l.emit(loop)
		return ir.NewIdent(name, acc)
	case "uniq":
		return l.todoExpr(n, "P2G7540", "List::Util uniq",
			"uniq is not implemented",
			"uniq removes repeated values while keeping the first of each.",
			"Range over the slice with a map[T]bool of what has been seen, appending "+
				"only the first occurrence of each value.",
			"maps-of-slices")
	}
	return l.todoExpr(n, "P2G7540", "List::Util "+n.Name,
		"the "+n.Name+" function is not implemented",
		"List::Util's "+n.Name+" has no direct counterpart in the Go standard "+
			"library.",
		"Write the loop directly. Go's standard library deliberately keeps very "+
			"few list combinators, and the explicit loop is the accepted style.",
		"small-stdlib-philosophy")
}

func (l *Lowerer) posix(n *ast.Call) ir.Expr {
	switch n.Name {
	case "floor":
		return call("math", "math", "Floor", ir.TFloat, l.argFloat(n, 0))
	case "ceil":
		return call("math", "math", "Ceil", ir.TFloat, l.argFloat(n, 0))
	case "fmod":
		return call("math", "math", "Mod", ir.TFloat, l.argFloat(n, 0), l.argFloat(n, 1))
	}
	return l.todoExpr(n, "P2G7540", "POSIX "+n.Name,
		"the "+n.Name+" function is not implemented",
		"POSIX::"+n.Name+" has no single Go counterpart.",
		"The time package covers the date and time functions, with layouts written "+
			"as an example timestamp rather than as percent codes.")
}

func (l *Lowerer) pathCall(n *ast.Call) ir.Expr {
	fn := "Base"
	if n.Name == "dirname" {
		fn = "Dir"
	}
	out := call("path/filepath", "filepath", fn, ir.TString, l.argStr(n, 0))
	l.note(out, "path/filepath works on the running system's separator; the path "+
		"package is the same functions for slash-separated paths such as URLs.")
	return out
}
