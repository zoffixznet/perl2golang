package lower

import (
	"strconv"
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// callExpr lowers a call used for its value.
func (l *Lowerer) callExpr(n *ast.Call) ir.Expr {
	if s, ok := l.findSub(n.Name); ok && !isBuiltinName(n.Name) {
		// A Go function returning several values can only be called where
		// all of them are taken at once, so anywhere else the call gets its
		// own statement first.
		if len(s.Results) > 1 {
			if x, done := l.multiResultCall(n, false); done {
				return x
			}
		}
		return l.callSub(s, n)
	}
	if x := l.builtin(n); x != nil {
		return x
	}
	if s, ok := l.findSub(n.Name); ok {
		return l.callSub(s, n)
	}
	// A name declared with `use constant` reads as a call with no arguments,
	// because that is exactly what it is in Perl.
	if b, ok := l.constNames[n.Name]; ok && argCount(n) == 0 {
		out := l.ident(b)
		l.note(out, "A Perl constant is really a sub that returns a fixed value, "+
			"which is why it is called rather than read. Go's const is a name for the "+
			"value itself, so there is nothing to call.",
			"constants-iota-named-types")
		return out
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
		"exit", "delete", "close", "open", "return", "opendir", "closedir", "local":
		return l.statementOnly(n), true
	case "splice":
		// The value is thrown away, so the call stands on its own line and
		// nothing names what it removed.
		x := l.spliceCall(n)
		if c, ok := x.(*ir.Call); ok {
			st := exprStmt(c)
			l.setProv(st, n)
			return []ir.Stmt{st}, true
		}
		return nil, false
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
	case "local":
		return l.localStmts(flatten(argList(n)), nil, n)
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
			x := l.scalar(n.Args[0])
			// A value whose type did not resolve may be holding a list, and
			// which of the two `scalar` meant cannot be decided here.
			if typeOrAny(x).Kind == ir.Any {
				out := l.helperCall(hScalarOf, ir.TAny, x)
				l.approximate(n, "P2G2031", "scalar on a dynamic value",
					"the choice is made while the program runs",
					"scalar asks for one value where a list might be, and Perl decides "+
						"which by looking at what is there. This value's type did not "+
						"resolve, so the same decision is made at run time.",
					"Give the value a concrete type where it is created, and the choice "+
						"disappears: a slice has a length and a string does not.",
					"context-is-gone", "static-types-and-zero-values")
				return out
			}
			return x
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

	case "delete":
		// delete yields the value it removed, and Go's built-in yields
		// nothing, so the value is read out before the key goes.
		if x, ok := l.deleteValue(n); ok {
			return x
		}
		for _, st := range l.statementForm(n) {
			l.emit(st)
		}
		return ir.BoolLit(true)

	case "close", "open", "print", "printf", "say", "die", "warn", "exit",
		"push", "unshift":
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
		return l.wantarray(n)
	case "ref":
		return l.refCall(n)
	case "undef":
		return l.undefCall(n)
	case "pos":
		return l.posCall(n)
	case "readdir":
		return l.readdirCall(n)
	case "unlink":
		return l.unlinkCall(n)
	case "GetOptions", "GetOptionsFromArray", "Getopt::Long::GetOptions",
		"Getopt::Long::GetOptionsFromArray":
		if x, ok := l.getOptions(n); ok {
			return x
		}
		return l.todoExpr(n, "P2G7500", n.Name,
			"this option block was not understood",
			"The converter reads an option block's specification strings to work out "+
				"each option's name, type and destination, and this one is not a shape it "+
				"could take apart.",
			"Register the options by hand with the flag package: one call per name, the "+
				"destination passed by address, and flag.NewFlagSet with ContinueOnError "+
				"so that a bad option is an error rather than an exit.",
			"flag-package")
	case "Getopt::Long::Configure", "Getopt::Long::config":
		if l.configureNote(n) {
			return ir.Nil(ir.TVoid)
		}
		return ir.Nil(ir.TVoid)
	case "__PACKAGE__":
		out := ir.Str(quote(l.curPkg))
		l.note(out, "__PACKAGE__ is the name of the package the line was compiled in, "+
			"which perl fills in when it compiles. It is known here too, so it is "+
			"written in.",
			"packages-and-exported-names")
		return out
	case "bless":
		if x, ok := l.blessCall(n); ok {
			return x
		}
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
	case "do":
		if n.Block != nil {
			return l.doBlock(n)
		}
		return l.todoExpr(n, "P2G3541", "do FILE",
			"running another file for its value is not implemented",
			"`do FILE` reads a Perl file, compiles it, and evaluates it, all while "+
				"the program runs. Go has no compiler at run time, so a second file "+
				"can only take part by being compiled into the program.",
			"Move the file's contents into this program, or, if it holds "+
				"configuration rather than code, read it with encoding/json or a "+
				"similar decoder.",
			"packages-and-exported-names")
	case "local":
		// A bare `local X` in an expression position has already been handled
		// as a statement; reaching here means it is being read for a value,
		// which it does not usefully have.
		return l.todoExpr(n, "P2G2001", "local",
			"local used for its value is not implemented",
			"local temporarily replaces a package variable's value for the duration "+
				"of the enclosing block. Its own value is the variable it localised, "+
				"which is almost never what the surrounding expression wanted.",
			"Split it into the localisation and the expression that reads the "+
				"variable.",
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
	case "sum", "sum0", "max", "min", "shuffle":
		return l.listUtil(n)
	case "maxstr", "minstr":
		return l.strExtreme(n)
	case "first":
		return l.firstCall(n)
	case "any", "all", "none":
		return l.quantifier(n)
	case "reduce":
		return l.reduceCall(n)
	case "uniq", "uniqnum":
		return l.uniqCall(n, n.Name == "uniqnum")
	case "pairs":
		return l.pairsCall(n)
	case "floor", "ceil", "fmod":
		return l.posix(n)
	case "strftime", "POSIX::strftime":
		return l.strftimeCall(n)
	case "gmtime", "localtime":
		return l.timeSplit(n, n.Name == "localtime")
	case "blessed", "Scalar::Util::blessed":
		return l.blessedCall(n)
	case "reftype", "Scalar::Util::reftype":
		return l.reftypeCall(n)
	case "looks_like_number", "Scalar::Util::looks_like_number":
		return l.helperCall(hLooksLikeNumber, ir.TBool,
			l.assignable(l.argExpr(n, 0), ir.TAny, n))
	case "setlocale", "POSIX::setlocale":
		l.inform(n, "P2G7578", "setlocale",
			"Go's time and number formatting is not locale-sensitive at all: month "+
				"and day names are always English and a decimal point is always a point. "+
				"There is no locale to set, and no risk that a program formats "+
				"differently on someone else's machine. golang.org/x/text is where the "+
				"locale-aware formatting lives when it is genuinely wanted.",
			"time-layouts", "small-stdlib-philosophy")
		return ir.Str(`"C"`)
	case "tzset", "POSIX::tzset":
		l.inform(n, "P2G7579", "tzset",
			"Go reads TZ when it first needs the local zone and caches the result, so "+
				"there is nothing to reset. time.LoadLocation names a zone explicitly, "+
				"which is what a program that must not depend on the environment does.",
			"time-layouts")
		return ir.BoolLit(true)
	case "LC_TIME", "LC_ALL", "LC_NUMERIC", "LC_CTYPE", "LC_COLLATE", "LC_MONETARY":
		if argCount(n) == 0 {
			return ir.Str(quote(n.Name))
		}
	case "basename", "dirname":
		return l.pathCall(n)
	case "md5_hex", "md5_base64", "encode_base64", "decode_base64",
		"encode_json", "decode_json", "to_json", "from_json",
		"Digest::MD5::md5_hex", "Digest::MD5::md5_base64",
		"MIME::Base64::encode_base64", "MIME::Base64::decode_base64",
		"JSON::PP::encode_json", "JSON::PP::decode_json",
		"JSON::PP::true", "JSON::PP::false", "JSON::PP::null",
		"JSON::true", "JSON::false", "JSON::null":
		if x, ok := l.formatFunction(n); ok {
			return x
		}
	case "find", "finddepth", "File::Find::find", "File::Find::finddepth":
		if x, ok := l.findCall(n); ok {
			return x
		}
	case "tempdir", "File::Temp::tempdir":
		return l.tempDirCall(n)
	case "tempfile", "File::Temp::tempfile":
		return l.tempFileCall(n)
	case "make_path", "mkpath", "File::Path::make_path", "File::Path::mkpath":
		return l.makePathCall(n)
	case "remove_tree", "rmtree", "File::Path::remove_tree", "File::Path::rmtree":
		return l.removeTreeCall(n)
	case "fileparse":
		return l.fileparseCall(n, l.argStr(n, 0), false)
	case "Dumper":
		return l.dumperCall(n)
	case "getcwd", "cwd", "fastcwd", "fastgetcwd", "abs_path", "realpath", "fast_abs_path":
		return l.cwdCall(n)
	case "catfile", "catdir", "canonpath", "file_name_is_absolute", "splitpath",
		"splitdir", "rel2abs", "abs2rel", "curdir", "updir", "rootdir", "tmpdir", "devnull":
		// File::Spec::Functions exports the class methods as plain
		// functions under the same names.
		if x, ok := l.fileSpecCall(n, n.Name, n.Args); ok {
			return x
		}
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
	dest := l.printDest(n, &args)
	if len(args) == 0 {
		args = []ast.Expr{&ast.Var{Sigil: '$', Name: "_"}}
	}

	// `$,` goes between print's arguments and `$\` goes after the last of
	// them. Perl keeps both in globals that print consults; Go has no such
	// state, so what they say is written into this call.
	ofs, ors := l.seps.ofs, l.seps.ors
	if newline {
		// say appends a newline of its own and pays no attention to `$\`.
		ors = ""
	}

	// One interpolated string is the overwhelmingly common case, and it maps
	// straight onto Printf, which is what a Go developer writes. With one
	// argument there is nothing for `$,` to go between.
	if len(args) == 1 && ofs == "" {
		if il, ok := args[0].(*ast.InterpLit); ok {
			if st, done := l.printfFromInterp(il, dest, newline, ors, n); done {
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
		if ofs != "" && len(parts) > 0 {
			parts = append(parts, ir.Str(quote(ofs)))
		}
		parts = append(parts, x)
	}
	if newline {
		parts = append(parts, ir.Str(`"\n"`))
	}
	if ors != "" {
		parts = append(parts, ir.Str(quote(ors)))
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
func (l *Lowerer) printfFromInterp(il *ast.InterpLit, dest ir.Expr, newline bool, trailer string, at ast.Node) ([]ir.Stmt, bool) {
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
	// `$\` is written after everything print wrote, so it joins the format
	// rather than becoming a global the call has to consult.
	format += trailer

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
	dest := l.printDest(n, &args)
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
		vals := l.formatValues(args[1:], format, n)
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

// printDest works out where a print, printf or say is writing.
//
// A bareword handle arrives as the first argument, because at the point it is
// parsed it is indistinguishable from one. A lexical handle, written either
// as `print $fh LIST` or as `print {EXPR} LIST`, arrives in its own field.
func (l *Lowerer) printDest(n *ast.Call, args *[]ast.Expr) ir.Expr {
	if n.Handle != nil {
		return l.writerFor(n.Handle)
	}
	if len(*args) > 0 {
		if fh, ok := (*args)[0].(*ast.FileHandle); ok {
			*args = (*args)[1:]
			return l.fileHandleExpr(fh)
		}
	}
	return nil
}

// writerFor renders the destination of a print as something Fprint accepts.
func (l *Lowerer) writerFor(e ast.Expr) ir.Expr {
	if fh, ok := e.(*ast.FileHandle); ok {
		return l.fileHandleExpr(fh)
	}
	x := l.expr(e)
	if x == nil {
		return nil
	}
	if t := typeOrAny(x); t.Kind == ir.Any {
		// The handle's type did not resolve, and Fprint takes an io.Writer,
		// so the value says what it is expected to be.
		want := ir.NamedType("io.Writer", "io")
		out := &ir.TypeAssert{X: x, Assert: want}
		out.T = want
		l.note(out, "Anything that writes bytes satisfies io.Writer, and a file is "+
			"only one of them. The assertion is here because this value's own type "+
			"did not resolve; where it did, no assertion is needed at all.",
			"io-reader-writer", "implicit-interfaces")
		return out
	}
	return x
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
	vals := l.formatValues(args[1:], format, n)
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
		// The values are a flat Perl list, so an array among them
		// contributes its elements rather than itself.
		parts, t := l.listParts(args[1:])
		list = l.listValue(parts, t)
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
	if c := l.classOf(t); c != nil && c.Record {
		return l.recordFields(n, c, m, wantValues)
	}
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

// reverseText is reverse read for one value: everything joined and then
// reversed character by character.
func (l *Lowerer) reverseText(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 1 {
		if x := l.expr(args[0]); typeOrAny(x).Kind == ir.String {
			out := l.helperCall(hReverseStr, ir.TString, x)
			l.note(out, "reverse read for one value reverses characters rather than "+
				"elements. Go has no built-in for it, partly because reversing text is "+
				"only meaningful character by character, not byte by byte.",
				"strings-are-bytes", "context-is-gone")
			return out
		}
	}
	joined := l.stringsJoin(l.list(argList(n)), ir.Str(`""`))
	out := l.helperCall(hReverseStr, ir.TString, joined)
	l.approximate(n, "P2G2031", "reverse read for one value",
		"the list is joined and then reversed",
		"reverse in list context reverses the elements; read for one value it "+
			"runs them together and reverses the characters of the result. Go has no "+
			"context, so which of the two was meant is decided here.",
		"Write the two out separately: slices.Reverse for the elements, and a "+
			"join followed by a character reversal for the text.",
		"context-is-gone", "strings-are-bytes")
	return out
}

func (l *Lowerer) reverseCall(n *ast.Call) ir.Expr {
	// reverse decides what it does from the context it is called in: a list
	// reversed where a list is wanted, and the characters of everything run
	// together reversed where one value is wanted. Only the scalar form
	// reaches reverseText.
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
	// Pushing onto something the converter only knows as `any`, which is what
	// a nested structure usually leaves behind, needs the assertion before
	// append will look at it.
	source := l.asSlice(target, args[0])
	elem := elemOf(typeOrAny(source))

	into := l.containerOf(args[0])
	var vals []ir.Expr
	var spread ir.Expr
	for _, a := range args[1:] {
		x := l.expr(a)
		if x == nil {
			continue
		}
		xt := typeOrAny(x)
		if xt.Kind == ir.Slice && elem.Kind != ir.Slice && flattensInList(a) {
			// Perl flattens an array into the push, so the array contributes
			// its elements. A reference does not: `push @a, [ ... ]` adds one
			// element that happens to be a list.
			l.observeIn(into, elemOf(xt), false)
			spread = x
			continue
		}
		l.observeIn(into, xt, false)
		vals = append(vals, l.assignable(x, elem, a))
	}

	var st ir.Stmt
	switch {
	case spread != nil && len(vals) == 0 && elemOf(typeOrAny(spread)).Equal(elem):
		c := appendTo(source, spread)
		c.Ellipsis = true
		st = assign("=", []ir.Expr{target}, []ir.Expr{c})
	case spread != nil:
		// The two element types do not match, so the values are appended one
		// at a time: Go spreads a slice only into a matching element type.
		item := l.tmp("item")
		loop := &ir.Range{
			Key:    ir.NewIdent("_", ir.TInt),
			Value:  ir.NewIdent(item, elemOf(typeOrAny(spread))),
			X:      spread,
			Define: true,
			Body: &ir.Block{Stmts: []ir.Stmt{
				assign("=", []ir.Expr{target},
					[]ir.Expr{appendTo(source, l.assignable(ir.NewIdent(item, elemOf(typeOrAny(spread))), elem, nil))}),
			}},
		}
		l.note(loop, "Go spreads a slice into append only when the element types "+
			"match. These do not, so the values go in one at a time, which is also "+
			"where the conversion between them happens.",
			"slices-not-arrays")
		if len(vals) > 0 {
			out := []ir.Stmt{loop, assign("=", []ir.Expr{target}, []ir.Expr{appendTo(source, vals...)})}
			l.setProv(loop, n)
			return out
		}
		l.setProv(loop, n)
		return []ir.Stmt{loop}
	default:
		st = assign("=", []ir.Expr{target}, []ir.Expr{appendTo(source, vals...)})
	}
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

	// Both halves are guarded together, because an empty array is the case
	// that matters and it is the case that panics. `my $file = shift @ARGV or
	// die "usage..."` runs with no arguments more often than with them, and
	// the unguarded read would end the program with a stack trace before the
	// usage message could be printed.
	decl := &ir.DeclStmt{Names: []string{name}, Type: elem}
	l.setProv(decl, n)
	take := &ir.If{
		Cond: ir.Bin(">", lenOf(target), ir.IntLit("0"), ir.TBool),
		Then: &ir.Block{Stmts: []ir.Stmt{
			assign("=", []ir.Expr{ir.NewIdent(name, elem), target}, []ir.Expr{pick, rest}),
		}},
	}
	l.setProv(take, n)
	if front {
		l.note(decl, "shift takes the first element and shortens the array. Go has no "+
			"such operation: the element is read, then the slice is re-sliced past it. "+
			"Re-slicing does not copy, it just moves the start of the window. Both "+
			"steps are guarded, because indexing an empty slice panics where Perl's "+
			"shift simply hands back undef.",
			"slices-not-arrays", "slice-aliasing-and-copy")
	} else {
		l.note(decl, "pop takes the last element and shortens the array. In Go that is "+
			"a read of the last index followed by a re-slice, guarded because both of "+
			"those panic on an empty slice where Perl's pop hands back undef.",
			"slices-not-arrays")
	}
	l.emit(decl)
	l.emit(take)
	return ir.NewIdent(name, elem)
}

func valueName(front bool) string {
	if front {
		return "first"
	}
	return "last"
}

// spliceCall lowers splice in all four of its calling forms.
//
// splice is the one Perl builtin that removes, inserts and replaces in a single
// call, and hands back what it removed on the way. Go has three separate
// functions for the three jobs and none of them returns the removed part, so
// the whole operation goes through one helper. The list is passed by pointer
// because splice changes its length, and a Go function given a slice cannot do
// that to its caller's.
func (l *Lowerer) spliceCall(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 0 {
		return l.spliceRefused(n)
	}
	target := l.assignTarget(args[0])
	if target == nil || !assignableTarget(target) {
		return l.spliceRefused(n)
	}
	t := typeOrAny(target)
	if t.Kind != ir.Slice {
		return l.spliceRefused(n)
	}
	elem := elemOf(t)

	offset := ir.Expr(ir.IntLit("0"))
	if len(args) > 1 {
		offset = l.toInt(l.expr(args[1]), args[1])
	}
	// A missing length means "to the end", and any count at least as large as
	// the list says that however the offset works out, because the helper
	// stops at the end rather than running past it.
	count := ir.Expr(lenOf(target))
	if len(args) > 2 {
		count = l.toInt(l.expr(args[2]), args[2])
	}
	repl := ir.Expr(ir.Nil(t))
	if len(args) > 3 {
		parts, _ := l.listParts(args[3:])
		// What goes in is as much evidence about what the list holds as what
		// was there already, so a replacement of a different type widens the
		// list rather than being forced into its current element type.
		if v, ok := args[0].(*ast.Var); ok && v.Sigil == '@' {
			b := l.lookup('@', v.Name, v)
			for _, p := range parts {
				l.observeElem(b, typeOrAny(p))
			}
		}
		for i, p := range parts {
			parts[i] = l.assignable(p, elem, nil)
		}
		repl = l.listValue(parts, elem)
		repl = l.assignable(repl, t, nil)
	}

	// Go has no address for a map entry, because the map may move it. A
	// splice through one goes via a variable, which is what a Go developer
	// writes for the same reason.
	var writeBack ir.Stmt
	if ix, isIndex := target.(*ir.Index); isIndex && typeOrAny(ix.X).Kind == ir.Map {
		name := l.tmp("entry")
		held := ir.NewIdent(name, t)
		decl := assign(":=", []ir.Expr{held}, []ir.Expr{target})
		l.setProv(decl, n)
		l.note(decl, "A map entry has no address in Go: the map is free to move its "+
			"contents, so nothing may point into one. The list is taken out into a "+
			"variable, changed there, and put back.",
			"pointers-vs-references", "maps-of-slices")
		l.emit(decl)
		writeBack = assign("=", []ir.Expr{target}, []ir.Expr{held})
		target = held
	}

	out := l.helperCall(hSplice, t, ir.Un("&", target, ir.PointerTo(t)), offset, count, repl)
	l.setProv(out, n)
	l.note(out, "splice does four things at once: it takes a run out of the list, "+
		"puts something else in its place, shortens or lengthens the list, and "+
		"returns what it took. Go splits those across slices.Delete, slices.Insert "+
		"and slices.Replace, none of which returns the removed part, so the whole "+
		"operation is one helper here. It takes a pointer because changing the "+
		"length of the caller's list is the one thing a function given a slice "+
		"cannot do.",
		"slice-surgery", "slices-not-arrays", "pointers-vs-references")
	l.approximate(n, "P2G5580", "splice",
		"the removed elements are a copy",
		"splice hands back the elements it removed. The generated code copies them "+
			"into a list of their own before the original is rebuilt, so the two are "+
			"independent afterwards.",
		"Where the removed part is not used, the call stands on its own and the "+
			"copy costs nothing worth measuring.",
		"slice-surgery", "slice-aliasing-and-copy")
	if writeBack == nil {
		return out
	}
	// The changed list has to go back into the map, and the removed part has
	// to be named first so that both happen in the right order.
	name := l.tmp("removed")
	removed := ir.NewIdent(name, t)
	decl := assign(":=", []ir.Expr{removed}, []ir.Expr{out})
	l.setProv(decl, n)
	l.emit(decl)
	l.setProv(writeBack, n)
	l.emit(writeBack)
	return removed
}

// spliceRefused reports the shapes of splice that have no target to work on,
// which is what a splice through something the converter could not resolve to
// a list comes to.
func (l *Lowerer) spliceRefused(n *ast.Call) ir.Expr {
	return l.todoExpr(n, "P2G5581", "splice",
		"this form of splice is not implemented",
		"splice works on a list, and the first argument here did not resolve to "+
			"one the generated code can change in place.",
		"Use slices.Delete, slices.Insert or slices.Replace on a slice variable, "+
			"which each do one of the jobs splice does and are clearer at the call "+
			"site.",
		"slice-surgery", "slices-not-arrays")
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

	// A target whose type did not resolve is not a string yet, and
	// strings.TrimSuffix will not take one, so it is read as text first.
	trimmed := ir.Expr(call("strings", "strings", "TrimSuffix", ir.TString,
		l.toStr(target, n), ir.Str(`"\n"`)))
	st := assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(trimmed, typeOrAny(target), n)})
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
	// Throwing an object is different from throwing a message: what is
	// caught is the object itself, and the code that catches it calls
	// methods on it. Rendering it as text here would lose it.
	if obj, ok := l.thrownObject(n); ok {
		return l.throwValue(obj, n)
	}
	msg := l.dieMessage(n)
	if l.traps {
		// Something in this program catches, so die has to unwind rather than
		// exit: an exit cannot be caught, and the catch is the whole point.
		st := exprStmt(ir.CallOf(ir.NewIdent("panic", nil), ir.TVoid, msg))
		l.setProv(st, n)
		l.note(st, "die throws, and this program catches somewhere, so it becomes a "+
			"panic: an os.Exit could not be caught. Go reserves panic for genuine "+
			"bugs, and a function that can fail returns an error instead, which the "+
			"caller checks on the next line.",
			"panic-and-recover", "errors-are-values", "if-err-nil-rhythm")
		return []ir.Stmt{st}
	}
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

// throwValue builds the statements that throw an object rather than a
// message.
func (l *Lowerer) throwValue(obj ir.Expr, n ast.Node) []ir.Stmt {
	// What is caught is the object, so the variable standing in for $@ has
	// to be able to hold one.
	l.throwsObject = true
	l.observe(l.errText(n), typeOrAny(obj))
	if !l.traps {
		// Nothing catches, so the object never gets read as an object: it is
		// printed and the program ends, which is what an uncaught die does.
		l.usedExit = true
		st := exprStmt(l.writeTo(ir.Pkg("os", "os", "Stderr", nil), l.toStr(obj, n)))
		l.setProv(st, n)
		return []ir.Stmt{st, exprStmt(call("os", "os", "Exit", ir.TVoid, ir.IntLit("255")))}
	}
	st := exprStmt(ir.CallOf(ir.NewIdent("panic", nil), ir.TVoid, obj))
	l.setProv(st, n)
	l.note(st, "panic carries a value of any type, not a string, so the object "+
		"survives the unwinding and whatever recovers gets it back whole. Go's own "+
		"convention is to panic with an error rather than with a bare value, so "+
		"that the recovering side has something with an Error method.",
		"panic-and-recover", "errors-are-values", "sentinel-and-custom-errors")
	return []ir.Stmt{st}
}

// thrownObject reports whether a die throws a blessed object rather than a
// message, and lowers it if so.
//
// It also records that the file throws objects at all, which is what decides
// whether the variable standing in for $@ holds text or a value.
func (l *Lowerer) thrownObject(n *ast.Call) (ir.Expr, bool) {
	args := flatten(argList(n))
	if len(args) != 1 {
		return nil, false
	}
	switch args[0].(type) {
	case *ast.StrLit, *ast.NumberLit:
		return nil, false
	}
	x := l.expr(args[0])
	if x == nil {
		return nil, false
	}
	t := typeOrAny(x)
	if l.classOf(t) == nil {
		return nil, false
	}
	return x, true
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

// deleteValue lowers a delete whose result is used, which Perl answers with
// the value that was removed.
func (l *Lowerer) deleteValue(n *ast.Call) (ir.Expr, bool) {
	args := flatten(argList(n))
	if len(args) != 1 {
		return nil, false
	}
	hi, ok := args[0].(*ast.HashIndex)
	if !ok {
		return nil, false
	}
	m, key, elem := l.hashParts(hi)
	if m == nil || key == nil {
		return nil, false
	}
	name := l.tmp("removed")
	decl := assign(":=", []ir.Expr{ir.NewIdent(name, elem)}, []ir.Expr{index(m, key, elem)})
	l.setProv(decl, n)
	l.note(decl, "Go's delete yields nothing, where Perl's yields the value it took "+
		"out. Reading the key first is what keeps that value, and reading a key a "+
		"map does not hold gives the zero value rather than an error.",
		"comma-ok-idiom")
	l.emit(decl)
	st := exprStmt(ir.CallOf(ir.NewIdent("delete", nil), ir.TVoid, m, key))
	l.setProv(st, n)
	l.emit(st)
	return ir.NewIdent(name, elem), true
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
		if !isOrdered(elem) {
			// slices.Max needs a type Go can compare with <, and this one is
			// not, so the comparison goes through the numeric reading Perl
			// would have used anyway.
			return l.numExtreme(n, src, elem)
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
	// basename takes a list of suffixes to strip, and filepath.Base has no
	// room for them: dropping them would compile and quietly keep the
	// extension. The name is the first of the three parts of the split.
	if fn == "Base" && argCount(n) > 1 {
		// Clean first, because basename ignores trailing separators and the
		// split that follows would keep them.
		clean := call("path/filepath", "filepath", "Clean", ir.TString, l.argStr(n, 0))
		out := index(l.fileparseCall(n, clean, true), ir.IntLit("0"), ir.TString)
		l.note(out, "filepath.Base has no notion of a suffix to strip, so the name "+
			"and the extension are separated first and only the name is kept. The "+
			"suffixes are quoted because basename takes them literally, where the "+
			"three-way split takes them as patterns.",
			"filepath-and-paths")
		return out
	}
	arg := l.argStr(n, 0)
	// dirname drops trailing separators before it looks for the parent, so
	// `dirname("logs/")` is "." and not "logs". filepath.Dir does not, and
	// filepath.Clean is what puts the two back in step. A literal that
	// plainly has no trailing separator needs neither.
	if fn == "Dir" && !endsWithoutSeparator(arg) {
		arg = call("path/filepath", "filepath", "Clean", ir.TString, arg)
	}
	out := call("path/filepath", "filepath", fn, ir.TString, arg)
	l.note(out, "path/filepath works on the running system's separator; the path "+
		"package is the same functions for slash-separated paths such as URLs.")
	return out
}

// endsWithoutSeparator reports whether an expression is written out in the
// source and plainly does not end in a path separator.
func endsWithoutSeparator(x ir.Expr) bool {
	lit, ok := x.(*ir.Lit)
	if !ok || lit.Kind != ir.LitString {
		return false
	}
	s, err := strconv.Unquote(lit.Value)
	return err == nil && s != "" && !strings.HasSuffix(s, "/") && !strings.HasSuffix(s, `\`)
}
