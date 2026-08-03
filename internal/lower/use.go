package lower

import (
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// useStmt handles `use` and `no`.
//
// Most pragmas describe how perl should compile the file and have no run-time
// meaning, so they simply disappear. A module, though, is a promise the
// generated program has to keep some other way, and saying which Go package
// takes its place is part of the teaching.
func (l *Lowerer) useStmt(n *ast.Use) []ir.Stmt {
	switch n.Module {
	case "":
		// `use 5.010` and friends: a version requirement.
		return nil

	case "overload":
		return l.useOverload(n)

	case "parent", "base":
		// The parent it names is already embedded in the generated struct.
		if l.pass == 2 && l.curPkg != "main" {
			l.inform(n, "P2G7552", "use "+n.Module,
				"Perl's @ISA makes method lookup walk to the parent class. The generated "+
					"type embeds the parent instead, which promotes its fields and its "+
					"methods onto this one and covers the same ground for everything except "+
					"overriding a method the parent calls on itself.",
				"structs-and-embedding", "late-binding-vs-embedding")
		}
		return nil

	case "strict", "warnings", "utf8", "feature", "vars", "lib", "integer",
		"diagnostics", "open", "bytes", "less", "sigtrap", "subs":
		if n.Module == "strict" && l.pass == 2 {
			l.inform(n, "P2G7555", "use strict",
				"`use strict` and `use warnings` ask perl to enforce at run time a "+
					"part of what Go's compiler enforces before the program starts: an "+
					"undeclared variable, a typo in a name, or a value used as the wrong "+
					"kind of thing. Go has no equivalent pragma because none of it is "+
					"optional.",
				"compile-time-mindset")
		}
		return nil

	case "constant":
		return l.useConstant(n)

	case "POSIX":
		l.inform(n, "P2G7560", "use POSIX",
			"POSIX's numeric functions come from the math package: floor, ceil and "+
				"fmod are math.Floor, math.Ceil and math.Mod. Its date formatting has no "+
				"direct counterpart, because Go writes layouts as an example timestamp "+
				"rather than as percent codes.")
		return nil

	case "List::Util":
		l.inform(n, "P2G7561", "use List::Util",
			"max and min are slices.Max and slices.Min, added to the standard "+
				"library in Go 1.21. sum, first and reduce have no counterpart on "+
				"purpose: an explicit loop is the accepted Go style, and it keeps the "+
				"accumulator's type where you can see it.",
			"small-stdlib-philosophy")
		return nil

	case "Data::Dumper":
		l.inform(n, "P2G7562", "use Data::Dumper",
			"Data::Dumper's job is done by fmt with the %#v verb for a Go-syntax "+
				"dump, or by encoding/json with MarshalIndent when the output is meant "+
				"to be read or re-parsed. Neither produces Perl source, which is what "+
				"Dumper's output really is.")
		return nil

	case "Scalar::Util":
		l.inform(n, "P2G7563", "use Scalar::Util",
			"Scalar::Util asks questions about what a scalar is holding. Go answers "+
				"most of them at compile time; the ones that remain are a type switch on "+
				"an interface value.",
			"type-assertions-and-switches")
		return nil

	case "File::Spec", "File::Spec::Unix", "File::Spec::Functions":
		l.inform(n, "P2G7566", "use "+n.Module,
			"File::Spec exists because a path separator is not the same everywhere, "+
				"and it names a class only so each system can subclass it. path/filepath "+
				"is the same idea without the object: catfile and catdir are both "+
				"filepath.Join, canonpath is filepath.Clean, and the separator comes from "+
				"the system the program was built for.",
			"filepath-and-paths")
		return nil

	case "Cwd":
		l.inform(n, "P2G7567", "use Cwd",
			"getcwd is os.Getwd, which returns an error as well, because the "+
				"directory a process is in can be deleted while it runs. abs_path is "+
				"filepath.Abs followed by filepath.EvalSymlinks: the first is textual and "+
				"succeeds for a path that is not there, the second is what consults the "+
				"disk.",
			"filepath-and-paths", "errors-are-values")
		return nil

	case "FindBin":
		l.inform(n, "P2G7589", "use FindBin",
			"FindBin works out where the script is so that `use lib` can find the "+
				"modules beside it. Go resolves every import when it compiles, so there "+
				"is no search path at run time and nothing to add to it. Where the "+
				"program genuinely needs its own location, os.Executable is the call, "+
				"and it names the built binary rather than a source file.",
			"go-mod-vs-cpan", "packages-and-exported-names")
		return nil

	case "File::Temp":
		l.inform(n, "P2G7583", "use File::Temp",
			"os.MkdirTemp and os.CreateTemp are the two constructors, and neither "+
				"cleans up by itself: `defer os.RemoveAll(dir)` is where a Go program "+
				"says when the tree goes. Inside a test, t.TempDir() does both.",
			"filepath-and-paths", "defer-timing")
		return nil

	case "File::Path":
		l.inform(n, "P2G7584", "use File::Path",
			"os.MkdirAll is make_path and os.RemoveAll is remove_tree. Both are "+
				"idempotent and both report an error rather than a list of what they "+
				"touched, so a script that printed the count has to work it out itself.",
			"filepath-and-paths", "errors-are-values")
		return nil

	case "File::Find":
		l.inform(n, "P2G7588", "use File::Find",
			"filepath.WalkDir is the counterpart, and the shape is better: the "+
				"callback is handed the path and a directory entry instead of reading "+
				"globals, returning fs.SkipDir prunes a subtree, and any other error "+
				"stops the walk and comes back out of the call. It also visits in "+
				"lexical order, so a preprocess block that sorted the names has nothing "+
				"left to do.",
			"filepath-and-paths", "errors-are-values")
		return nil

	case "File::Basename":
		l.inform(n, "P2G7564", "use File::Basename",
			"basename and dirname are filepath.Base and filepath.Dir. The filepath "+
				"package uses the running system's separator; the path package is the "+
				"same functions for slash-separated names such as URL paths.")
		return nil

	case "Getopt::Long":
		l.inform(n, "P2G7505", "use Getopt::Long",
			"The flag package covers the same ground with a different shape: each "+
				"option is declared with its type and its default, and flag.Parse fills "+
				"them in. Go's flag package stops at the first non-flag argument rather "+
				"than permuting, so `prog file --verbose` leaves --verbose in the "+
				"positional arguments.")
		return nil

	case "JSON::PP", "JSON", "JSON::XS":
		l.inform(n, "P2G7510", "use JSON",
			"encoding/json is the equivalent. It decodes into a declared struct, "+
				"which is usually what you want, or into map[string]any, where every "+
				"number arrives as a float64 whatever it looked like in the input.",
			"encoding-json", "struct-tags")
		return nil

	case "Time::HiRes", "Time::Local":
		l.inform(n, "P2G7565", "use "+n.Module,
			"The time package covers both. A duration is its own type rather than a "+
				"number of seconds, so time.Sleep(2 * time.Second) will not accept a bare "+
				"2, which is the point.")
		return nil
	}

	l.entryForModule(n)
	return nil
}

// entryForModule records a module the converter has no mapping for.
func (l *Lowerer) entryForModule(n *ast.Use) {
	// A module whose own file is part of this conversion is already here:
	// its packages became types and functions in the one Go package.
	if l.loaded[n.Module] {
		l.inform(n, "P2G7551", "use "+n.Module,
			"`"+n.Module+"` was converted along with this file. Go has no way to reach "+
				"a second file except through a second directory, so everything it "+
				"declared is in this package: its classes are types here and the names it "+
				"exported are ordinary functions. Splitting them back out is a matter of "+
				"moving the declarations into their own directory and exporting the names "+
				"the script uses.",
			"packages-and-exported-names", "go-mod-vs-cpan")
		return
	}
	if n.Module == "Exporter" || n.Module == "vars" || n.Module == "mro" {
		return
	}
	verb := "use"
	if n.No {
		verb = "no"
	}
	l.refuse(n, "P2G7550", verb+" "+n.Module,
		"the "+n.Module+" module has no mapping",
		"The converter has no Go counterpart recorded for "+n.Module+", so anything "+
			"the program does with it is left untranslated rather than guessed at.",
		"Search pkg.go.dev for what the module does. Much of what CPAN is used for "+
			"is in Go's standard library, and the parts that are not usually have one "+
			"well-known package.",
		"go-mod-vs-cpan", "small-stdlib-philosophy")
}

// useConstant lowers `use constant NAME => value` into a Go constant.
func (l *Lowerer) useConstant(n *ast.Use) []ir.Stmt {
	if len(n.Args) == 0 {
		return nil
	}
	// `use constant { A => 1, B => 2 }` declares several at once, and
	// `use constant A => 1` declares one. Both are a flat key/value list.
	pairs := flatten(n.Args[0])
	if len(pairs) == 1 {
		if h, ok := pairs[0].(*ast.AnonHash); ok {
			pairs = nil
			for _, e := range h.Elems {
				pairs = append(pairs, flatten(e)...)
			}
		}
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		l.defineConstant(n, pairs[i], pairs[i+1])
	}
	return nil
}

// defineConstant declares one name from `use constant`.
func (l *Lowerer) defineConstant(n *ast.Use, nameExpr, valueExpr ast.Expr) {
	name, ok := constantName(nameExpr)
	if !ok {
		return
	}
	// The binding is registered so that uses of the name resolve, but it is
	// deliberately kept out of the package-variable list: it is emitted as a
	// const, and declaring it twice would not compile.
	key := varKey('$', name)
	b, ok := l.globalSeen[key]
	if !ok {
		b = &Binding{Perl: key, Sigil: '$', Kind: KindGlobal, Line: posLine(n)}
		b.Go = l.names.take(goName(name))
		l.globalSeen[key] = b
	}
	value := l.scalar(valueExpr)
	l.observe(b, typeOrAny(value))
	if l.pass == 2 {
		b.Type = typeOrAny(value)
	}
	if l.constNames == nil {
		l.constNames = map[string]*Binding{}
	}
	l.constNames[name] = b

	d := &ir.VarDecl{
		Names:  []string{b.Go},
		Const:  true,
		Values: []ir.Expr{value},
		Doc:    []string{b.Go + " is a fixed value used throughout the program."},
	}
	l.note(d, "`use constant` makes a name that cannot be reassigned, and so does "+
		"Go's const. A Go constant has to be computable at compile time, which rules "+
		"out anything that calls a function; those become package-level vars instead.",
		"constants-iota-named-types")
	l.constants = append(l.constants, d)
}

// constantName reads the name from the first argument of `use constant`.
func constantName(e ast.Expr) (string, bool) {
	switch n := e.(type) {
	case *ast.StrLit:
		return n.Value, true
	case *ast.Call:
		if len(n.Args) == 0 {
			return n.Name, true
		}
	}
	return "", false
}

// useOverload turns down `use overload`.
//
// Overloading makes an operator a method call on the object, decided while the
// program runs. Go has no operator overloading of any kind: + on two values of
// a named type is either the built-in operation or a compile error, and there
// is no hook to install.
func (l *Lowerer) useOverload(n *ast.Use) []ir.Stmt {
	ops := overloadedOps(n)
	c := l.classes[l.curPkg]
	if c != nil {
		if c.Overloads == nil {
			c.Overloads = map[string]bool{}
		}
		for _, op := range ops {
			c.Overloads[op] = true
		}
	}
	where := l.curPkg
	if where == "" || where == "main" {
		where = "this package"
	}
	list := "its operators"
	if len(ops) > 0 {
		list = "`" + strings.Join(ops, "`, `") + "`"
	}
	l.refuse(n, "P2G7025", "use overload",
		"operator overloading has no Go equivalent",
		"`use overload` in "+where+" makes "+list+" method calls on the object, "+
			"chosen while the program runs from what the operands were blessed into. "+
			"Go has no operator overloading at all, and `\"\"` in particular has no "+
			"counterpart: interpolating a value calls no method the type can supply.",
		"Give the type ordinary methods and call them: Add, Mul, Equal. For `\"\"`, "+
			"implement fmt.Stringer, which is the one place Go does dispatch on a type "+
			"for formatting, and remember that it only fires for %v, %s and Print, not "+
			"for string concatenation.",
		"methods-and-receivers", "fmt-and-verbs")
	return nil
}

// overloadedOps reads the operator names out of a `use overload` list.
func overloadedOps(n *ast.Use) []string {
	var out []string
	var flat []ast.Expr
	for _, a := range n.Args {
		flat = append(flat, flatten(a)...)
	}
	for i := 0; i < len(flat); i += 2 {
		if op, ok := staticString(flat[i]); ok && op != "" {
			out = append(out, op)
		}
	}
	return out
}
