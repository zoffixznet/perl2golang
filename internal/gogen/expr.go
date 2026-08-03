package gogen

import (
	"strings"

	"perl2golang/internal/ir"
)

// Go's binary operator precedence, from the language specification. A higher
// number binds tighter. The emitter uses these numbers to decide where
// parentheses are genuinely required, so the output reads like hand-written Go
// (a + b*c) instead of like a parse tree ((a + (b * c))).
const (
	precLowest  = 0 // no enclosing operator
	precOr      = 1 // ||
	precAnd     = 2 // &&
	precCompare = 3 // == != < <= > >=
	precAdd     = 4 // + - | ^
	precMul     = 5 // * / % << >> & &^
	precUnary   = 6 // ! - + ^ * & <-
	precPrimary = 7 // names, literals, selectors, calls, indexes
)

// binaryPrec returns the precedence of a binary operator. An operator the
// emitter does not know about is treated as the loosest possible binding, so
// an unexpected spelling produces redundant parentheses rather than code that
// means something else.
func binaryPrec(op string) int {
	switch op {
	case "||":
		return precOr
	case "&&":
		return precAnd
	case "==", "!=", "<", "<=", ">", ">=":
		return precCompare
	case "+", "-", "|", "^":
		return precAdd
	case "*", "/", "%", "<<", ">>", "&", "&^":
		return precMul
	}
	return precLowest
}

// precedenceOf reports how tightly an expression's outermost operator binds.
func precedenceOf(x ir.Expr) int {
	switch x := x.(type) {
	case *ir.Binary:
		return binaryPrec(x.Op)
	case *ir.Unary:
		return precUnary
	case *ir.Lit:
		if x.Kind == ir.LitRaw {
			return rawPrecedence(x.Value)
		}
		if strings.HasPrefix(x.Value, "-") || strings.HasPrefix(x.Value, "+") {
			return precUnary
		}
		return precPrimary
	case *ir.RawExpr:
		return rawPrecedence(x.Source)
	}
	return precPrimary
}

// rawPrecedence guesses the binding of a hand-written Go expression by looking
// for an operator outside any bracket or quote. Verbatim source is opaque, so
// the emitter errs towards parenthesising it.
func rawPrecedence(src string) int {
	depth := 0
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '"', '\'', '`':
			if j := skipQuoted(src, i); j > i {
				i = j
			}
		case '+', '-', '*', '/', '%', '<', '>', '=', '!', '&', '|', '^':
			if depth == 0 {
				return precLowest
			}
		}
	}
	return precPrimary
}

// skipQuoted returns the index of the closing quote of the literal that starts
// at i, or i when the literal is unterminated.
func skipQuoted(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		if s[j] == '\\' && q != '`' {
			j++
			continue
		}
		if s[j] == q {
			return j
		}
	}
	return i
}

// expr renders an expression with nothing enclosing it.
func (e *Emitter) expr(x ir.Expr) string { return e.exprIn(x, precLowest, false) }

// exprHeader renders an expression for the head of an if, for, or switch,
// where a bare composite literal would be read as the start of the block.
func (e *Emitter) exprHeader(x ir.Expr) string { return e.exprIn(x, precLowest, true) }

// exprIn renders x for a context that binds at precedence min. noLit marks the
// statement-header positions where Go forbids an unparenthesised composite
// literal; it stops at the first bracket, exactly as the language does.
func (e *Emitter) exprIn(x ir.Expr, min int, noLit bool) string {
	if x == nil {
		return ""
	}
	s := e.render(x, noLit)
	if precedenceOf(x) < min {
		s = "(" + s + ")"
	}
	return e.inlineNotes(x) + s
}

// operand renders an expression in a position where Go requires one. An IR
// node that is missing its operand is a bug in the caller, and the untyped nil
// keeps the output parseable so that bug surfaces as a type error rather than
// as unreadable source.
func (e *Emitter) operand(x ir.Expr, min int, noLit bool) string {
	if s := e.exprIn(x, min, noLit); s != "" {
		return s
	}
	return "nil"
}

func (e *Emitter) render(x ir.Expr, noLit bool) string {
	switch x := x.(type) {
	case *ir.Ident:
		return x.Name

	case *ir.Lit:
		return literal(x)

	case *ir.Call:
		args := make([]string, 0, len(x.Args))
		for i, a := range x.Args {
			s := e.exprIn(a, precLowest, false)
			if x.Ellipsis && i == len(x.Args)-1 {
				s += "..."
			}
			args = append(args, s)
		}
		fun := e.operand(x.Fun, precPrimary, noLit)
		if len(x.TypeArgs) > 0 {
			names := make([]string, 0, len(x.TypeArgs))
			for _, t := range x.TypeArgs {
				names = append(names, e.typ(t))
			}
			fun += "[" + strings.Join(names, ", ") + "]"
		}
		return fun + "(" + strings.Join(args, ", ") + ")"

	case *ir.Selector:
		if x.Import != "" {
			// The import set owns the package identifier, because a second
			// package with the same base name has to be aliased.
			return e.imports.Add(x.Import) + "." + x.Sel
		}
		sel := x.Sel
		if sel == "" {
			sel = "_"
		}
		return e.operand(x.X, precPrimary, noLit) + "." + sel

	case *ir.Index:
		return e.operand(x.X, precPrimary, noLit) + "[" + e.operand(x.Index, precLowest, false) + "]"

	case *ir.IndexComma:
		return e.operand(x.X, precPrimary, noLit) + "[" + e.operand(x.Index, precLowest, false) + "]"

	case *ir.SliceExpr:
		return e.operand(x.X, precPrimary, noLit) + "[" +
			e.exprIn(x.Low, precLowest, false) + ":" +
			e.exprIn(x.High, precLowest, false) + "]"

	case *ir.Binary:
		p := binaryPrec(x.Op)
		// Go's binary operators are left associative, so only the right
		// operand needs parentheses at equal precedence.
		return e.operand(x.L, p, noLit) + " " + x.Op + " " + e.operand(x.R, p+1, noLit)

	case *ir.Unary:
		operand := e.operand(x.X, precUnary, noLit)
		if joinsIntoOperator(x.Op, operand) {
			return x.Op + " " + operand
		}
		return x.Op + operand

	case *ir.Paren:
		return "(" + e.operand(x.X, precLowest, false) + ")"

	case *ir.CompositeLit:
		return e.compositeLit(x, noLit)

	case *ir.FuncLit:
		return e.funcLit(x)

	case *ir.TypeAssert:
		t := e.typ(x.Assert)
		if t == "" {
			t = "any"
		}
		return e.operand(x.X, precPrimary, noLit) + ".(" + t + ")"

	case *ir.Conversion:
		t := e.typ(x.To)
		if t == "" {
			t = "any"
		}
		if typeNeedsParens(t) {
			t = "(" + t + ")"
		}
		return t + "(" + e.operand(x.X, precLowest, false) + ")"

	case *ir.RawExpr:
		// A raw expression spells its own Go, and that Go can name a package.
		// Rendering its type registers the import the same way every other
		// expression does; the rendered text itself is not used.
		if x.T != nil {
			_ = x.T.Go(func(path string) { e.imports.Add(path) })
		}
		return x.Source
	}
	// An expression kind the emitter does not know is still rendered as
	// something that parses, so one gap never cascades into broken source.
	return "nil"
}

// literal renders a literal, filling in a spelling when the IR carries none.
func literal(l *ir.Lit) string {
	if l.Value != "" {
		return l.Value
	}
	switch l.Kind {
	case ir.LitString:
		return `""`
	case ir.LitInt, ir.LitFloat:
		return "0"
	case ir.LitBool:
		return "false"
	}
	return "nil"
}

// joinsIntoOperator reports whether writing a prefix operator directly against
// its operand would lex as a different, longer operator: -(-x) must be "- -x"
// and not "--x".
func joinsIntoOperator(op, operand string) bool {
	if op == "" || operand == "" {
		return false
	}
	last := op[len(op)-1]
	if last != operand[0] {
		return false
	}
	switch last {
	case '-', '+', '&', '<':
		return true
	}
	return false
}

// typeNeedsParens reports whether a conversion's target type has to be
// parenthesised: *T(x) parses as *(T(x)), so a pointer or function type needs
// (*T)(x).
func typeNeedsParens(t string) bool {
	return strings.HasPrefix(t, "*") || strings.HasPrefix(t, "func") || strings.HasPrefix(t, "<-")
}

func (e *Emitter) compositeLit(x *ir.CompositeLit, noLit bool) string {
	head := ""
	if x.LitType != nil {
		head = e.typ(x.LitType)
	}
	parts := make([]string, 0, len(x.Elems))
	for i, el := range x.Elems {
		s := e.exprIn(el, precLowest, false)
		if i < len(x.Keys) && x.Keys[i] != nil {
			s = e.exprIn(x.Keys[i], precLowest, false) + ": " + s
		}
		parts = append(parts, s)
	}

	var out string
	if multiline(parts) {
		pad := strings.Repeat("\t", e.indent+1)
		var sb strings.Builder
		sb.WriteString(head + "{\n")
		for _, p := range parts {
			sb.WriteString(pad + p + ",\n")
		}
		sb.WriteString(strings.Repeat("\t", e.indent) + "}")
		out = sb.String()
	} else {
		out = head + "{" + strings.Join(parts, ", ") + "}"
	}

	if noLit && head != "" {
		return "(" + out + ")"
	}
	return out
}

// multiline reports whether any element already spans lines, in which case the
// literal is written one element per line so the result stays readable.
func multiline(parts []string) bool {
	for _, p := range parts {
		if strings.Contains(p, "\n") {
			return true
		}
	}
	return false
}

func (e *Emitter) funcLit(x *ir.FuncLit) string {
	body := &Emitter{mode: e.mode, imports: e.imports, indent: e.indent + 1, atLineStart: true, saidBefore: e.saidBefore}
	if x.Body != nil {
		body.prologue(x.Body)
		for _, s := range x.Body.Stmts {
			body.stmt(s)
		}
	}
	return "func(" + e.params(x.Params) + ")" + e.results(x.Results, nil) + " {\n" +
		body.sb.String() + strings.Repeat("\t", e.indent) + "}"
}

// params renders a parameter list.
func (e *Emitter) params(ps []ir.Param) string {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, e.param(p))
	}
	return strings.Join(parts, ", ")
}

func (e *Emitter) param(p ir.Param) string {
	t := e.typ(p.Type)
	if t == "" {
		t = "any"
	}
	if p.Variadic {
		t = "..." + t
	}
	if p.Name == "" {
		return t
	}
	return p.Name + " " + t
}

// results renders a result list, including the leading space, or an empty
// string for a function that returns nothing.
func (e *Emitter) results(rs []*ir.Type, names []string) string {
	types := make([]string, 0, len(rs))
	for _, r := range rs {
		if s := e.typ(r); s != "" {
			types = append(types, s)
		}
	}
	if len(types) == 0 {
		return ""
	}
	named := len(names) == len(types)
	if len(types) == 1 && !named {
		return " " + types[0]
	}
	parts := make([]string, len(types))
	for i, t := range types {
		if named && names[i] != "" {
			parts[i] = names[i] + " " + t
			continue
		}
		parts[i] = t
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// inlineNotes renders a node's annotations as block comments, for the
// positions where a line comment would swallow the rest of the construct: an
// expression operand, or the header of an else-if. It returns the empty string
// whenever there is nothing to say.
//
// TODOs are not among them. They are written above the statement instead, by
// prologue, because an explanation in the middle of an expression pushes the
// code it belongs to off the edge of the screen. The positions prologue cannot
// reach ask for them explicitly, through inlineTodos.
func (e *Emitter) inlineNotes(n ir.Annotated) string {
	m := metaOf(n)
	if m == nil {
		return ""
	}
	var parts []string
	if e.mode == Annotated {
		// Provenance is deliberately not repeated inside an expression. The
		// statement above it already quotes the source, and a `Perl: "Ada"`
		// against every operand buries the explanation it sits next to.
		for _, note := range m.Notes {
			if e.firstMention(note.Text) {
				parts = appendComment(parts, note.Text)
			}
		}
	}
	if m.Todo != nil && e.fragment {
		if e.mode == Annotated {
			parts = appendComment(parts, "TODO: "+todoMessage(*m.Todo))
		} else {
			parts = appendComment(parts, "TODO: "+todoShort(*m.Todo))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ") + " "
}

// inlineTodos renders a node's TODOs as block comments, for the two headers a
// line comment cannot be written above: an else-if, and a case clause. The
// short wording is used in both renderings, because the full explanation on one
// line is what made these unreadable in the first place, and it is in the
// report either way.
func (e *Emitter) inlineTodos(n ir.Annotated) string {
	var todos []ir.Todo
	if m := metaOf(n); m != nil && m.Todo != nil {
		todos = append(todos, *m.Todo)
	}
	return renderTodos(append(todos, hoistedTodos(n)...))
}

// inlineExprTodos is inlineTodos for a bare expression list, which is what a
// case clause is.
func inlineExprTodos(xs []ir.Expr) string {
	var todos []ir.Todo
	for _, x := range xs {
		walkExprTodos(x, &todos)
	}
	return renderTodos(todos)
}

// renderTodos folds a list of TODOs into one run of block comments, saying each
// distinct one once.
func renderTodos(todos []ir.Todo) string {
	said := map[string]bool{}
	var parts []string
	for _, t := range todos {
		short := todoShort(t)
		if said[short] {
			continue
		}
		said[short] = true
		parts = appendComment(parts, "TODO: "+short)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ") + " "
}

func appendComment(parts []string, text string) []string {
	if c := blockComment(text); c != "" {
		return append(parts, c)
	}
	return parts
}

// blockComment folds text onto one line and wraps it in /* */, escaping any
// embedded terminator so the comment cannot end early.
func blockComment(text string) string {
	t := strings.Join(strings.Fields(strings.ReplaceAll(text, "*/", "* /")), " ")
	if t == "" {
		return ""
	}
	return "/* " + t + " */"
}

// todoMessage is the full wording of a Todo, used wherever the annotated
// rendering has room for it.
func todoMessage(t ir.Todo) string {
	text := t.Message
	if text == "" {
		text = todoShort(t)
	}
	if t.Code != "" {
		text = t.Code + ": " + text
	}
	return text
}

// todoShort is the wording used in the clean program, which describes the
// missing behaviour in the reader's own terms and never mentions where the
// code came from.
func todoShort(t ir.Todo) string {
	if t.Short != "" {
		return t.Short
	}
	return "not implemented"
}
