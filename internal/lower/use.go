package lower

import (
	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
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

	case "strict", "warnings", "utf8", "feature", "vars", "lib", "integer",
		"overload", "diagnostics", "open", "bytes", "less", "sigtrap", "subs":
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
