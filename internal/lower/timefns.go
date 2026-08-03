package lower

import (
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file covers the two questions a script asks that Perl answers with a
// list of numbers and a percent-coded format string: what time is it, and
// what is this value.
//
// Go answers both differently. A moment is a time.Time and each part of it is
// a method on that value, so nothing has to remember that the month is one
// less than its name. A format is written as an example timestamp rather than
// as codes, which is checked by eye instead of by memory. And the questions
// Scalar::Util asks about a scalar are mostly answered by the compiler, with
// the ones that remain being a type switch.

// timeSplit lowers gmtime and localtime.
//
// In list context both give the nine numbers a time can be taken apart into;
// in scalar context they give a readable stamp. The two are different enough
// that Go has separate calls for them, so the context decides here.
func (l *Lowerer) timeSplit(n *ast.Call, local bool) ir.Expr {
	epoch := ir.Expr(conversion(ir.TInt, selectorCall(
		call("time", "time", "Now", ir.NamedType("time.Time", "time")), "Unix", ir.TInt)))
	if argCount(n) > 0 {
		epoch = l.argInt(n, 0)
	}
	out := l.helperCall(hTimeParts, ir.SliceOf(ir.TInt), epoch, ir.BoolLit(local))
	zone := "UTC"
	if local {
		zone = "the local zone"
	}
	l.note(out, "Perl hands back nine numbers and leaves you to remember which is "+
		"which, that the month counts from zero and that the year counts from 1900. "+
		"Go gives a time.Time whose parts are methods: t.Month() is a time.Month and "+
		"prints as its name. This reads the moment in "+zone+".",
		"time-layouts")
	l.approximate(n, "P2G7575", n.Name,
		"the nine-number split has no Go counterpart",
		"A moment in Go is a time.Time, and every part of it is a method on that "+
			"value. Nothing takes it apart into a list, because nothing needs to.",
		"Keep the time.Time and call t.Year(), t.Month() and t.Day() where the "+
			"Perl indexed the list. The month and year offsets disappear with the list.",
		"time-layouts")
	return out
}

// timeStamp lowers gmtime or localtime read for one value rather than a list,
// which is the readable form of the moment.
func (l *Lowerer) timeStamp(n *ast.Call, local bool) ir.Expr {
	out := l.helperCall(hTimeText, ir.TString, l.momentOf(n, local))
	l.note(out, "Read for a single value rather than a list, this is the readable "+
		"stamp. Go writes it with a layout, which is an example timestamp rather "+
		"than a set of percent codes: the example is always Mon Jan 2 15:04:05 MST "+
		"2006, whose fields are 1 2 3 4 5 6 7 in order.",
		"time-layouts")
	return out
}

// momentOf builds the time.Time a call is talking about.
func (l *Lowerer) momentOf(n *ast.Call, local bool) ir.Expr {
	if argCount(n) == 0 {
		if local {
			return call("time", "time", "Now", ir.NamedType("time.Time", "time"))
		}
		return selectorCall(call("time", "time", "Now", ir.NamedType("time.Time", "time")),
			"UTC", ir.NamedType("time.Time", "time"))
	}
	inner := call("time", "time", "Unix", ir.NamedType("time.Time", "time"),
		ir.CallOf(ir.NewIdent("int64", nil), ir.TInt, l.argInt(n, 0)), ir.IntLit("0"))
	if local {
		return inner
	}
	return selectorCall(inner, "UTC", ir.NamedType("time.Time", "time"))
}

// selectorCall builds x.Method().
func selectorCall(x ir.Expr, name string, t *ir.Type) ir.Expr {
	return ir.CallOf(selector(x, name, nil), t)
}

// strftimeCall lowers POSIX::strftime.
//
// Where every code in the format has a layout, the layout is written out and
// the call becomes t.Format, which is what a Go program looks like. Where one
// of them has no layout at all, the whole format goes to a helper, because a
// format half in one notation and half in the other would be worse than
// either.
func (l *Lowerer) strftimeCall(n *ast.Call) ir.Expr {
	format, ok := staticString(l.argNode(n, 0))
	moment := l.timeValue(n, 1)
	if !ok {
		out := l.helperCall(hFormatStamp, ir.TString, l.argStr(n, 0), moment)
		l.approximate(n, "P2G7576", "strftime",
			"the format is only known while the program runs",
			"Go writes a time format as an example timestamp, which is a constant in "+
				"the source. A format worked out at run time cannot be written that way.",
			"Where the set of formats is small, name each layout as a constant and "+
				"pick between them.",
			"time-layouts")
		return out
	}
	if layout, ok := goLayout(format); ok {
		out := ir.CallOf(selector(moment, "Format", nil), ir.TString, ir.Str(quote(layout)))
		l.note(out, "Go writes a time format as an example timestamp rather than as "+
			"percent codes. The example is always the same instant, Mon Jan 2 15:04:05 "+
			"MST 2006, and its fields are the numbers 1 to 7 in order, which is what "+
			"makes a layout readable without a table beside it.",
			"time-layouts")
		l.approximate(n, "P2G7576", "strftime",
			"the format became a layout, and the layout is not locale-aware",
			"`"+format+"` became the layout `"+layout+"`. Go's time package writes "+
				"month and day names in English and has no locale setting at all.",
			"For another language, format the parts yourself or use "+
				"golang.org/x/text/message, which is the package that knows about locales.",
			"time-layouts")
		return out
	}
	out := l.helperCall(hFormatStamp, ir.TString, ir.Str(quote(format)), moment)
	l.approximate(n, "P2G7576", "strftime",
		"this format has a code with no layout",
		"`"+format+"` uses a code Go's layouts have no spelling for, such as the day "+
			"of the year or the week number, so the format is kept as it is and read "+
			"by a helper.",
		"time.Time answers those directly: t.YearDay() and t.ISOWeek() are the two "+
			"that come up most.",
		"time-layouts")
	return out
}

// timeValue reads the moment argument of a time-formatting call, which Perl
// passes as the nine-number list.
func (l *Lowerer) timeValue(n *ast.Call, from int) ir.Expr {
	args := flatten(argList(n))
	if from >= len(args) {
		return selectorCall(call("time", "time", "Now", ir.NamedType("time.Time", "time")),
			"UTC", ir.NamedType("time.Time", "time"))
	}
	rest := args[from:]
	// The usual call passes the whole list at once. A call that passes the
	// nine numbers separately is the same thing written out.
	parts, _ := l.listParts(rest)
	var list ir.Expr
	if len(parts) == 1 && typeOrAny(parts[0]).Kind == ir.Slice {
		list = parts[0]
	} else {
		ints := make([]ir.Expr, len(parts))
		for i, p := range parts {
			ints[i] = l.toInt(p, n)
		}
		list = composite(ir.SliceOf(ir.TInt), nil, ints)
	}
	if t := typeOrAny(list); t.Kind == ir.Slice && t.Elem != nil && t.Elem.Kind != ir.Int {
		list = l.helperCall(hIntList, ir.SliceOf(ir.TInt), list)
	}
	return l.helperCall(hTimeFrom, ir.NamedType("time.Time", "time"), list)
}

// goLayout translates a percent-coded time format into a Go layout, reporting
// false when any code in it has no layout at all.
func goLayout(format string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			// A layout is matched by pattern, so a literal character that
			// happens to start one of the patterns would be read as a field
			// rather than kept.
			if startsLayoutToken(format, i) {
				return "", false
			}
			b.WriteByte(format[i])
			continue
		}
		i++
		if i >= len(format) {
			return "", false
		}
		layout, ok := layoutFor[format[i]]
		if !ok {
			return "", false
		}
		b.WriteString(layout)
	}
	return b.String(), true
}

// layoutFor maps the percent codes that have an exact layout onto it.
var layoutFor = map[byte]string{
	'Y': "2006", 'y': "06", 'm': "01", 'd': "02", 'e': "_2",
	'H': "15", 'I': "03", 'M': "04", 'S': "05", 'p': "PM",
	'a': "Mon", 'A': "Monday", 'b': "Jan", 'B': "January", 'h': "Jan",
	'Z': "MST", 'z': "-0700",
	'F': "2006-01-02", 'T': "15:04:05", 'D': "01/02/06", 'R': "15:04",
	'%': "%", 'n': "\n", 't': "\t",
}

// startsLayoutToken reports whether the literal character at i would be read
// as the start of a layout field rather than kept as itself.
//
// Every layout field begins with a digit or with one of a few letters, and a
// sign or a Z only counts when digits follow it, which is why the test looks
// at what comes next as well.
func startsLayoutToken(format string, i int) bool {
	c := format[i]
	next := byte(0)
	if i+1 < len(format) {
		next = format[i+1]
	}
	digit := func(b byte) bool { return b >= '0' && b <= '9' }
	switch {
	case digit(c):
		return true
	case c == 'J' || c == 'M' || c == 'P' || c == 'p':
		return true
	case c == 'Z' || c == '-' || c == '+':
		return digit(next)
	case c == '.' || c == ',':
		return next == '0' || next == '9'
	case c == '_':
		return next == '2'
	}
	return false
}

// blessedCall lowers Scalar::Util::blessed, which answers the class a value
// was blessed into, or nothing when it was never blessed.
func (l *Lowerer) blessedCall(n *ast.Call) ir.Expr {
	x := l.argExpr(n, 0)
	if c := l.classOf(typeOrAny(x)); c != nil && !c.Record {
		out := ir.Str(quote(c.Perl))
		l.note(out, "This value has one Go type, decided when the program was "+
			"compiled, so the class it belongs to is the same on every run and is "+
			"written in here.",
			"static-types-and-zero-values")
		return out
	}
	fn, ok := l.classNamePredicate()
	if !ok {
		out := ir.Str(`""`)
		l.approximate(n, "P2G7577", "blessed",
			"nothing in this file declares a class",
			"blessed reports the class a reference was blessed into, and the empty "+
				"answer for anything else. No package in this file is used as a class, so "+
				"the answer is always the empty one.",
			"Where the object comes from a module, convert that file too so its type "+
				"is in the same package.",
			"methods-and-receivers")
		return out
	}
	out := ir.CallOf(ir.NewIdent(fn, nil), ir.TString, l.assignable(x, ir.TAny, n))
	l.approximate(n, "P2G7577", "blessed",
		"the class name is looked up in a generated table",
		"A Go value carries its type and not a class name, and the two are spelled "+
			"differently. The generated function maps one to the other by switching "+
			"over the types this file declares.",
		"A type switch is the direct form of this question, and it hands back the "+
			"typed value along with the answer rather than a name to compare.",
		"type-assertions-and-switches", "methods-and-receivers")
	return out
}

// classNamePredicate declares the function that maps a value's Go type back
// to the class name the Perl knew it by.
func (l *Lowerer) classNamePredicate() (string, bool) {
	var types []*Class
	for _, name := range l.classOrd {
		c := l.classes[name]
		if c != nil && c.IsType && !c.Interface && !c.Options && !c.Record {
			types = append(types, c)
		}
	}
	if len(types) == 0 {
		return "", false
	}
	if l.pass != 2 {
		return "className", true
	}
	if l.classNameFunc != "" {
		return l.classNameFunc, true
	}
	name := l.names.take("className")
	l.classNameFunc = name

	var b strings.Builder
	b.WriteString("func " + name + "(v any) string {\n\tswitch v.(type) {\n")
	for _, c := range types {
		b.WriteString("\tcase " + c.Ptr.String() + ":\n\t\treturn " + quote(c.Perl) + "\n")
	}
	b.WriteString("\t}\n\treturn \"\"\n}")
	d := &ir.RawDecl{
		Source: b.String(),
		Doc: []string{name + " returns the name each of these types was known by, " +
			"or the empty string for a value that is not one of them."},
	}
	ir.Annotate(d, "A Go value carries its type, not a name, and the type is checked "+
		"when the program is compiled rather than looked up while it runs. Mapping "+
		"one to the other has to be written down, and a switch is where it goes.",
		"type-assertions-and-switches", "static-types-and-zero-values")
	l.isaDecls = append(l.isaDecls, d)
	return name, true
}

// reftypeCall lowers Scalar::Util::reftype, which asks what a reference is
// made of rather than what it was blessed into.
func (l *Lowerer) reftypeCall(n *ast.Call) ir.Expr {
	x := l.argExpr(n, 0)
	if c := l.classOf(typeOrAny(x)); c != nil {
		out := ir.Str(quote("HASH"))
		l.note(out, "An object here is a struct, and the hash it was built from is "+
			"what reftype was reporting. The answer is the same on every run, so it "+
			"is written in.",
			"structs-and-embedding")
		return out
	}
	out := l.helperCall(hRefKind, ir.TString, l.assignable(x, ir.TAny, n))
	l.note(out, "reftype asks what a reference is made of, ignoring the class it "+
		"was blessed into. Go answers from the value's own type, which is the same "+
		"question asked the other way round.",
		"type-assertions-and-switches")
	return out
}
