package gogen

import (
	"strconv"
	"strings"

	"perl2go/internal/ir"
)

// stmt writes one statement, preceded by its annotations. Every statement goes
// through here, which is what guarantees that the clean and annotated
// renderings are the same program: only the prologue differs.
func (e *Emitter) stmt(s ir.Stmt) {
	if s == nil {
		return
	}
	e.prologue(s)

	switch s := s.(type) {
	case *ir.Block:
		// The annotations were written above the brace by the prologue, so
		// they are not written again inside it.
		e.braces(s, false)
		e.nl()

	case *ir.BlockStmt:
		e.block(s.Body)
		e.nl()

	case *ir.Assign:
		if len(s.LHS) == 0 {
			return
		}
		op := s.Op
		if op == "" {
			op = "="
		}
		rhs := e.exprList(s.RHS)
		if rhs == "" {
			// An assignment with nothing on the right would not compile, and
			// emitting half of one would only hide the mistake.
			return
		}
		e.line(e.lhsList(s.LHS) + " " + op + " " + rhs)

	case *ir.DeclStmt:
		e.line(e.varSpec("var", s.Names, s.Type, s.Values))

	case *ir.ExprStmt:
		if s.X == nil {
			return
		}
		e.line(e.expr(s.X))

	case *ir.IncDec:
		if s.X == nil {
			return
		}
		op := "++"
		if s.Dec {
			op = "--"
		}
		e.line(e.exprIn(s.X, precPrimary, false) + op)

	case *ir.If:
		e.ifStmt(s)

	case *ir.For:
		e.forStmt(s)

	case *ir.Range:
		e.rangeStmt(s)

	case *ir.Return:
		if len(s.Results) == 0 {
			e.line("return")
			return
		}
		e.line("return " + e.exprList(s.Results))

	case *ir.Branch:
		kind := s.Kind
		if kind == "" || (kind == "goto" && s.Label == "") {
			// A goto with nowhere to go is not a statement at all.
			return
		}
		if s.Label != "" {
			kind += " " + s.Label
		}
		e.line(kind)

	case *ir.Labeled:
		if s.Label != "" {
			e.line(s.Label + ":")
		}
		e.stmt(s.Stmt)

	case *ir.Switch:
		e.switchStmt(s)

	case *ir.Defer:
		e.line("defer " + e.call(s.Call))

	case *ir.Go:
		e.line("go " + e.call(s.Call))

	case *ir.CommentStmt:
		// The developer wrote this comment, so it belongs in both renderings.
		e.comment(s.Lines)

	case *ir.TodoStmt:
		e.todo(s.Info)
		if s.Panic {
			// Identical text in both renderings: the panic is part of the
			// program, not part of the commentary.
			e.line("panic(" + strconv.Quote(panicText(s.Info)) + ")")
		}

	case *ir.RawStmt:
		e.rawLines(s.Source)
	}
}

// prologue writes everything that precedes a node: its provenance and notes in
// annotated mode, and its TODO in both, because work the tool could not do is
// never hidden from the reader.
func (e *Emitter) prologue(n ir.Annotated) {
	e.notes(n)
	if m := metaOf(n); m != nil && m.Todo != nil {
		e.todo(*m.Todo)
	}
}

// hasVisibleNotes reports whether prologue would write anything, which decides
// whether a doc comment needs a blank line to stay a doc comment.
func (e *Emitter) hasVisibleNotes(n ir.Annotated) bool {
	m := metaOf(n)
	if m == nil {
		return false
	}
	if m.Todo != nil {
		return true
	}
	if e.mode != Annotated {
		return false
	}
	if m.Prov.Valid() && m.Prov.Text != "" {
		return true
	}
	for _, note := range m.Notes {
		if e.unsaid(note.Text) {
			return true
		}
	}
	return false
}

// block writes a brace-delimited statement list, with the block's own
// annotations inside the braces, which is the only place they can go when the
// brace shares a line with an if or a for. The closing brace is written
// without a newline so callers can continue the line with else.
func (e *Emitter) block(b *ir.Block) { e.braces(b, true) }

func (e *Emitter) braces(b *ir.Block, annotate bool) {
	e.line("{")
	e.in()
	if b != nil {
		if annotate {
			e.prologue(b)
		}
		for _, s := range b.Stmts {
			e.stmt(s)
		}
	}
	e.out()
	e.w("}")
}

func (e *Emitter) ifStmt(s *ir.If) {
	e.w("if ")
	if s.Init != nil {
		e.w(e.inline(s.Init) + "; ")
	}
	cond := e.exprHeader(s.Cond)
	if cond == "" {
		cond = "true"
	}
	e.w(cond + " ")
	e.block(s.Then)

	switch el := s.Else.(type) {
	case nil:
		e.nl()
	case *ir.If:
		// The annotations of an else-if cannot go on their own line without
		// splitting the chain, so they are written inline.
		e.w(" else " + e.inlineNotes(el))
		e.ifStmt(el)
	case *ir.Block:
		e.w(" else ")
		e.block(el)
		e.nl()
	default:
		e.w(" else ")
		e.line("{")
		e.in()
		e.stmt(s.Else)
		e.out()
		e.line("}")
	}
}

func (e *Emitter) forStmt(s *ir.For) {
	if s.Label != "" {
		e.line(s.Label + ":")
	}
	e.w("for")
	switch {
	case s.Init == nil && s.Post == nil && s.Cond == nil:
		// An infinite loop: for { ... }.
	case s.Init == nil && s.Post == nil:
		e.w(" " + e.exprHeader(s.Cond))
	default:
		e.w(" " + e.inline(s.Init) + "; " + e.exprHeader(s.Cond) + "; " + e.inline(s.Post))
	}
	e.w(" ")
	e.block(s.Body)
	e.nl()
}

func (e *Emitter) rangeStmt(s *ir.Range) {
	if s.Label != "" {
		e.line(s.Label + ":")
	}
	op := "="
	if s.Define {
		op = ":="
	}
	x := e.exprHeader(s.X)
	if x == "" {
		x = "nil"
	}

	e.w("for ")
	switch {
	case s.Key == nil && s.Value == nil:
		// for range x: the loop runs for its side effects alone.
	case s.Value == nil:
		e.w(e.expr(s.Key) + " " + op + " ")
	default:
		key := e.expr(s.Key)
		if key == "" {
			key = "_"
		}
		e.w(key + ", " + e.expr(s.Value) + " " + op + " ")
	}
	e.w("range " + x + " ")
	e.block(s.Body)
	e.nl()
}

func (e *Emitter) switchStmt(s *ir.Switch) {
	e.w("switch ")
	if s.Init != nil {
		e.w(e.inline(s.Init) + "; ")
	}
	if s.Tag != nil {
		e.w(e.exprHeader(s.Tag) + " ")
	}
	e.line("{")
	for _, c := range s.Cases {
		if len(c.Values) == 0 {
			e.line("default:")
		} else {
			e.line("case " + e.exprList(c.Values) + ":")
		}
		e.in()
		if c.Body != nil {
			e.prologue(c.Body)
			for _, st := range c.Body.Stmts {
				e.stmt(st)
			}
		}
		e.out()
	}
	e.line("}")
}

// call renders the operand of defer or go. Go's grammar accepts only a call
// there, so anything else is wrapped in a function literal that is then
// called: the result still says what the IR said, and it still parses.
func (e *Emitter) call(x ir.Expr) string {
	s := e.expr(x)
	if s == "" {
		return "func() {}()"
	}
	switch x.(type) {
	case *ir.Call, *ir.RawExpr:
		return s
	}
	return "func() { _ = " + s + " }()"
}

// exprList renders a comma-separated expression list, dropping entries the IR
// left empty.
func (e *Emitter) exprList(xs []ir.Expr) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		if s := e.expr(x); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

// lhsList renders the left-hand side of an assignment, where a missing entry
// is the blank identifier rather than a dropped position.
func (e *Emitter) lhsList(xs []ir.Expr) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		s := e.expr(x)
		if s == "" {
			s = "_"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// varSpec renders a var or const declaration line. A declaration with neither
// a type nor a value would not compile, so a type is supplied.
func (e *Emitter) varSpec(keyword string, names []string, t *ir.Type, values []ir.Expr) string {
	if len(names) == 0 {
		return ""
	}
	line := keyword + " " + strings.Join(names, ", ")
	// A nil type means the type was left to the initialiser, which is what
	// `var x = value` says. Writing `any` there instead would be a different
	// and much worse declaration.
	typ := ""
	if t != nil && t.Kind != ir.Invalid && t.Kind != ir.Void {
		typ = e.typ(t)
	}
	if typ != "" {
		line += " " + typ
	}
	switch {
	case len(values) > 0:
		line += " = " + e.exprList(values)
	case typ == "":
		line += " any"
	case keyword == "const":
		// A const has to have a value; the zero value of its type is the
		// honest choice when the IR carries none.
		line += " = " + t.Zero(func(path string) { e.imports.Add(path) })
	}
	return line
}

// rawLines writes verbatim Go source line by line, so the emitter's own
// indentation still applies to it.
func (e *Emitter) rawLines(src string) {
	src = strings.TrimRight(src, "\n")
	if strings.TrimSpace(src) == "" {
		return
	}
	for _, l := range strings.Split(src, "\n") {
		if strings.TrimSpace(l) == "" {
			e.nl()
			continue
		}
		e.line(l)
	}
}

// inline renders a statement onto a single line for an if, for, or switch
// header. Comments are suppressed there: a line comment inside a header would
// swallow the rest of the construct.
func (e *Emitter) inline(s ir.Stmt) string {
	if s == nil {
		return ""
	}
	sub := &Emitter{mode: Clean, imports: e.imports, atLineStart: true, saidBefore: e.saidBefore}
	sub.stmt(s)

	// Drop any comment line: inside a header it would run to the end of the
	// line and take the rest of the construct with it.
	var kept []string
	for _, l := range strings.Split(sub.sb.String(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "//") {
			continue
		}
		kept = append(kept, l)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// panicText is the message of a TODO panic. It is the clean wording in both
// renderings, so the two programs stay byte-identical outside comments.
func panicText(t ir.Todo) string {
	msg := todoShort(t)
	if t.Code != "" {
		msg = t.Code + ": " + msg
	}
	return msg
}
