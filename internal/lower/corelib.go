package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file covers the core modules whose class methods are really plain
// functions: `File::Spec->catfile` names a package only so that the module
// can be subclassed per operating system, and nothing about the call needs an
// object. Go's answer to all of them is one package of ordinary functions, so
// the arrow disappears and `path/filepath` takes over.
//
// The dispatch lives here rather than beside the user-declared classes
// because these are not classes the file declared, and because the same
// mapping has to answer whether the module was reached through its class
// interface or through the functions it exports.

// coreClassCall lowers `Module->method(...)` for the core modules the
// converter knows, and reports whether it recognised one.
func (l *Lowerer) coreClassCall(n *ast.MethodCall, module, method string) (ir.Expr, bool) {
	if x, ok := l.formatConstructor(n, module, method); ok {
		return x, true
	}
	switch module {
	case "File::Spec", "File::Spec::Unix", "File::Spec::Functions":
		return l.fileSpecCall(n, method, n.Args)
	}
	return nil, false
}

// fileSpecCall maps one File::Spec class method onto path/filepath.
//
// The one difference worth carrying into the report is that Join cleans as it
// goes: empty components vanish and `..` is resolved textually, where catfile
// pastes the pieces together and leaves the result alone.
func (l *Lowerer) fileSpecCall(n ast.Node, method string, argNodes []ast.Expr) (ir.Expr, bool) {
	args, _ := l.listParts(argNodes)
	str := func(i int) ir.Expr {
		if i < len(args) {
			return l.toStr(args[i], n)
		}
		return ir.Str(`""`)
	}

	switch method {
	case "catfile", "catdir", "join", "catpath":
		// catdir keeps a `..` where it was written; filepath.Join resolves it
		// away as it builds. The two only differ when a component is dot-dot,
		// so the updir idiom takes the faithful join and everything else takes
		// the stdlib one.
		if hasUpdirArg(args) {
			joined := call("strings", "strings", "Join", ir.TString,
				composite(ir.SliceOf(ir.TString), nil, strParts(l, args, n)), ir.Str(`"/"`))
			out := l.helperCall(hCanonPath, ir.TString, joined)
			l.note(out, "filepath.Join would also resolve the `..` here, textually, "+
				"which changes where the path points when the directory above it is a "+
				"symbolic link. catdir keeps the `..` as written, so the components are "+
				"joined plainly and tidied without touching it.",
				"filepath-and-paths")
			return out, true
		}
		out := l.joinPath(args, n)
		l.note(out, "filepath.Join is both catfile and catdir, and it cleans the "+
			"result as it builds it: an empty component disappears and `..` is "+
			"resolved textually. It also uses the running system's separator, which "+
			"is the whole reason the module existed.",
			"filepath-and-paths")
		return out, true

	case "canonpath":
		out := l.helperCall(hCanonPath, ir.TString, str(0))
		l.note(out, "The near-miss here is filepath.Clean, which also resolves "+
			"`a/../b` to `b`. canonpath deliberately does not: `a` could be a "+
			"symbolic link, and then the two paths name different places. The "+
			"helper does the safe cleanup, doubled slashes and dots, and leaves "+
			"`..` where it was written; filepath.EvalSymlinks is the version that "+
			"asks the filesystem.",
			"filepath-and-paths")
		return out, true

	case "file_name_is_absolute":
		out := call("path/filepath", "filepath", "IsAbs", ir.TBool, str(0))
		return out, true

	case "rel2abs":
		base := ir.Expr(ir.Str(`""`))
		if len(args) > 1 {
			base = str(1)
		}
		out := l.helperCall(hAbsFrom, ir.TString, str(0), base)
		l.note(out, "filepath.Abs is this without the base: it joins a relative path "+
			"onto the working directory and cleans the result. It returns an error as "+
			"well, because reading the working directory can fail. None of it follows "+
			"symbolic links, which filepath.EvalSymlinks does and which is the version "+
			"worth having before deciding a path is inside a directory you control.",
			"filepath-and-paths", "errors-are-values")
		return out, true

	case "abs2rel":
		var base ir.Expr = l.helperCall(hWorkingDir, ir.TString)
		if len(args) > 1 {
			base = str(1)
		}
		out := l.helperCall(hRelPath, ir.TString, str(0), base)
		l.note(out, "filepath.Rel is the other direction from Join, and it reports an "+
			"error when no relative path exists at all, which is what happens when "+
			"one path is absolute and the other is not.",
			"filepath-and-paths", "errors-are-values")
		return out, true

	case "splitpath":
		out := l.helperCall(hSplitPath, ir.SliceOf(ir.TString), str(0))
		l.note(out, "filepath.Split returns the directory and the final component as "+
			"two values, which is the pair almost every caller wants. The volume is a "+
			"separate question, answered by filepath.VolumeName, and it is empty on "+
			"every system but Windows.",
			"filepath-and-paths", "multiple-return-values")
		return out, true

	case "splitdir":
		out := l.helperCall(hPathParts, ir.SliceOf(ir.TString), str(0))
		l.note(out, "Splitting a path into components is strings.Split on the "+
			"separator. It is deliberately not filepath.SplitList, which splits a "+
			"search path such as PATH on the colon.",
			"filepath-and-paths", "strings-package")
		return out, true

	case "curdir":
		return ir.Str(`"."`), true
	case "updir":
		return ir.Str(`".."`), true
	case "rootdir":
		out := ir.CallOf(ir.NewIdent("string", nil), ir.TString,
			ir.Pkg("path/filepath", "filepath", "Separator", nil))
		return out, true
	case "devnull":
		return ir.Pkg("os", "os", "DevNull", ir.TString), true
	case "tmpdir":
		out := call("os", "os", "TempDir", ir.TString)
		l.note(out, "os.TempDir reads TMPDIR and falls back to /tmp, the same rule, "+
			"and never fails: it returns a name whether or not the directory is "+
			"there.")
		return out, true
	}
	return nil, false
}

// joinPath builds filepath.Join over a list of components, spreading a single
// slice argument rather than passing it as one component.
func (l *Lowerer) joinPath(args []ir.Expr, n ast.Node) ir.Expr {
	spread := false
	for _, a := range args {
		if t := a.Type(); t != nil && t.Kind == ir.Slice {
			spread = true
		}
	}
	if !spread {
		parts := make([]ir.Expr, len(args))
		for i, a := range args {
			parts[i] = l.toStr(a, n)
		}
		return call("path/filepath", "filepath", "Join", ir.TString, parts...)
	}
	// One of the components is itself a list of components, which Perl
	// flattened into the argument list. Go passes a slice with ... and only
	// as the whole argument list, so the pieces are gathered first.
	parts := make([]ir.Expr, len(args))
	for i, a := range args {
		if t := a.Type(); t != nil && t.Kind == ir.Slice {
			parts[i] = l.strSlice(a, n)
			continue
		}
		parts[i] = l.toStr(a, n)
	}
	out := call("path/filepath", "filepath", "Join", ir.TString, l.listValue(parts, ir.TString))
	out.Ellipsis = true
	return out
}

// strSlice renders a list expression as []string, which is what a spread
// argument to filepath.Join has to be.
func (l *Lowerer) strSlice(x ir.Expr, n ast.Node) ir.Expr {
	t := x.Type()
	if t != nil && t.Kind == ir.Slice && t.Elem != nil && t.Elem.Kind == ir.String {
		return x
	}
	return l.helperCall(hStrList, ir.SliceOf(ir.TString), x)
}

// cwdCall lowers the functions Cwd exports, which report where the process is
// and resolve a path against it.
func (l *Lowerer) cwdCall(n *ast.Call) ir.Expr {
	switch n.Name {
	case "getcwd", "cwd", "fastcwd", "fastgetcwd":
		out := l.helperCall(hWorkingDir, ir.TString)
		l.note(out, "os.Getwd returns an error alongside the directory, because the "+
			"directory a process is sitting in can be removed while it runs. Nothing "+
			"in the standard library hands back a bare string for it.",
			"errors-are-values", "filepath-and-paths")
		return out
	}
	out := l.helperCall(hAbsPath, ir.TString, l.argStr(n, 0))
	l.approximate(n, "P2G7568", n.Name,
		"a path that is not there comes back empty, not undefined",
		"abs_path returns undef when the path does not exist, and there is no "+
			"undef here: the result is a string, and the answer for a missing path "+
			"is the empty one.",
		"Compare the result against \"\" where the difference matters, or call "+
			"os.Stat first and handle the error, which is the Go shape for the same "+
			"question.",
		"nil-vs-undef", "filepath-and-paths")
	return out
}

// dumperCall lowers Data::Dumper::Dumper, which prints a structure for a
// person to read.
//
// What it prints is Perl source, and there is no Go counterpart for that: the
// nearest thing is the %#v verb, which prints Go syntax. The shape of the
// output is therefore different, and the report says so rather than pretending
// otherwise, because a program that compares two dumps still works and a
// program that parses one does not.
func (l *Lowerer) dumperCall(n *ast.Call) ir.Expr {
	args, _ := l.listParts(n.Args)
	anys := make([]ir.Expr, len(args))
	for i, a := range args {
		anys[i] = l.assignable(a, ir.TAny, n)
	}
	out := l.helperCall(hDumpValues, ir.TString, anys...)
	l.approximate(n, "P2G7569", "Dumper",
		"the dump is Go syntax, not Perl syntax",
		"Dumper writes a structure as source that can be read back with eval. "+
			"The %#v verb writes the same structure as Go syntax, which is as close "+
			"as the standard library gets, and nothing reads it back.",
		"Where the dump is only there to be looked at, this is the same tool. "+
			"Where something parses it, use encoding/json: MarshalIndent produces a "+
			"format both sides agree on.",
		"fmt-and-verbs", "encoding-json")
	l.note(out, "%#v sorts map keys before printing them, which fmt has done since "+
		"Go 1.12, so two dumps of equal maps are equal strings. Ranging over the map "+
		"yourself would not give you that.",
		"fmt-and-verbs", "map-iteration-order")
	return out
}

// fileparseCall lowers File::Basename::fileparse, which is basename and
// dirname together plus a rule for which extension counts as the suffix.
//
// quote says whether a suffix given as text is a literal. It is for basename,
// which quotes what it is handed, and it is not for fileparse, which takes
// every suffix as a pattern.
func (l *Lowerer) fileparseCall(n *ast.Call, path ir.Expr, quote bool) ir.Expr {
	args := flatten(argList(n))
	call := []ir.Expr{path}
	for i := 1; i < len(args); i++ {
		call = append(call, l.suffixPattern(args[i], quote))
	}
	out := l.helperCall(hParseFilename, ir.SliceOf(ir.TString), call...)
	l.note(out, "filepath.Ext takes everything after the last dot, so it calls `.1` "+
		"the extension of `access.log.1`. A list of acceptable suffixes has no "+
		"counterpart, and matching a pattern against the end of the name is how "+
		"you say which one you meant.",
		"filepath-and-paths", "regexp-is-re2")
	return out
}

// suffixPattern renders one fileparse suffix argument as a *regexp.Regexp.
func (l *Lowerer) suffixPattern(e ast.Expr, quote bool) ir.Expr {
	if q, ok := e.(*ast.QrExpr); ok {
		if p, ok := l.patternOf(q); ok {
			return p
		}
	}
	x := l.expr(e)
	if x != nil {
		if t := x.Type(); t != nil && t.Name == "*regexp.Regexp" {
			return x
		}
	}
	text := l.toStr(x, e)
	if quote {
		text = call("regexp", "regexp", "QuoteMeta", ir.TString, text)
	}
	return call("regexp", "regexp", "MustCompile",
		ir.NamedType("*regexp.Regexp", "regexp"), text)
}

// abbrevCall lowers Text::Abbrev's abbrev, which maps every unambiguous
// prefix of its words to the word it identifies.
func (l *Lowerer) abbrevCall(n *ast.Call) ir.Expr {
	src := l.list(argList(n))
	out := l.helperCall(hAbbrevTable, ir.MapOf(ir.TString), l.strSlice(src, n))
	out.Ellipsis = true
	l.note(out, "abbrev hands back a hash of every prefix that identifies exactly "+
		"one word, with each whole word mapping to itself. The helper is the same "+
		"two passes Text::Abbrev makes: count the owners of every prefix, then "+
		"keep the prefixes owned by one word.",
		"maps-of-slices")
	return out
}

// hasUpdirArg reports whether one of a path join's components is a literal
// "..", which is the one component filepath.Join would rewrite away.
func hasUpdirArg(args []ir.Expr) bool {
	for _, a := range args {
		if lit, ok := a.(*ir.Lit); ok && lit.Kind == ir.LitString && lit.Value == `".."` {
			return true
		}
	}
	return false
}

// strParts renders each join component as a string expression.
func strParts(l *Lowerer, args []ir.Expr, n ast.Node) []ir.Expr {
	parts := make([]ir.Expr, len(args))
	for i, a := range args {
		parts[i] = l.toStr(a, n)
	}
	return parts
}
