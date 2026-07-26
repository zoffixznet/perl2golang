package lower

import (
	"strings"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// perlFormat translates a Perl printf format into a Go one and coerces each
// argument to the type the Go verb needs.
//
// The two format languages look identical and are not. Perl's %s accepts a
// number and stringifies it Perl's way; Go's %s on an int produces the literal
// text %!s(int=5). Perl's %d accepts a float and truncates; Go's %d on a
// float64 is a formatting error. So every argument is converted here to match
// its verb, which is also what go vet will check afterwards.
func (l *Lowerer) perlFormat(format string, args []ir.Expr, at ast.Node) (string, []ir.Expr) {
	var out strings.Builder
	var outArgs []ir.Expr
	next := func() ir.Expr {
		if len(args) == 0 {
			return nil
		}
		a := args[0]
		args = args[1:]
		return a
	}

	i := 0
	for i < len(format) {
		c := format[i]
		if c != '%' {
			out.WriteByte(c)
			i++
			continue
		}
		spec, verb, end := scanSpec(format, i)
		if end < 0 {
			out.WriteString("%%")
			i++
			continue
		}
		if verb == '%' {
			out.WriteString("%%")
			i = end
			continue
		}

		// A * in the width or precision consumes an argument of its own.
		stars := strings.Count(spec, "*")
		for n := 0; n < stars; n++ {
			outArgs = append(outArgs, l.toInt(next(), at))
		}

		a := next()
		switch verb {
		case 'd', 'i', 'u':
			out.WriteString(strings.Replace(spec, string(verb), "d", 1))
			outArgs = append(outArgs, l.toInt(a, at))
		case 'o', 'x', 'X', 'b':
			out.WriteString(spec)
			outArgs = append(outArgs, l.toInt(a, at))
		case 'e', 'E', 'f', 'F', 'g', 'G':
			out.WriteString(spec)
			outArgs = append(outArgs, l.toFloat(a, at))
		case 's':
			out.WriteString(spec)
			outArgs = append(outArgs, l.toStr(a, at))
		case 'c':
			out.WriteString(spec)
			outArgs = append(outArgs, l.toInt(a, at))
		case 'v':
			l.approximate(at, "P2G5010", "%v version flag",
				"the %v format flag has no Go equivalent",
				"Perl's %vd prints a string as a dotted list of its character ordinals, "+
					"which is how version strings are formatted. Go's %v means something "+
					"completely different: the default format for any value.",
				"Format the parts explicitly, or use a version library.")
			out.WriteString("%s")
			outArgs = append(outArgs, l.toStr(a, at))
		default:
			out.WriteString("%v")
			outArgs = append(outArgs, a)
		}
		i = end
	}

	// Anything the format did not consume still has to go somewhere, or the
	// values would silently vanish.
	for _, a := range args {
		outArgs = append(outArgs, a)
		out.WriteString("%!(EXTRA)")
		_ = a
	}
	return out.String(), outArgs
}

// scanSpec reads one conversion specification starting at a % and returns the
// spec text, its verb, and the index just past it. It returns -1 for end when
// the text is not a conversion.
func scanSpec(s string, start int) (spec string, verb byte, end int) {
	i := start + 1
	for i < len(s) && strings.IndexByte("-+ 0#", s[i]) >= 0 {
		i++
	}
	for i < len(s) && (s[i] == '*' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && (s[i] == '*' || (s[i] >= '0' && s[i] <= '9')) {
			i++
		}
	}
	// Perl's size modifiers carry no meaning in Go and are dropped.
	drop := i
	for i < len(s) && strings.IndexByte("hlqLV", s[i]) >= 0 {
		i++
	}
	if i >= len(s) {
		return "", 0, -1
	}
	verb = s[i]
	spec = s[start:drop] + string(verb)
	return spec, verb, i + 1
}

// staticString returns the text of an expression when it is a literal string,
// which is what lets the format be translated at conversion time rather than
// at run time.
func staticString(e ast.Expr) (string, bool) {
	switch n := e.(type) {
	case *ast.StrLit:
		return n.Value, true
	case *ast.InterpLit:
		var sb strings.Builder
		for _, p := range n.Parts {
			s, ok := p.(*ast.StrLit)
			if !ok {
				return "", false
			}
			sb.WriteString(s.Value)
		}
		return sb.String(), true
	}
	return "", false
}

// stringsJoin builds strings.Join or the helper, depending on the element type.
func (l *Lowerer) stringsJoin(list ir.Expr, sep ir.Expr) ir.Expr {
	t := typeOrAny(list)
	if t.Kind == ir.Slice && t.Elem != nil && t.Elem.Kind == ir.String {
		return call("strings", "strings", "Join", ir.TString, list, sep)
	}
	return l.helperCall(hJoinList, ir.TString, list, sep)
}
