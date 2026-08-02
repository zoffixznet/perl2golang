package lower

import (
	"strings"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// This file translates an option block.
//
// Getopt::Long is driven by a mini-language of specification strings, and the
// flag package is driven by one call per option with the type and the default
// written out. So this is not a matter of calling the equivalent function: the
// whole block is rewritten, one registration per name, with the destinations
// staying ordinary Go variables.
//
// The single most damaging difference is that flag.Parse stops at the first
// argument that is not an option, where Getopt::Long sorts the two apart
// wherever they appear. A converted script invoked as `prog input.txt
// --verbose` would lose --verbose entirely, with no error on either stream, so
// every emitted block routes its arguments through permuteArgs first.

// optionSpec is one specification string taken apart.
type optionSpec struct {
	// Names are the option's spellings, the primary one first.
	Names []string
	// Kind is 's', 'i', 'f' or 'o' for an option that takes a value, and 0
	// for one that does not.
	Kind byte
	// Optional marks the ':' forms, whose value may be left off.
	Optional bool
	// Negate marks '!', which also registers --no-NAME.
	Negate bool
	// Count marks '+', which counts occurrences.
	Count bool
	// Repeat is '@' for a list destination, '%' for a keyed one, 0 for one
	// value.
	Repeat byte
	// Repetition records a `{n,m}` count, which flag cannot express at all.
	Repetition string
	Raw        string
}

// primary is the name the options hash stores an option under.
func (s optionSpec) primary() string {
	if len(s.Names) == 0 {
		return ""
	}
	return s.Names[0]
}

// destType is the Go type the option's destination wants.
func (s optionSpec) destType() *ir.Type {
	base := ir.TString
	switch s.Kind {
	case 'i', 'o':
		base = ir.TInt
	case 'f':
		base = ir.TFloat
	case 0:
		base = ir.TBool
	}
	if s.Count {
		base = ir.TInt
	}
	switch s.Repeat {
	case '@':
		return ir.SliceOf(base)
	case '%':
		return ir.MapOf(base)
	}
	return base
}

// parseOptionSpec takes apart one Getopt::Long specification string.
func parseOptionSpec(raw string) (optionSpec, bool) {
	s := optionSpec{Raw: raw}
	body := raw

	// The type suffix, if any, starts at the first character that cannot be
	// part of a name.
	i := strings.IndexAny(body, "=:!+")
	names := body
	if i >= 0 {
		names = body[:i]
		body = body[i:]
	} else {
		body = ""
	}
	for _, n := range strings.Split(names, "|") {
		if n = strings.TrimSpace(n); n != "" {
			s.Names = append(s.Names, n)
		}
	}
	if len(s.Names) == 0 {
		return s, false
	}

	switch {
	case body == "":
		return s, true
	case body == "!":
		s.Negate = true
		return s, true
	case body == "+":
		s.Count = true
		return s, true
	case body[0] == '=' || body[0] == ':':
		s.Optional = body[0] == ':'
		rest := body[1:]
		if rest == "" {
			return s, false
		}
		if s.Optional && (rest == "+" || isDigits(rest)) {
			// `:+` increments and `:5` defaults to a number; both leave the
			// destination a whole number.
			s.Kind = 'i'
			s.Count = rest == "+"
			return s, true
		}
		switch rest[0] {
		case 's', 'i', 'f', 'o':
			s.Kind = rest[0]
		default:
			return s, false
		}
		rest = rest[1:]
		if rest != "" && (rest[0] == '@' || rest[0] == '%') {
			s.Repeat = rest[0]
			rest = rest[1:]
		}
		if strings.HasPrefix(rest, "{") {
			s.Repetition = rest
			rest = ""
		}
		return s, rest == ""
	}
	return s, false
}

// ---------------------------------------------------------------------------
// The options hash

// collectOptions finds the hashes an option block fills in and gives each one
// a struct type built from the specification strings.
//
// `GetOptions(\%opt, 'warn=i', 'crit=i', 'skip-pseudo!')` names every key the
// hash will ever hold and says what type each one is, which is exactly what a
// Go struct declaration is. Leaving it a map would mean one value type for a
// hash whose values are a number, a number and a flag, and that value type
// would have to be `any`.
func (l *Lowerer) collectOptions() {
	var stmts []ast.Stmt
	for _, f := range l.files {
		stmts = append(stmts, f.Prog.Stmts...)
	}
	walkExprs(stmts, func(e ast.Expr) {
		c, ok := e.(*ast.Call)
		if !ok || !isGetOptions(c.Name) {
			return
		}
		args := flatten(argList(c))
		if c.Name == "GetOptionsFromArray" && len(args) > 0 {
			args = args[1:]
		}
		if len(args) == 0 {
			return
		}
		v, ok := hashRefVar(args[0])
		if !ok {
			return
		}
		key := varKey('%', v.Name)
		cls, seen := l.optionHash[key]
		if !seen {
			cls = &Class{
				Perl:    v.Name,
				Go:      l.names.take(exportedName(v.Name)),
				fieldBy: map[string]*ClassField{},
				subBy:   map[string]*Sub{},
				IsType:  true,
				Line:    posLine(v),
				Options: true,
			}
			cls.Value = ir.NamedType(cls.Go, "")
			cls.Ptr = ir.PointerTo(cls.Value)
			l.byGoType[cls.Go] = cls
			l.classes["\x00options\x00"+v.Name] = cls
			l.classOrd = append(l.classOrd, "\x00options\x00"+v.Name)
			l.optionHash[key] = cls
		}
		for _, a := range args[1:] {
			text, ok := staticString(a)
			if !ok {
				continue
			}
			spec, ok := parseOptionSpec(text)
			if !ok {
				continue
			}
			f := l.declareField(cls, spec.primary(), c)
			f.Type = spec.destType()
			f.Fixed = true
		}
	})
}

// isGetOptions reports whether a name is one of the option-parsing entry
// points, qualified or not.
func isGetOptions(name string) bool {
	switch strings.TrimPrefix(name, "Getopt::Long::") {
	case "GetOptions", "GetOptionsFromArray":
		return true
	}
	return false
}

// hashRefVar reads the hash a `\%opt` argument refers to.
func hashRefVar(e ast.Expr) (*ast.Var, bool) {
	switch n := e.(type) {
	case *ast.RefGen:
		if v, ok := n.X.(*ast.Var); ok && v.Sigil == '%' {
			return v, true
		}
	case *ast.Var:
		if n.Sigil == '%' {
			return n, true
		}
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// Lowering the call

// getOptions lowers a call to GetOptions, emitting the whole option block and
// yielding the boolean the Perl tested.
func (l *Lowerer) getOptions(n *ast.Call) (ir.Expr, bool) {
	args := flatten(argList(n))
	var source *Binding
	if n.Name == "GetOptionsFromArray" || strings.HasSuffix(n.Name, "::GetOptionsFromArray") {
		if len(args) == 0 {
			return nil, false
		}
		v, ok := arrayRefVar(args[0])
		if !ok {
			return nil, false
		}
		source = l.lookup('@', v.Name, v)
		args = args[1:]
	}
	if len(args) == 0 {
		return nil, false
	}

	var opts *Class
	if v, ok := hashRefVar(args[0]); ok {
		if c := l.optionHash[varKey('%', v.Name)]; c != nil {
			opts = c
			b := l.lookup('%', v.Name, v)
			b.Reads++
			args = args[1:]
		}
	}

	if source == nil {
		source = l.lookup('@', "args", n)
		source.Perl = "@ARGV"
		source.Type = ir.SliceOf(ir.TString)
		if source.Init == nil {
			source.Init = slicing(ir.Pkg("os", "os", "Args", ir.SliceOf(ir.TString)),
				ir.IntLit("1"), nil, ir.SliceOf(ir.TString))
			source.Doc = "args holds the command line arguments, without the program name."
		}
	}

	set := l.tmp("flags")
	setT := ir.NamedType("*flag.FlagSet", "flag")
	decl := assign(":=", []ir.Expr{ir.NewIdent(set, setT)},
		[]ir.Expr{call("flag", "flag", "NewFlagSet", setT,
			index(ir.Pkg("os", "os", "Args", ir.SliceOf(ir.TString)), ir.IntLit("0"), ir.TString),
			ir.Pkg("flag", "flag", "ContinueOnError", ir.TInt))})
	l.setProv(decl, n)
	l.note(decl, "The package-level flag.Parse exits the program itself when an option "+
		"is wrong, printing its own usage block first. A flag set built with "+
		"ContinueOnError hands the error back instead, which is what the original did: "+
		"it returned false and let the script decide.",
		"flag-package", "errors-are-values")
	l.emit(decl)

	registered := 0
	for i := 0; i < len(args); i++ {
		text, ok := staticString(args[i])
		if !ok {
			continue
		}
		spec, ok := parseOptionSpec(text)
		if !ok {
			l.approximate(n, "P2G7501", "option specification "+text,
				"this option specification was not understood",
				"The converter reads the specification strings to work out each option's "+
					"name, type and destination, and this one is not a shape it knows.",
				"Register the option by hand with the flag call that matches its type.",
				"flag-package")
			continue
		}
		var dest ir.Expr
		if opts != nil {
			f := opts.field(spec.primary())
			if f == nil {
				continue
			}
			b := l.lookup('%', opts.Perl, n)
			dest = selector(l.identFor(b), f.Go, f.Type)
		} else {
			if i+1 >= len(args) {
				break
			}
			i++
			d, ok := l.optionDest(args[i], spec, n)
			if !ok {
				continue
			}
			dest = d
		}
		l.registerOption(set, setT, spec, dest, n)
		registered++
	}
	if registered == 0 {
		return nil, false
	}

	errName := l.tmp("err")
	parse := assign(":=", []ir.Expr{ir.NewIdent(errName, ir.TError)},
		[]ir.Expr{ir.CallOf(selector(ir.NewIdent(set, setT), "Parse", nil), ir.TError,
			l.helperCall(hPermuteArgs, ir.SliceOf(ir.TString),
				ir.NewIdent(set, setT), l.identFor(source)))})
	l.setProv(parse, n)
	l.note(parse, "Perl's option parser sorts options and file names apart wherever "+
		"they appear, and flag stops at the first word that is not an option. Without "+
		"the reordering, `prog input.txt --verbose` would leave --verbose sitting among "+
		"the file names: the option never takes effect and nothing is printed on either "+
		"stream. That is the single most expensive difference between the two.",
		"flag-package")
	l.emit(parse)

	rest := assign("=", []ir.Expr{l.identFor(source)},
		[]ir.Expr{ir.CallOf(selector(ir.NewIdent(set, setT), "Args", nil), ir.SliceOf(ir.TString))})
	l.note(rest, "The Perl parser removes the options it consumed from @ARGV and leaves "+
		"the rest. Go leaves os.Args alone and reports the leftovers from the flag set, "+
		"so the array is refilled here to keep every later use of it meaning the same "+
		"thing.")
	source.Writes++
	l.emit(rest)

	l.inform(n, "P2G7505", "GetOptions",
		"The option block became one flag set: one registration per name, the "+
			"destinations left as ordinary variables, and the arguments reordered first "+
			"so that options may still follow file names. What does not survive is "+
			"abbreviation, which the flag package has none of: a caller writing "+
			"--verb for --verbose now gets an error where it used to work.",
		"flag-package", "small-stdlib-philosophy")

	out := ir.Bin("==", ir.NewIdent(errName, ir.TError), ir.Nil(ir.TError), ir.TBool)
	l.note(out, "GetOptions returns a plain true or false. Go returns an error, and "+
		"comparing it against nil is the same question asked the Go way.",
		"errors-are-values", "if-err-nil-rhythm")
	return out, true
}

// arrayRefVar reads the array a `\@list` argument refers to.
func arrayRefVar(e ast.Expr) (*ast.Var, bool) {
	switch n := e.(type) {
	case *ast.RefGen:
		if v, ok := n.X.(*ast.Var); ok && v.Sigil == '@' {
			return v, true
		}
	case *ast.Var:
		if n.Sigil == '@' {
			return n, true
		}
	}
	return nil, false
}

// optionDest resolves the destination of one `'spec' => \$var` pair.
func (l *Lowerer) optionDest(e ast.Expr, spec optionSpec, at ast.Node) (ir.Expr, bool) {
	var v *ast.Var
	switch n := e.(type) {
	case *ast.RefGen:
		vv, ok := n.X.(*ast.Var)
		if !ok {
			break
		}
		v = vv
	case *ast.Var:
		if n.Sigil == '@' || n.Sigil == '%' {
			v = n
		}
	}
	if v == nil {
		l.approximate(at, "P2G7502", "option destination for "+spec.primary(),
			"the destination is not a variable the converter can name",
			"An option's destination can be a reference to a scalar, an array or a "+
				"hash, or a subroutine called for each occurrence. This one is none of "+
				"those, so there is nothing for the flag registration to write through.",
			"Give the option a plain variable to write into and do the work afterwards, "+
				"which also puts the validation where a reader will find it.",
			"flag-package", "pointers-vs-references")
		return nil, false
	}
	b := l.lookup(v.Sigil, v.Name, v)
	b.Writes++
	want := spec.destType()
	l.optionDests[b] = want
	b.Type = want
	return l.identFor(b), true
}

// registerOption emits the flag registration for one option, once per name it
// answers to.
func (l *Lowerer) registerOption(set string, setT *ir.Type, spec optionSpec, dest ir.Expr, at ast.Node) {
	if spec.Repetition != "" {
		l.approximate(at, "P2G7503", "option "+spec.primary()+spec.Repetition,
			"an option that swallows several words is registered as a repeatable one",
			"The specification says this option takes a fixed number of words after it. "+
				"The flag package hands a value exactly one word and has already decided "+
				"where the option ends, so there is nothing to teach it.",
			"Callers have to repeat the option instead: `--pair a --pair b` in place of "+
				"`--pair a b`.",
			"flag-package")
	}
	if spec.Optional {
		l.approximate(at, "P2G7504", "option "+spec.primary()+" with an optional value",
			"a detached value no longer attaches to this option",
			"An option whose value may be left off has to be registered as a boolean "+
				"one, and the flag package will not let a boolean option take the word "+
				"after it. Only the attached form means the same thing in both.",
			"Rewrite the callers to use `--"+spec.primary()+"=VALUE`, which behaves the "+
				"same either way.",
			"flag-package")
	}
	addr := ir.Un("&", dest, ir.PointerTo(typeOrAny(dest)))
	fset := ir.NewIdent(set, setT)
	for _, name := range spec.Names {
		lit := ir.Str(quote(name))
		usage := ir.Str(`""`)
		var st ir.Stmt
		switch {
		case spec.Negate:
			st = exprStmt(l.helperCall(hNegatableBool, ir.TVoid, fset, addr, lit, usage))
		case spec.Count:
			st = exprStmt(l.helperCall(hCountOption, ir.TVoid, fset, addr, lit, usage))
		case spec.Repeat == '@':
			st = exprStmt(l.helperCall(hStringListOption, ir.TVoid, fset, addr, lit, usage))
		case spec.Repeat == '%' && spec.Kind == 'i':
			st = exprStmt(l.helperCall(hIntMapOption, ir.TVoid, fset, addr, lit, usage))
		case spec.Repeat == '%':
			st = exprStmt(l.helperCall(hStringMapOption, ir.TVoid, fset, addr, lit, usage))
		case spec.Optional:
			st = exprStmt(l.helperCall(hOptionalString, ir.TVoid, fset, addr, lit, usage))
		case spec.Kind == 'i':
			st = exprStmt(l.helperCall(hIntOption, ir.TVoid, fset, addr, lit, usage))
		case spec.Kind == 'o':
			st = exprStmt(ir.CallOf(selector(fset, "IntVar", nil), ir.TVoid, addr, lit, dest, usage))
		case spec.Kind == 'f':
			st = exprStmt(l.helperCall(hFloatOption, ir.TVoid, fset, addr, lit, usage))
		case spec.Kind == 's':
			st = exprStmt(ir.CallOf(selector(fset, "StringVar", nil), ir.TVoid, addr, lit, dest, usage))
		default:
			st = exprStmt(ir.CallOf(selector(fset, "BoolVar", nil), ir.TVoid, addr, lit, dest, usage))
		}
		l.setProv(st, at)
		l.emit(st)
	}
	l.optionAliasNote(at, spec)
}

// optionAliasNote explains what an option with several spellings becomes.
func (l *Lowerer) optionAliasNote(at ast.Node, spec optionSpec) {
	if len(spec.Names) < 2 {
		return
	}
	l.inform(at, "P2G7506", "option aliases for "+spec.primary(),
		"An option with several spellings is several registrations against one "+
			"variable. The flag package has no notion of an alias, but a second name "+
			"pointing at the same destination works and costs a line.",
		"flag-package")
}

// configureNote reports what a Getopt::Long::Configure call changes.
func (l *Lowerer) configureNote(n *ast.Call) bool {
	var modes []string
	for _, a := range flatten(argList(n)) {
		if s, ok := staticString(a); ok {
			modes = append(modes, s)
		}
	}
	if len(modes) == 0 {
		return false
	}
	for _, m := range modes {
		switch m {
		case "bundling", "gnu_getopt", "bundling_override":
			l.approximate(n, "P2G7507", "Configure "+m,
				"single-letter options can no longer be run together",
				"Bundling reads `-abc` as three options and `-n5` as an option with its "+
					"value attached. The flag package cannot be taught either, so `-vv` "+
					"becomes an unknown option rather than a count of two.",
				"Callers have to write the options out separately. Where that is not "+
					"acceptable, github.com/spf13/pflag is a drop-in replacement for flag "+
					"with GNU semantics, including bundling.",
				"flag-package", "go-mod-vs-cpan")
		case "pass_through":
			l.approximate(n, "P2G7508", "Configure pass_through",
				"an unknown option is an error rather than an operand",
				"pass_through leaves an option the parser does not recognise in @ARGV and "+
					"carries on, which is how a script that wraps another command forwards "+
					"its arguments. The flag package stops at the first unknown option.",
				"Split the argument list yourself before parsing, or use "+
					"github.com/spf13/pflag, which has an allowlist for unknown options.",
				"flag-package")
		case "require_order", "posix_default", "no_auto_abbrev", "no_ignore_case",
			"no_bundling", "no_permute":
			l.inform(n, "P2G7509", "Configure "+m,
				"This asks the option parser to behave the way Go's flag package already "+
					"does, so there is nothing to translate.",
				"flag-package")
		default:
			l.approximate(n, "P2G7509", "Configure "+m,
				"this setting has no counterpart",
				"The flag package has one behaviour and no settings, so a call that "+
					"changes how the parser reads its input has nothing to change here.",
				"Check what the setting did and whether the generated block still does "+
					"what the callers expect.",
				"flag-package", "small-stdlib-philosophy")
		}
	}
	return true
}

// hashPairs returns the key/value list a hash initialiser was written with,
// when every key is a literal.
func hashPairs(rhs ast.Expr) ([]ast.Expr, bool) {
	flat := flatten(rhs)
	if len(flat) == 0 || len(flat)%2 != 0 {
		return nil, false
	}
	for i := 0; i < len(flat); i += 2 {
		if _, ok := staticString(flat[i]); !ok {
			return nil, false
		}
	}
	return flat, true
}

// optionLit builds the struct literal a `my %opt = (...)` becomes when the
// hash is the one an option block fills in. The defaults written there are the
// struct's initial field values.
func (l *Lowerer) optionLit(c *Class, flat []ast.Expr) ir.Expr {
	var keys, vals []ir.Expr
	for i := 0; i+1 < len(flat); i += 2 {
		key, _ := staticString(flat[i])
		f := c.field(key)
		if f == nil {
			f = l.declareField(c, key, flat[i])
		}
		value := l.scalar(flat[i+1])
		if !f.Fixed {
			l.observeField(f, typeOrAny(value))
		}
		keys = append(keys, ir.NewIdent(f.Go, nil))
		vals = append(vals, l.assignable(value, f.Type, flat[i+1]))
	}
	out := composite(c.Value, keys, vals)
	l.note(out, "The defaults the hash was written with are the struct's starting "+
		"field values. Every field it does not mention starts at its type's zero value, "+
		"which is what an option nobody gave should be.",
		"static-types-and-zero-values", "flag-package")
	return out
}
