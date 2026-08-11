// Package parser builds a Perl AST from the token stream produced by the
// lexer. It is a recursive-descent parser with precedence climbing for
// expressions. It never executes the input and it never gives up on a whole
// file: statements it cannot understand become ast.Untranslated nodes with
// a reason and the raw source, and parsing continues at the next statement.
package parser

import (
	"fmt"
	"strings"

	"perl2golang/internal/perl/ast"
	"perl2golang/internal/perl/lexer"
	"perl2golang/internal/perl/token"
)

// Diag is a parse diagnostic with a position.
type Diag struct {
	Pos token.Pos
	Msg string
}

func (d Diag) Error() string { return fmt.Sprintf("%s: %s", d.Pos, d.Msg) }

// Result carries the parse output.
type Result struct {
	Program *ast.Program
	Diags   []Diag
}

// Parse lexes and parses src.
func Parse(src []byte) Result {
	toks, lexDiags := lexer.Lex(src)
	p := &parser{
		src:  string(src),
		toks: toks,
	}
	for _, d := range lexDiags {
		p.diags = append(p.diags, Diag{Pos: d.Pos, Msg: d.Msg})
	}
	prog := p.parseProgram()
	return Result{Program: prog, Diags: p.diags}
}

// ParseExprString parses a standalone expression, used by string
// interpolation to parse embedded code.
func ParseExprString(src string) (ast.Expr, []Diag) {
	toks, lexDiags := lexer.Lex([]byte(src))
	p := &parser{src: src, toks: toks}
	for _, d := range lexDiags {
		p.diags = append(p.diags, Diag{Pos: d.Pos, Msg: d.Msg})
	}
	p.skipTrivia()
	e := p.parseExpr(precLowest)
	return e, p.diags
}

type parser struct {
	src   string
	toks  []token.Token
	pos   int
	diags []Diag

	// pending comments to attach to the next statement
	pending []*ast.Comment

	// declaredSubs tracks sub names seen so far, mirroring perl's
	// declaration-order sensitivity for bareword parsing.
	declaredSubs map[string]bool
	// subProtos records each declared sub's prototype text, because a
	// prototype changes how later calls to that sub parse.
	subProtos map[string]string
}

func (p *parser) errorf(pos token.Pos, format string, args ...any) {
	p.diags = append(p.diags, Diag{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// cur returns the current significant token (trivia already skipped by next()).
func (p *parser) cur() token.Token { return p.toks[p.pos] }

func (p *parser) kind() token.Kind { return p.toks[p.pos].Kind }

// peekAt returns the nth significant token after the current one.
func (p *parser) peekAt(n int) token.Token {
	i := p.pos
	for n > 0 && p.toks[i].Kind != token.EOF {
		i++
		for isTrivia(p.toks[i].Kind) {
			i++
		}
		n--
	}
	return p.toks[i]
}

func isTrivia(k token.Kind) bool {
	return k == token.Comment || k == token.Pod
}

// next advances past the current token and any following trivia, collecting
// comments into pending.
func (p *parser) next() {
	if p.toks[p.pos].Kind != token.EOF {
		p.pos++
	}
	p.skipTrivia()
}

func (p *parser) skipTrivia() {
	for isTrivia(p.toks[p.pos].Kind) {
		t := p.toks[p.pos]
		c := commentAt(t)
		c.Pod = t.Kind == token.Pod
		p.pending = append(p.pending, c)
		p.pos++
	}
}

func commentAt(t token.Token) *ast.Comment {
	c := &ast.Comment{Text: t.Text}
	setSpan(c, t.Pos, endOf(t))
	return c
}

func endOf(t token.Token) token.Pos {
	e := t.Pos
	e.Offset += len(t.Text)
	nl := strings.Count(t.Text, "\n")
	if nl > 0 {
		e.Line += nl
		e.Col = len(t.Text) - strings.LastIndexByte(t.Text, '\n')
	} else {
		e.Col += len(t.Text)
	}
	return e
}

// expect consumes a token of kind k or records a diagnostic.
func (p *parser) expect(k token.Kind, what string) token.Token {
	t := p.cur()
	if t.Kind != k {
		p.errorf(t.Pos, "expected %s, found %q", what, t.Text)
		return t
	}
	p.next()
	return t
}

// accept consumes the current token if it has kind k.
func (p *parser) accept(k token.Kind) bool {
	if p.kind() == k {
		p.next()
		return true
	}
	return false
}

// acceptIdent consumes the current token when it is the identifier s.
func (p *parser) acceptIdent(s string) bool {
	if p.kind() == token.Ident && p.cur().Text == s {
		p.next()
		return true
	}
	return false
}

func (p *parser) isIdent(s string) bool {
	return p.kind() == token.Ident && p.cur().Text == s
}

// ---------------------------------------------------------------------------
// Program and statements

func (p *parser) parseProgram() *ast.Program {
	prog := &ast.Program{Source: p.src}
	if p.declaredSubs == nil {
		p.declaredSubs = map[string]bool{}
	}
	if p.subProtos == nil {
		p.subProtos = map[string]string{}
	}
	p.skipTrivia()
	start := p.cur().Pos
	for p.kind() != token.EOF && p.kind() != token.Data {
		st := p.parseStatement()
		if st != nil {
			prog.Stmts = append(prog.Stmts, st)
		}
	}
	if p.kind() == token.Data {
		t := p.cur()
		prog.HasData = true
		if len(t.Parts) > 0 {
			prog.Data = t.Parts[0]
		}
		p.next()
	}
	end := p.cur().Pos
	setSpan(prog, start, end)
	return prog
}

// parseStatement parses one statement, or returns nil for stray semicolons.
func (p *parser) parseStatement() ast.Stmt {
	lead := p.takePending()
	startTok := p.cur()

	st := p.parseStatementInner()
	if st == nil {
		return nil
	}
	attachComments(st, lead)
	p.attachLineComment(st, startTok.Pos.Line)
	return st
}

func (p *parser) takePending() []*ast.Comment {
	lead := p.pending
	p.pending = nil
	return lead
}

// attachLineComment attaches a trailing comment that sits on the statement's
// last line.
func (p *parser) attachLineComment(st ast.Stmt, _ int) {
	if len(p.pending) == 0 {
		return
	}
	last := p.pending[len(p.pending)-1]
	if last.Pos().Line == st.End().Line {
		setLineComment(st, last)
		p.pending = p.pending[:len(p.pending)-1]
	}
}

func (p *parser) parseStatementInner() ast.Stmt {
	t := p.cur()
	switch {
	case t.Kind == token.Semi:
		p.next()
		return nil
	case t.Kind == token.EOF, t.Kind == token.Data:
		return nil
	case t.Kind == token.LBrace:
		if p.looksLikeAnonHash() {
			return p.parseExprStatement()
		}
		return p.parseBareBlock("")
	case t.Kind == token.Ident:
		// Label?
		if p.peekAt(1).Kind == token.Colon && isLabelName(t.Text) && p.peekAt(2).Kind != token.Colon {
			label := t.Text
			p.next() // label
			p.next() // colon
			return p.parseLabeledStatement(label)
		}
		switch t.Text {
		case "if", "unless":
			return p.parseIf(t.Text == "unless")
		case "while", "until":
			return p.parseWhile(t.Text == "until", "")
		case "for", "foreach":
			return p.parseFor("")
		case "sub":
			if p.peekAt(1).Kind == token.Ident {
				return p.parseSubDecl()
			}
		case "package":
			return p.parsePackage()
		case "use", "no":
			return p.parseUse(t.Text == "no")
		case "return":
			return p.parseReturn()
		case "last", "next", "redo":
			return p.parseLoopCtl()
		case "do":
			if p.peekAt(1).Kind == token.LBrace {
				return p.parseDoBlockStatement()
			}
		case "BEGIN", "END", "INIT", "CHECK", "UNITCHECK":
			if p.peekAt(1).Kind == token.LBrace {
				return p.parsePhaseBlock(t.Text)
			}
		case "format":
			return p.refuseStatement("format blocks are a separate picture language with no Go equivalent")
		}
	}
	return p.parseExprStatement()
}

func isLabelName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	// Labels are conventionally upper-case; requiring that avoids
	// misreading `key: value` style text.
	return strings.ToUpper(s) == s
}

func (p *parser) parseLabeledStatement(label string) ast.Stmt {
	switch {
	case p.isIdent("while"), p.isIdent("until"):
		return p.parseWhile(p.cur().Text == "until", label)
	case p.isIdent("for"), p.isIdent("foreach"):
		return p.parseFor(label)
	case p.kind() == token.LBrace:
		return p.parseBareBlock(label)
	default:
		st := p.parseStatementInner()
		return st
	}
}

func (p *parser) parseIf(unless bool) ast.Stmt {
	start := p.cur().Pos
	p.next() // if/unless
	p.expect(token.LParen, "(")
	cond := p.parseExpr(precLowest)
	p.expect(token.RParen, ")")
	then := p.parseBlockBody()
	n := &ast.If{Unless: unless, Cond: cond, Then: then}
	for p.isIdent("elsif") {
		p.next()
		p.expect(token.LParen, "(")
		ec := p.parseExpr(precLowest)
		p.expect(token.RParen, ")")
		eb := p.parseBlockBody()
		n.ElseIfs = append(n.ElseIfs, ast.ElseIf{Cond: ec, Then: eb})
	}
	if p.isIdent("else") {
		p.next()
		n.Else = p.parseBlockBody()
	}
	setSpan(n, start, p.prevEnd())
	return n
}

func (p *parser) parseWhile(until bool, label string) ast.Stmt {
	start := p.cur().Pos
	p.next()
	p.expect(token.LParen, "(")
	var cond ast.Expr
	if p.kind() != token.RParen {
		cond = p.parseExpr(precLowest)
	}
	p.expect(token.RParen, ")")
	body := p.parseBlockBody()
	n := &ast.While{Until: until, Cond: cond, Body: body, Label: label}
	setSpan(n, start, p.prevEnd())
	return n
}

// parseFor handles C-style for, foreach over lists, and `for my $x (...)`.
func (p *parser) parseFor(label string) ast.Stmt {
	start := p.cur().Pos
	p.next() // for/foreach

	var loopVar ast.Expr
	myVar := false
	if p.isIdent("my") || p.isIdent("our") || p.isIdent("state") {
		myVar = p.cur().Text == "my" || p.cur().Text == "state"
		p.next()
		loopVar = p.parseTerm()
	} else if p.kind() == token.ScalarVar && p.peekAt(1).Kind == token.LParen {
		loopVar = p.parseTerm()
	}

	p.expect(token.LParen, "(")

	if loopVar == nil {
		// Could be C-style: for (init; cond; post)
		if p.looksLikeCStyleFor() {
			return p.parseCStyleFor(start, label)
		}
	}

	var list []ast.Expr
	if p.kind() != token.RParen {
		list = p.parseCommaList(token.RParen)
	}
	p.expect(token.RParen, ")")
	body := p.parseBlockBody()
	n := &ast.Foreach{Var: loopVar, MyVar: myVar, List: list, Body: body, Label: label}
	setSpan(n, start, p.prevEnd())
	return n
}

// looksLikeCStyleFor scans ahead for a `;` before the matching `)`.
func (p *parser) looksLikeCStyleFor() bool {
	depth := 0
	for i := p.pos; p.toks[i].Kind != token.EOF; i++ {
		switch p.toks[i].Kind {
		case token.LParen, token.LBracket, token.LBrace:
			depth++
		case token.RParen, token.RBracket, token.RBrace:
			if depth == 0 {
				return false
			}
			depth--
		case token.Semi:
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

func (p *parser) parseCStyleFor(start token.Pos, label string) ast.Stmt {
	var init, cond, post ast.Expr
	// Each clause may be a comma expression: for (my ($i, $j) = (0, 10); ...; $i++, $j--).
	if p.kind() != token.Semi {
		init = listToExpr(p.parseCommaList(token.Semi))
	}
	p.expect(token.Semi, ";")
	if p.kind() != token.Semi {
		cond = listToExpr(p.parseCommaList(token.Semi))
	}
	p.expect(token.Semi, ";")
	if p.kind() != token.RParen {
		post = listToExpr(p.parseCommaList(token.RParen))
	}
	p.expect(token.RParen, ")")
	body := p.parseBlockBody()
	n := &ast.ForC{Init: init, Cond: cond, Post: post, Body: body, Label: label}
	setSpan(n, start, p.prevEnd())
	return n
}

func (p *parser) parseSubDecl() ast.Stmt {
	start := p.cur().Pos
	p.next() // sub
	name := p.expect(token.Ident, "sub name").Text
	proto := ""
	// Prototype or signature.
	if p.kind() == token.LParen {
		proto = p.rawParenText()
	}
	// Attributes: `:attr` list.
	for p.kind() == token.Colon && p.peekAt(1).Kind == token.Ident {
		p.next()
		p.next()
		if p.kind() == token.LParen {
			p.rawParenText()
		}
	}
	var body []ast.Stmt
	if p.kind() == token.LBrace {
		body = p.parseBlockBody()
	} else {
		p.accept(token.Semi) // forward declaration
	}
	p.declaredSubs[name] = true
	if p.subProtos == nil {
		p.subProtos = map[string]string{}
	}
	p.subProtos[name] = proto
	n := &ast.SubDecl{Name: name, Proto: proto, Body: body}
	setSpan(n, start, p.prevEnd())
	return n
}

// rawParenText consumes a balanced (...) group and returns its raw text.
func (p *parser) rawParenText() string {
	var sb strings.Builder
	depth := 0
	for p.kind() != token.EOF {
		t := p.cur()
		if t.Kind == token.LParen {
			depth++
		}
		if t.Kind == token.RParen {
			depth--
		}
		sb.WriteString(t.Text)
		p.next()
		if depth == 0 {
			break
		}
	}
	s := sb.String()
	return strings.TrimSuffix(strings.TrimPrefix(s, "("), ")")
}

func (p *parser) parsePackage() ast.Stmt {
	start := p.cur().Pos
	p.next()
	name := p.expect(token.Ident, "package name").Text
	n := &ast.PackageDecl{Name: name}
	if p.kind() == token.Version || p.kind() == token.Number {
		p.next() // package version, ignored
	}
	if p.kind() == token.LBrace {
		n.Body = p.parseBlockBody()
	} else {
		p.accept(token.Semi)
	}
	setSpan(n, start, p.prevEnd())
	return n
}

func (p *parser) parseUse(no bool) ast.Stmt {
	start := p.cur().Pos
	p.next() // use/no
	n := &ast.Use{No: no}
	switch p.kind() {
	case token.Ident:
		n.Module = p.cur().Text
		p.next()
	case token.Number, token.Version:
		n.Module = ""
		p.next()
	}
	if p.kind() != token.Semi && p.kind() != token.EOF {
		n.Args = p.parseCommaList(token.Semi)
	}
	p.accept(token.Semi)
	setSpan(n, start, p.prevEnd())
	return n
}

func (p *parser) parseReturn() ast.Stmt {
	start := p.cur().Pos
	p.next()
	n := &ast.Return{}
	if !isStatementEnd(p.cur()) && !isStatementModifierKeyword(p.cur()) {
		n.Exprs = p.parseListOpArgs(token.Semi)
	}
	st := p.finishSimpleStatement(n, start, func(inner ast.Stmt) {})
	return st
}

func (p *parser) parseLoopCtl() ast.Stmt {
	start := p.cur().Pos
	op := p.cur().Text
	p.next()
	n := &ast.LoopCtl{Op: op}
	if p.kind() == token.Ident && isLabelName(p.cur().Text) && !isStatementModifierKeyword(p.cur()) {
		n.Label = p.cur().Text
		p.next()
	}
	return p.finishSimpleStatement(n, start, nil)
}

// parseDoBlockStatement handles `do { ... } while/until COND;` and a bare
// do-block statement.
func (p *parser) parseDoBlockStatement() ast.Stmt {
	start := p.cur().Pos
	p.next() // do
	body := p.parseBlockBody()
	if p.isIdent("while") || p.isIdent("until") {
		until := p.cur().Text == "until"
		p.next()
		paren := p.accept(token.LParen)
		cond := p.parseExpr(precLowest)
		if paren {
			p.expect(token.RParen, ")")
		}
		p.accept(token.Semi)
		n := &ast.While{Until: until, Cond: cond, Body: body, DoWhile: true}
		setSpan(n, start, p.prevEnd())
		return n
	}
	p.accept(token.Semi)
	n := &ast.Block{Body: body}
	setSpan(n, start, p.prevEnd())
	return n
}

// parsePhaseBlock maps BEGIN/END/INIT/CHECK blocks onto Block statements
// tagged by label so later phases can treat them specially.
func (p *parser) parsePhaseBlock(name string) ast.Stmt {
	start := p.cur().Pos
	p.next()
	body := p.parseBlockBody()
	p.accept(token.Semi)
	n := &ast.Block{Body: body, Label: name}
	setSpan(n, start, p.prevEnd())
	return n
}

// looksLikeAnonHash decides whether a `{` at the start of a statement opens a
// block or an anonymous hash.
//
// Perl faces the same ambiguity and resolves it by looking ahead in exactly
// this way: a brace followed by a bareword or a quoted string and then a comma
// or a fat comma is a hash constructor, and anything else is a block. The case
// that matters is the last statement of a map block, `{ name => $n, id => $i }`,
// which is the value the block produces rather than a block of its own.
func (p *parser) looksLikeAnonHash() bool {
	switch p.peekAt(1).Kind {
	case token.Ident, token.StrSingle, token.StrDouble, token.Number, token.ScalarVar:
	default:
		return false
	}
	switch p.peekAt(2).Kind {
	case token.FatComma:
		return true
	case token.Comma:
		// A comma alone is weaker evidence, so it only counts after something
		// that cannot start a statement of its own.
		return p.peekAt(1).Kind == token.StrSingle || p.peekAt(1).Kind == token.StrDouble
	}
	return false
}

func (p *parser) parseBareBlock(label string) ast.Stmt {
	start := p.cur().Pos
	body := p.parseBlockBody()
	n := &ast.Block{Body: body, Label: label}
	setSpan(n, start, p.prevEnd())
	return n
}

// parseBlockBody parses `{ stmt... }`.
//
// The loop stops at the __END__ marker as well as at EOF, because both end
// the program. parseStatement returns nil at either without consuming it, so
// a block left unclosed above a data section would otherwise ask for the next
// statement forever and the parse would never finish.
func (p *parser) parseBlockBody() []ast.Stmt {
	p.expect(token.LBrace, "{")
	var body []ast.Stmt
	for p.kind() != token.RBrace && p.kind() != token.EOF && p.kind() != token.Data {
		st := p.parseStatement()
		if st != nil {
			body = append(body, st)
		}
	}
	p.expect(token.RBrace, "}")
	return body
}

func isStatementEnd(t token.Token) bool {
	switch t.Kind {
	case token.Semi, token.RBrace, token.EOF, token.Data:
		return true
	}
	return false
}

func isStatementModifierKeyword(t token.Token) bool {
	if t.Kind != token.Ident {
		return false
	}
	switch t.Text {
	case "if", "unless", "while", "until", "for", "foreach":
		return true
	}
	return false
}

// parseExprStatement parses an expression statement plus optional statement
// modifier. On failure it recovers to the next semicolon and returns an
// Untranslated node.
func (p *parser) parseExprStatement() ast.Stmt {
	start := p.cur().Pos
	startIdx := p.pos
	diagsBefore := len(p.diags)

	x := listToExpr(p.parseCommaList(token.Semi))
	if x == nil {
		x = p.parseExpr(precLowest)
	}

	if len(p.diags) > diagsBefore && p.kind() != token.Semi && !isStatementEnd(p.cur()) && !isStatementModifierKeyword(p.cur()) {
		// The expression parse went wrong midway; give up on this
		// statement and resynchronise.
		return p.recoverStatement(start, startIdx, p.diags[diagsBefore].Msg)
	}

	st := ast.Stmt(nil)
	es := &ast.ExprStmt{X: x}
	// The span goes on before any modifier wraps the statement, or the inner
	// statement of `EXPR if COND` is left with no position at all and every
	// diagnostic inside it falls back to nowhere.
	setSpan(es, start, p.prevEnd())
	st = es
	st = p.applyStatementModifiers(st, start)
	p.accept(token.Semi)
	if s, ok := st.(*ast.ExprStmt); ok {
		setSpan(s, start, p.prevEnd())
	}
	return st
}

// finishSimpleStatement applies modifiers and the closing semicolon.
func (p *parser) finishSimpleStatement(inner ast.Stmt, start token.Pos, _ func(ast.Stmt)) ast.Stmt {
	setSpanStmt(inner, start, p.prevEnd())
	st := p.applyStatementModifiers(inner, start)
	p.accept(token.Semi)
	setSpanStmt(st, start, p.prevEnd())
	return st
}

// applyStatementModifiers wraps st in If/While/Foreach for trailing
// `... if COND`, `... while COND`, `... for LIST` forms.
func (p *parser) applyStatementModifiers(st ast.Stmt, start token.Pos) ast.Stmt {
	for p.kind() == token.Ident {
		switch p.cur().Text {
		case "if", "unless":
			unless := p.cur().Text == "unless"
			p.next()
			cond := p.parseExpr(precLowest)
			n := &ast.If{Unless: unless, Cond: cond, Then: []ast.Stmt{st}, Modifier: true}
			setSpan(n, start, p.prevEnd())
			st = n
		case "while", "until":
			until := p.cur().Text == "until"
			p.next()
			cond := p.parseExpr(precLowest)
			n := &ast.While{Until: until, Cond: cond, Body: []ast.Stmt{st}, Modifier: true}
			setSpan(n, start, p.prevEnd())
			st = n
		case "for", "foreach":
			p.next()
			list := p.parseCommaList(token.Semi)
			n := &ast.Foreach{List: list, Body: []ast.Stmt{st}, Modifier: true}
			setSpan(n, start, p.prevEnd())
			st = n
		default:
			return st
		}
	}
	return st
}

// refuseStatement records a diagnostic and skips the statement (through a
// balanced region up to `;` or, for block-shaped constructs, the closing
// brace or terminator).
func (p *parser) refuseStatement(reason string) ast.Stmt {
	start := p.cur().Pos
	return p.recoverStatement(start, p.pos, reason)
}

// recoverStatement skips to the next statement boundary and produces an
// Untranslated node covering the skipped source.
func (p *parser) recoverStatement(start token.Pos, startIdx int, reason string) ast.Stmt {
	// Special case: `format NAME =` ... `.` picture blocks end at a lone dot line.
	if p.isIdent("format") {
		p.skipFormatBlock()
	} else {
		depth := 0
		for p.kind() != token.EOF && p.kind() != token.Data {
			switch p.kind() {
			case token.LBrace, token.LParen, token.LBracket:
				depth++
			case token.RBrace, token.RParen, token.RBracket:
				if depth == 0 {
					goto done
				}
				depth--
			case token.Semi:
				if depth == 0 {
					p.next()
					goto done
				}
			}
			p.next()
		}
	done:
	}
	end := p.prevEnd()
	raw := ""
	if startTok := p.toks[startIdx]; startTok.Pos.Offset < len(p.src) {
		to := end.Offset
		if to > len(p.src) {
			to = len(p.src)
		}
		if to > startTok.Pos.Offset {
			raw = p.src[startTok.Pos.Offset:to]
		}
	}
	p.errorf(start, "could not translate statement: %s", reason)
	n := &ast.Untranslated{Reason: reason, Raw: strings.TrimSpace(raw)}
	setSpan(n, start, end)
	return n
}

// skipFormatBlock consumes tokens until a lone `.` line. The lexer does not
// understand format pictures, so this is a best-effort resync.
func (p *parser) skipFormatBlock() {
	for p.kind() != token.EOF {
		t := p.cur()
		if t.Kind == token.Dot && t.Pos.Col == 1 {
			p.next()
			return
		}
		p.next()
	}
}

// prevEnd returns the end position of the most recently consumed token.
func (p *parser) prevEnd() token.Pos {
	i := p.pos - 1
	for i >= 0 && isTrivia(p.toks[i].Kind) {
		i--
	}
	if i < 0 {
		return p.cur().Pos
	}
	return endOf(p.toks[i])
}

// ---------------------------------------------------------------------------
// span/comment plumbing

type spanner interface{ SetSpan(from, to token.Pos) }

func setSpan(n any, from, to token.Pos) {
	if v, ok := n.(spanner); ok {
		v.SetSpan(from, to)
	}
}

func setSpanStmt(st ast.Stmt, from, to token.Pos) { setSpan(st, from, to) }

func attachComments(st ast.Stmt, lead []*ast.Comment) {
	if len(lead) == 0 {
		return
	}
	if sc := stmtComments(st); sc != nil {
		sc.Lead = append(sc.Lead, lead...)
	}
}

func setLineComment(st ast.Stmt, c *ast.Comment) {
	if sc := stmtComments(st); sc != nil {
		sc.Line = c
	}
}

func stmtComments(st ast.Stmt) *ast.StmtComments {
	switch v := st.(type) {
	case *ast.ExprStmt:
		return &v.StmtComments
	case *ast.If:
		return &v.StmtComments
	case *ast.While:
		return &v.StmtComments
	case *ast.ForC:
		return &v.StmtComments
	case *ast.Foreach:
		return &v.StmtComments
	case *ast.Block:
		return &v.StmtComments
	case *ast.SubDecl:
		return &v.StmtComments
	case *ast.PackageDecl:
		return &v.StmtComments
	case *ast.Use:
		return &v.StmtComments
	case *ast.Return:
		return &v.StmtComments
	case *ast.LoopCtl:
		return &v.StmtComments
	case *ast.Untranslated:
		return &v.StmtComments
	}
	return nil
}
