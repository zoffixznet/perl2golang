package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file handles Perl's separator variables: `$/`, `$,`, `$\` and `$"`.
//
// They are globals that change what other operations do. `$/` decides where a
// read stops, `$,` and `$\` decide what `print` puts between and after its
// arguments, and `$"` decides what an array interpolated into a string is
// joined with. Nothing in Go works like that: every call states what it does,
// and the standard library has no global anyone can reach in and change.
//
// The translation follows from that. Each of them is read while the program is
// being converted rather than while it runs, and what it says is written into
// the calls it governs: `local $/;` before a read becomes a call that reads to
// the end, and `local $, = ", "` before a print becomes a strings.Join with
// that separator in it. The value is nearly always a literal, which is what
// makes this work; where it is not, the assignment is reported and the default
// stays in force.
//
// Scoping is lexical here, which is not quite what `local` means. A sub called
// from inside the block sees the localised value in Perl and does not here.
// That difference is reported where it could matter.

// sepMode is what `$/` was last set to.
type sepMode int

const (
	// sepLine is the default: a read stops at a newline.
	sepLine sepMode = iota
	// sepSlurp is `$/ = undef`: one read takes the whole handle.
	sepSlurp
	// sepPara is `$/ = ""`: a read takes a paragraph, and runs of blank lines
	// between paragraphs collapse.
	sepPara
	// sepCustom is `$/ = "text"`: a read stops at that text.
	sepCustom
)

// separators is the state the four separator variables carry.
type separators struct {
	// rec is `$/`, the input record separator.
	rec  sepMode
	text string // the separator itself when rec is sepCustom

	// ofs is `$,`, written between print's arguments, empty by default.
	ofs string
	// ors is `$\`, written after everything print wrote, empty by default.
	ors string
	// list is `$"`, used between the elements of an interpolated array.
	list string
	// listSet distinguishes `$" = ""` from `$"` never having been touched,
	// because the default is a space and the empty string is a real choice.
	listSet bool
}

// defaultSeparators is what a program starts with, and what a block that
// localised one of them goes back to on the way out.
func defaultSeparators() separators { return separators{rec: sepLine} }

// listSep returns the text an interpolated array is joined with.
func (s separators) listSep() string {
	if s.listSet {
		return s.list
	}
	return " "
}

// separatorVar names the separator a variable is, or returns the empty string.
func separatorVar(e ast.Expr) string {
	v, ok := e.(*ast.Var)
	if !ok || v.Sigil != '$' {
		return ""
	}
	switch v.Name {
	case "/", ",", "\\", "\"":
		return v.Name
	}
	return ""
}

// setSeparator records what a separator variable was set to. It reports
// whether the assignment was one it could read; an assignment it could not
// read leaves the separator alone and is reported to the developer.
//
// It produces no statements at all. Nothing in the generated program holds
// these values: they are folded into the calls they govern.
func (l *Lowerer) setSeparator(name string, value ast.Expr, n ast.Node, scoped bool) []ir.Stmt {
	text, known := "", false
	if value == nil {
		known = true // `local $/;` and `local $,;` set the variable to undef
	} else if s, ok := staticString(value); ok {
		text, known = s, true
	} else if isUndefLiteral(value) {
		known = true
	}
	if !known {
		l.approximate(n, "P2G6018", "$"+name,
			"the separator is read while converting, and this value is not known then",
			"Perl's separator variables are read by print and by the read operator "+
				"while the program runs, so a value worked out at run time is fine there. "+
				"Go has no such variable: what the separator says is written into the "+
				"calls it governs, which means it has to be known while converting.",
			"Pass the separator to the calls that use it. strings.Join takes one "+
				"directly, and a read with a separator in it is one call with the "+
				"separator as an argument.",
			"small-stdlib-philosophy")
		return nil
	}

	switch name {
	case "/":
		switch {
		case value == nil || isUndefLiteral(value):
			l.seps.rec, l.seps.text = sepSlurp, ""
		case text == "":
			l.seps.rec, l.seps.text = sepPara, ""
		case text == "\n":
			l.seps.rec, l.seps.text = sepLine, ""
		default:
			l.seps.rec, l.seps.text = sepCustom, text
		}
	case ",":
		l.seps.ofs = text
	case "\\":
		l.seps.ors = text
	case "\"":
		l.seps.list, l.seps.listSet = text, true
	}

	l.inform(n, "P2G6019", "$"+name, separatorExplanation(name, l.seps))
	if scoped {
		l.approximate(n, "P2G6019", "local $"+name,
			"the separator applies to the reads and writes in this block",
			"`local` puts the old value back when the block ends, and a sub called "+
				"from inside the block sees the new one meanwhile. Here the value is "+
				"folded into the calls in this block, so the restore is automatic and a "+
				"sub called from inside is unaffected.",
			"Where a called sub was meant to see the change, pass the separator to it "+
				"as an argument.",
			"small-stdlib-philosophy")
	}
	return nil
}

// separatorExplanation says what the new value means for the code around it.
func separatorExplanation(name string, s separators) string {
	switch name {
	case "/":
		switch s.rec {
		case sepSlurp:
			return "Setting $/ to undef makes the next read take the whole handle in " +
				"one piece. Go says that directly: io.ReadAll reads to the end, and " +
				"there is no mode to leave set afterwards."
		case sepPara:
			return "Setting $/ to the empty string is paragraph mode: a read stops at a " +
				"run of blank lines. Go has no reading mode, so the whole input is read " +
				"and split on blank lines, which is the same result stated as what it is."
		case sepLine:
			return "Setting $/ back to a newline restores the default. Go's readers stop " +
				"at whatever byte the call names, so nothing is set back."
		default:
			return "Setting $/ to text makes a read stop at that text rather than at a " +
				"newline. Go has bufio.Reader.ReadString for a single byte and " +
				"strings.Split for anything longer, and either one names the separator " +
				"at the point of use."
		}
	case ",":
		if s.ofs == "" {
			return "Setting $, back to the empty string restores the default: print " +
				"writes its arguments with nothing between them, which is what fmt.Print " +
				"does with strings."
		}
		return "$, is written between print's arguments. Go has no such global, so the " +
			"separator becomes part of the call: strings.Join over the arguments says " +
			"the same thing and says it where it happens."
	case "\\":
		if s.ors == "" {
			return "Setting $\\ back to the empty string restores the default: print " +
				"adds nothing after its arguments."
		}
		return "$\\ is written after everything print wrote, which is how a program " +
			"gets a newline on every print without writing one. Go has no such global; " +
			"the text becomes the last argument of the call."
	default:
		if !s.listSet || s.list == " " {
			return "$\" is the text an array interpolated into a string is joined with, " +
				"and a space is the default. Go has no interpolation, so the join is " +
				"written out with strings.Join."
		}
		return "$\" is the text an array interpolated into a string is joined with. Go " +
			"has no interpolation at all, so what Perl did implicitly becomes a " +
			"strings.Join call with this separator in it."
	}
}

// isUndefLiteral reports whether an expression is the bare word undef.
func isUndefLiteral(e ast.Expr) bool {
	c, ok := e.(*ast.Call)
	return ok && c.Name == "undef" && len(c.Args) == 0
}

// separatorAssign intercepts an assignment to one of the separator variables,
// with or without `local` in front of it.
func (l *Lowerer) separatorAssign(target ast.Expr, value ast.Expr, n ast.Node, scoped bool) ([]ir.Stmt, bool) {
	name := separatorVar(target)
	if name == "" {
		return nil, false
	}
	return l.setSeparator(name, value, n, scoped), true
}
