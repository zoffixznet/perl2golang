package parser

import (
	"strings"

	"perl2go/internal/perl/ast"
	"perl2go/internal/perl/token"
)

// Operator precedence levels, high binds tight. Comma is handled
// structurally (parseCommaList), not as a binary operator.
const (
	precLowest = 1

	precOrLow  = 3 // or xor
	precAndLow = 4 // and
	precNotLow = 5 // not (prefix)

	precAssign         = 7 // = += .= //= ...
	precTernary        = 8
	precRange          = 9  // .. ...
	precOrOr           = 10 // || //
	precAndAnd         = 11
	precBitOr          = 12 // | ^
	precBitAnd         = 13
	precEquality       = 14 // == != <=> eq ne cmp
	precRelational     = 15 // < > <= >= lt gt le ge
	precNamedUnary     = 16 // defined, length, -e ...
	precShift          = 17 // << >>
	precAdditive       = 18 // + - .
	precMultiplicative = 19 // * / % x
	precMatchBind      = 20 // =~ !~
	precUnary          = 21 // ! ~ \ unary + unary -
	precPower          = 22 // **
)

type opInfo struct {
	prec       int
	rightAssoc bool
}

func binaryOp(k token.Kind) (opInfo, bool) {
	switch k {
	case token.OrLow, token.XorLow:
		return opInfo{precOrLow, false}, true
	case token.AndLow:
		return opInfo{precAndLow, false}, true
	case token.Assign, token.OpAssign:
		return opInfo{precAssign, true}, true
	case token.Question:
		return opInfo{precTernary, true}, true
	case token.DotDot, token.DotDotDot:
		return opInfo{precRange, false}, true
	case token.OrOr, token.DefinedOr:
		return opInfo{precOrOr, false}, true
	case token.AndAnd:
		return opInfo{precAndAnd, false}, true
	case token.BitOr, token.BitXor:
		return opInfo{precBitOr, false}, true
	case token.BitAnd:
		return opInfo{precBitAnd, false}, true
	case token.NumEq, token.NumNe, token.NumCmp, token.StrEq, token.StrNe, token.StrCmp:
		return opInfo{precEquality, false}, true
	case token.NumLt, token.NumGt, token.NumLe, token.NumGe,
		token.StrLt, token.StrGt, token.StrLe, token.StrGe:
		return opInfo{precRelational, false}, true
	case token.ShiftLeft, token.ShiftRight:
		return opInfo{precShift, false}, true
	case token.Plus, token.Minus, token.Dot:
		return opInfo{precAdditive, false}, true
	case token.Star, token.Slash, token.Percent, token.Repeat:
		return opInfo{precMultiplicative, false}, true
	case token.MatchBind, token.NotMatchBind:
		return opInfo{precMatchBind, false}, true
	case token.StarStar:
		return opInfo{precPower, true}, true
	}
	return opInfo{}, false
}

// parseExpr parses an expression with precedence climbing.
func (p *parser) parseExpr(minPrec int) ast.Expr {
	var left ast.Expr
	if p.kind() == token.NotLow {
		start := p.cur().Pos
		p.next()
		x := p.parseExpr(precNotLow)
		n := &ast.UnOp{Op: "not", X: x}
		setSpan(n, start, p.prevEnd())
		left = n
	} else {
		left = p.parseTerm()
	}

	for {
		t := p.cur()
		info, ok := binaryOp(t.Kind)
		if !ok || info.prec < minPrec {
			return left
		}
		switch t.Kind {
		case token.Question:
			p.next()
			a := p.parseExpr(precAssign)
			p.expect(token.Colon, ":")
			b := p.parseExpr(precTernary)
			n := &ast.Ternary{Cond: left, A: a, B: b}
			setSpan(n, left.Pos(), p.prevEnd())
			left = n
		case token.Assign, token.OpAssign:
			p.next()
			rhs := p.parseExpr(precAssign)
			n := &ast.Assign{Op: t.Text, LHS: left, RHS: rhs}
			setSpan(n, left.Pos(), p.prevEnd())
			left = n
		case token.MatchBind, token.NotMatchBind:
			p.next()
			left = p.parseMatchBind(left, t.Kind == token.NotMatchBind)
		default:
			p.next()
			nextMin := info.prec + 1
			if info.rightAssoc {
				nextMin = info.prec
			}
			rhs := p.parseExpr(nextMin)
			n := &ast.BinOp{Op: t.Text, L: left, R: rhs}
			setSpan(n, left.Pos(), p.prevEnd())
			left = n
		}
	}
}

// parseMatchBind parses the right side of =~ / !~.
func (p *parser) parseMatchBind(bound ast.Expr, negate bool) ast.Expr {
	t := p.cur()
	switch t.Kind {
	case token.Match, token.QuoteRegex:
		p.next()
		re := p.regexFromToken(t)
		n := &ast.Match{Bound: bound, Negate: negate, Pattern: re}
		setSpan(n, bound.Pos(), p.prevEnd())
		return n
	case token.Substitute:
		p.next()
		n := p.substFromToken(t)
		n.Bound = bound
		n.Negate = negate
		setSpan(n, bound.Pos(), p.prevEnd())
		return n
	case token.Transliterate:
		p.next()
		n := p.transFromToken(t)
		n.Bound = bound
		n.Negate = negate
		setSpan(n, bound.Pos(), p.prevEnd())
		return n
	default:
		// $str =~ $qr  or  $str =~ "pattern"
		x := p.parseExpr(precMatchBind + 1)
		n := &ast.Match{Bound: bound, Negate: negate, PatternExpr: x}
		setSpan(n, bound.Pos(), p.prevEnd())
		return n
	}
}

// parseCommaList parses a comma-separated list of expressions, stopping
// before stop (which is not consumed) or at any list terminator. Trailing
// low-precedence operators are folded in, so `@a = f(1), g(2) or die` parses
// the way Perl parses it.
func (p *parser) parseCommaList(stop token.Kind) []ast.Expr {
	return p.parseLowList(stop, precLowest)
}

// parseLowList parses a comma list and then folds any `and`/`or`/`xor` that
// follows it. In Perl the comma operator binds tighter than these three and
// looser than everything else, so their operands are whole lists.
func (p *parser) parseLowList(stop token.Kind, minPrec int) []ast.Expr {
	elems := p.parseSimpleList(stop)
	for len(elems) > 0 {
		var prec int
		switch p.kind() {
		case token.OrLow, token.XorLow:
			prec = precOrLow
		case token.AndLow:
			prec = precAndLow
		default:
			return elems
		}
		if prec < minPrec {
			return elems
		}
		t := p.cur()
		p.next()
		rhs := p.parseLowList(stop, prec+1)
		left := listToExpr(elems)
		right := listToExpr(rhs)
		if right == nil {
			p.errorf(t.Pos, "missing right operand for %q", t.Text)
			return elems
		}
		n := &ast.BinOp{Op: t.Text, L: left, R: right}
		setSpan(n, left.Pos(), p.prevEnd())
		elems = []ast.Expr{n}
	}
	return elems
}

// parseSimpleList parses comma-separated elements only, with no low-operator
// folding.
func (p *parser) parseSimpleList(stop token.Kind) []ast.Expr {
	var out []ast.Expr
	for {
		if p.kind() == stop || isListEnd(p.cur()) || isStatementModifierKeyword(p.cur()) {
			return out
		}
		e := p.parseListElem()
		out = append(out, e)
		if p.kind() == token.Comma || p.kind() == token.FatComma {
			p.next()
			continue
		}
		return out
	}
}

// listToExpr collapses a parsed list into a single expression: one element
// stays itself, several become a List, none becomes nil.
func listToExpr(es []ast.Expr) ast.Expr {
	switch len(es) {
	case 0:
		return nil
	case 1:
		return es[0]
	}
	n := &ast.List{Elems: es}
	setSpan(n, es[0].Pos(), es[len(es)-1].End())
	return n
}

// parseListElem parses one list element, applying fat-comma autoquoting.
func (p *parser) parseListElem() ast.Expr {
	if p.kind() == token.Ident && p.peekAt(1).Kind == token.FatComma && isAutoquotable(p.cur().Text) {
		t := p.cur()
		p.next()
		n := &ast.StrLit{Value: t.Text}
		setSpan(n, t.Pos, endOf(t))
		return n
	}
	return p.parseExpr(precAssign)
}

// isAutoquotable applies the fat-comma rule: /^[A-Za-z_]\w*$/.
func isAutoquotable(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func isListEnd(t token.Token) bool {
	switch t.Kind {
	case token.RParen, token.RBracket, token.RBrace, token.Semi, token.EOF,
		token.Colon, token.Data:
		return true
	case token.OrLow, token.AndLow, token.XorLow:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Terms

func (p *parser) parseTerm() ast.Expr {
	t := p.cur()
	start := t.Pos

	var x ast.Expr
	switch t.Kind {
	case token.Number, token.Version:
		p.next()
		n := &ast.NumberLit{Text: t.Text}
		setSpan(n, start, endOf(t))
		x = n

	case token.StrSingle:
		p.next()
		n := &ast.StrLit{Value: decodeSingleQuoted(rawBody(t))}
		setSpan(n, start, endOf(t))
		x = n

	case token.StrDouble:
		p.next()
		x = p.interpString(rawBody(t), t)

	case token.Heredoc:
		p.next()
		if t.Interp {
			x = p.interpString(rawBody(t), t)
		} else {
			n := &ast.StrLit{Value: rawBody(t)}
			setSpan(n, start, endOf(t))
			x = n
		}

	case token.StrBacktick:
		p.next()
		n := &ast.BacktickCmd{Parts: p.interpParts(rawBody(t), t, true)}
		setSpan(n, start, endOf(t))
		x = n

	case token.QwList:
		p.next()
		n := &ast.QwLit{Words: strings.Fields(rawBody(t))}
		setSpan(n, start, endOf(t))
		x = n

	case token.Match, token.QuoteRegex:
		p.next()
		re := p.regexFromToken(t)
		if t.Kind == token.QuoteRegex {
			n := &ast.QrExpr{Pattern: re}
			setSpan(n, start, endOf(t))
			x = n
		} else {
			n := &ast.Match{Pattern: re}
			setSpan(n, start, endOf(t))
			x = n
		}

	case token.Substitute:
		p.next()
		n := p.substFromToken(t)
		setSpan(n, start, endOf(t))
		x = n

	case token.Transliterate:
		p.next()
		n := p.transFromToken(t)
		setSpan(n, start, endOf(t))
		x = n

	case token.Readline:
		p.next()
		x = readlineFromToken(t)

	case token.Glob:
		p.next()
		inner := strings.TrimSuffix(strings.TrimPrefix(t.Text, "<"), ">")
		lit := &ast.StrLit{Value: inner}
		setSpan(lit, start, endOf(t))
		n := &ast.GlobExpr{Pattern: lit}
		setSpan(n, start, endOf(t))
		x = n

	case token.FileTest:
		p.next()
		var arg ast.Expr
		if p.startsTerm() {
			arg = p.parseExpr(precNamedUnary)
		}
		n := &ast.FileTest{Op: t.Text[1], Arg: arg}
		setSpan(n, start, p.prevEnd())
		x = n

	case token.ScalarVar, token.ArrayVar, token.HashVar, token.FuncVar, token.GlobVar, token.ArrayLen:
		x = p.parseVariable()

	case token.Backslash:
		p.next()
		inner := p.parseExpr(precUnary)
		n := &ast.RefGen{X: inner}
		setSpan(n, start, p.prevEnd())
		x = n

	case token.Not:
		p.next()
		inner := p.parseExpr(precUnary)
		n := &ast.UnOp{Op: "!", X: inner}
		setSpan(n, start, p.prevEnd())
		x = n

	case token.BitNot:
		p.next()
		inner := p.parseExpr(precUnary)
		n := &ast.UnOp{Op: "~", X: inner}
		setSpan(n, start, p.prevEnd())
		x = n

	case token.Minus:
		p.next()
		if p.kind() == token.Ident && !p.declaredSubs[p.cur().Text] && !isBuiltin(p.cur().Text) && p.peekAt(1).Kind != token.LParen {
			// Unary minus on a bareword yields the string "-bareword".
			id := p.cur()
			p.next()
			n := &ast.StrLit{Value: "-" + id.Text}
			setSpan(n, start, endOf(id))
			x = n
		} else {
			inner := p.parseExpr(precUnary)
			n := &ast.UnOp{Op: "-", X: inner}
			setSpan(n, start, p.prevEnd())
			x = n
		}

	case token.Plus:
		p.next()
		x = p.parseExpr(precUnary)

	case token.PlusPlus, token.MinusMinus:
		p.next()
		inner := p.parseTerm()
		n := &ast.UnOp{Op: t.Text, X: inner}
		setSpan(n, start, p.prevEnd())
		x = n

	case token.LParen:
		p.next()
		elems := p.parseCommaList(token.RParen)
		p.expect(token.RParen, ")")
		n := &ast.List{Elems: elems}
		setSpan(n, start, p.prevEnd())
		x = n

	case token.LBracket:
		p.next()
		elems := p.parseCommaList(token.RBracket)
		p.expect(token.RBracket, "]")
		n := &ast.AnonArray{Elems: elems}
		setSpan(n, start, p.prevEnd())
		x = n

	case token.LBrace:
		p.next()
		elems := p.parseCommaList(token.RBrace)
		p.expect(token.RBrace, "}")
		n := &ast.AnonHash{Elems: elems}
		setSpan(n, start, p.prevEnd())
		x = n

	case token.Ident:
		x = p.parseIdentTerm()

	default:
		p.errorf(t.Pos, "unexpected %q in expression", t.Text)
		p.next()
		n := &ast.BadExpr{Reason: "unexpected token " + t.Text, Raw: t.Text}
		setSpan(n, start, endOf(t))
		return n
	}

	return p.parsePostfix(x)
}

// startsTerm reports whether the current token can begin a term.
func (p *parser) startsTerm() bool {
	switch p.kind() {
	case token.Number, token.Version, token.StrSingle, token.StrDouble, token.Heredoc,
		token.StrBacktick, token.QwList, token.Match, token.QuoteRegex, token.Substitute,
		token.Transliterate, token.Readline, token.Glob, token.FileTest,
		token.ScalarVar, token.ArrayVar, token.HashVar, token.FuncVar, token.GlobVar,
		token.ArrayLen, token.Backslash, token.Not, token.BitNot, token.Minus,
		token.Plus, token.PlusPlus, token.MinusMinus, token.LParen, token.LBracket,
		token.LBrace:
		return true
	case token.Ident:
		return !isStatementModifierKeyword(p.cur()) && !isLowOpIdent(p.cur().Text)
	}
	return false
}

func isLowOpIdent(s string) bool {
	switch s {
	case "and", "or", "not", "xor", "eq", "ne", "lt", "gt", "le", "ge", "cmp", "x":
		return true
	}
	return false
}

// parsePostfix applies subscripts, arrows, calls and ++/-- to a term.
func (p *parser) parsePostfix(x ast.Expr) ast.Expr {
	for {
		t := p.cur()
		switch t.Kind {
		case token.LBracket:
			if lst, ok := x.(*ast.List); ok {
				// List slice: (LIST)[indices].
				p.next()
				idx := p.parseCommaList(token.RBracket)
				p.expect(token.RBracket, "]")
				n := &ast.Slice{Base: lst, Idx: idx}
				setSpan(n, x.Pos(), p.prevEnd())
				x = n
				continue
			}
			if !canSubscript(x) {
				return x
			}
			p.next()
			if isSliceBase(x) {
				idx := p.parseCommaList(token.RBracket)
				p.expect(token.RBracket, "]")
				n := &ast.Slice{Base: x, Idx: idx}
				setSpan(n, x.Pos(), p.prevEnd())
				x = n
				continue
			}
			idx := p.parseExpr(precLowest)
			p.expect(token.RBracket, "]")
			n := &ast.Index{Base: x, Idx: idx, Arrow: afterSubscript(x)}
			setSpan(n, x.Pos(), p.prevEnd())
			x = n
		case token.LBrace:
			if !canSubscript(x) {
				return x
			}
			p.next()
			if isSliceBase(x) {
				idx := p.parseHashKeyList(token.RBrace)
				p.expect(token.RBrace, "}")
				n := &ast.Slice{Base: x, Idx: idx, Hash: true}
				setSpan(n, x.Pos(), p.prevEnd())
				x = n
				continue
			}
			key := p.parseHashKey()
			p.expect(token.RBrace, "}")
			n := &ast.HashIndex{Base: x, Key: key, Arrow: afterSubscript(x)}
			setSpan(n, x.Pos(), p.prevEnd())
			x = n
		case token.Arrow:
			x = p.parseArrow(x)
		case token.LParen:
			// &$code(...) and &{$code}(...) call through a code reference.
			if d, ok := x.(*ast.Deref); ok && d.Sigil == '&' {
				p.next()
				args := p.parseCommaList(token.RParen)
				p.expect(token.RParen, ")")
				n := &ast.FuncCallRef{Ref: d.X, Args: args}
				setSpan(n, x.Pos(), p.prevEnd())
				x = n
				continue
			}
			// $cr->(...) is handled by the Arrow case; (expr)(args) is not
			// a Perl call form.
			return x
		case token.PlusPlus, token.MinusMinus:
			p.next()
			n := &ast.UnOp{Op: t.Text, X: x, Postfix: true}
			setSpan(n, x.Pos(), p.prevEnd())
			x = n
		default:
			return x
		}
	}
}

// canSubscript reports whether a [ or { directly after x is a subscript.
func canSubscript(x ast.Expr) bool {
	switch v := x.(type) {
	case *ast.Var:
		return v.Sigil == '$' || v.Sigil == '@' || v.Sigil == '%'
	case *ast.Deref:
		return v.Sigil == '$' || v.Sigil == '@' || v.Sigil == '%' || v.Sigil == '#'
	case *ast.Index, *ast.HashIndex, *ast.Slice:
		return true
	case *ast.MethodCall, *ast.FuncCallRef:
		return false
	}
	return false
}

// isSliceBase reports whether x is an @-sigil or %-sigil entity so that a
// subscript on it means a slice rather than an element.
func isSliceBase(x ast.Expr) bool {
	switch v := x.(type) {
	case *ast.Var:
		return v.Sigil == '@' || v.Sigil == '%'
	case *ast.Deref:
		return v.Sigil == '@' || v.Sigil == '%'
	}
	return false
}

// afterSubscript reports whether x is already a subscript access, which
// makes a following bare subscript an implicit-arrow chained access.
func afterSubscript(x ast.Expr) bool {
	switch x.(type) {
	case *ast.Index, *ast.HashIndex:
		return true
	}
	return false
}

// parseArrow handles ->[i], ->{k}, ->(args), ->method, ->$m, ->@*, ->%*, ->$#*.
func (p *parser) parseArrow(x ast.Expr) ast.Expr {
	p.next() // ->
	t := p.cur()
	switch t.Kind {
	case token.LBracket:
		p.next()
		idx := p.parseExpr(precLowest)
		p.expect(token.RBracket, "]")
		n := &ast.Index{Base: x, Idx: idx, Arrow: true}
		setSpan(n, x.Pos(), p.prevEnd())
		return n
	case token.LBrace:
		p.next()
		key := p.parseHashKey()
		p.expect(token.RBrace, "}")
		n := &ast.HashIndex{Base: x, Key: key, Arrow: true}
		setSpan(n, x.Pos(), p.prevEnd())
		return n
	case token.LParen:
		p.next()
		args := p.parseCommaList(token.RParen)
		p.expect(token.RParen, ")")
		n := &ast.FuncCallRef{Ref: x, Args: args}
		setSpan(n, x.Pos(), p.prevEnd())
		return n
	case token.Ident:
		name := t.Text
		p.next()
		var args []ast.Expr
		paren := false
		if p.kind() == token.LParen {
			paren = true
			p.next()
			args = p.parseCommaList(token.RParen)
			p.expect(token.RParen, ")")
		}
		n := &ast.MethodCall{Invocant: x, Method: name, Args: args, Paren: paren}
		setSpan(n, x.Pos(), p.prevEnd())
		return n
	case token.ScalarVar:
		mv := p.parseVariable()
		var args []ast.Expr
		paren := false
		if p.kind() == token.LParen {
			paren = true
			p.next()
			args = p.parseCommaList(token.RParen)
			p.expect(token.RParen, ")")
		}
		n := &ast.MethodCall{Invocant: x, Dynamic: mv, Args: args, Paren: paren}
		setSpan(n, x.Pos(), p.prevEnd())
		return n
	case token.ArrayVar, token.HashVar:
		// Postfix deref: ->@*, ->%*
		sigil := '@'
		if t.Kind == token.HashVar {
			sigil = '%'
		}
		p.next()
		p.accept(token.Star)
		n := &ast.Deref{Sigil: sigil, X: x}
		setSpan(n, x.Pos(), p.prevEnd())
		return n
	case token.ArrayLen:
		p.next()
		p.accept(token.Star)
		n := &ast.Deref{Sigil: '#', X: x}
		setSpan(n, x.Pos(), p.prevEnd())
		return n
	default:
		p.errorf(t.Pos, "unexpected %q after ->", t.Text)
		return x
	}
}

// parseHashKey parses the inside of a { } hash subscript, applying bareword
// autoquoting.
func (p *parser) parseHashKey() ast.Expr {
	t := p.cur()
	if t.Kind == token.Ident && p.peekAt(1).Kind == token.RBrace && isAutoquotable(t.Text) {
		p.next()
		n := &ast.StrLit{Value: t.Text}
		setSpan(n, t.Pos, endOf(t))
		return n
	}
	if t.Kind == token.Minus && p.peekAt(1).Kind == token.Ident && p.peekAt(2).Kind == token.RBrace {
		p.next()
		id := p.cur()
		p.next()
		n := &ast.StrLit{Value: "-" + id.Text}
		setSpan(n, t.Pos, endOf(id))
		return n
	}
	return p.parseExpr(precLowest)
}

// parseHashKeyList parses the inside of a { } hash-slice subscript.
func (p *parser) parseHashKeyList(stop token.Kind) []ast.Expr {
	var out []ast.Expr
	for p.kind() != stop && !isListEnd(p.cur()) {
		e := p.parseListElem()
		out = append(out, e)
		if !p.accept(token.Comma) && p.kind() != token.FatComma {
			break
		}
		if p.kind() == token.FatComma {
			p.next()
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Variables and dereferences

// parseVariable parses a variable token, assembling deref chains from bare
// sigils: $$x, ${...}, @$x, @{...}, $#$x, $#{...}, &$x.
func (p *parser) parseVariable() ast.Expr {
	t := p.cur()
	start := t.Pos
	sigilOnly := t.Text == "$" || t.Text == "@" || t.Text == "%" || t.Text == "&" || t.Text == "*" || t.Text == "$#"

	if !sigilOnly {
		p.next()
		n := varFromToken(t)
		return n
	}

	sig := rune(t.Text[0])
	if t.Text == "$#" {
		sig = '#'
	}
	p.next()

	var inner ast.Expr
	switch p.kind() {
	case token.LBrace:
		p.next()
		inner = p.parseExpr(precLowest)
		p.expect(token.RBrace, "}")
	case token.ScalarVar:
		inner = p.parseVariable()
	default:
		p.errorf(p.cur().Pos, "expected variable or block after sigil %q", t.Text)
		n := &ast.BadExpr{Reason: "dangling sigil", Raw: t.Text}
		setSpan(n, start, p.prevEnd())
		return n
	}
	n := &ast.Deref{Sigil: sig, X: inner}
	setSpan(n, start, p.prevEnd())
	return n
}

func varFromToken(t token.Token) ast.Expr {
	text := t.Text
	sig := rune(text[0])
	name := text[1:]
	if strings.HasPrefix(text, "$#") {
		sig = '#'
		name = text[2:]
	}
	// ${name} and ${^NAME} forms.
	name = strings.TrimSuffix(strings.TrimPrefix(name, "{"), "}")
	n := &ast.Var{Sigil: sig, Name: name}
	n.SetSpan(t.Pos, endOf(t))
	return n
}

func readlineFromToken(t token.Token) ast.Expr {
	inner := strings.TrimSuffix(strings.TrimPrefix(t.Text, "<"), ">")
	n := &ast.Readline{}
	if strings.HasPrefix(inner, "$") {
		v := &ast.Var{Sigil: '$', Name: inner[1:]}
		v.SetSpan(t.Pos, endOf(t))
		n.Var = v
	} else {
		n.Handle = inner // may be "", "STDIN", "FH", or "<>" for <<>>
	}
	n.SetSpan(t.Pos, endOf(t))
	return n
}

// ---------------------------------------------------------------------------
// Quote-like helpers

// unescapeDelim removes the backslash from every \<delim> pair, and from the
// matching closer when the delimiter is a bracketing one. Everything else is
// left exactly as written.
func unescapeDelim(s string, delim byte) string {
	if delim == 0 || !strings.ContainsRune(s, '\\') {
		return s
	}
	closer := delim
	switch delim {
	case '(':
		closer = ')'
	case '[':
		closer = ']'
	case '{':
		closer = '}'
	case '<':
		closer = '>'
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (s[i+1] == delim || s[i+1] == closer) {
			i++
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

func rawBody(t token.Token) string {
	if len(t.Parts) > 0 {
		return t.Parts[0]
	}
	return ""
}

func (p *parser) regexFromToken(t token.Token) *ast.Regex {
	re := &ast.Regex{Raw: rawBody(t), Mods: t.Mods}
	re.SetSpan(t.Pos, endOf(t))
	re.Parts = p.regexParts(rawBody(t), t)
	return re
}

func (p *parser) substFromToken(t token.Token) *ast.Subst {
	n := &ast.Subst{}
	n.Pattern = &ast.Regex{Raw: rawBody(t), Mods: t.Mods}
	n.Pattern.SetSpan(t.Pos, endOf(t))
	n.Pattern.Parts = p.regexParts(rawBody(t), t)
	repl := ""
	if len(t.Parts) > 1 {
		repl = t.Parts[1]
	}
	n.ReplRaw = repl
	n.EvalRepl = strings.Contains(t.Mods, "e")
	if n.EvalRepl {
		// Under /e the replacement is code, so the backslashes that were
		// only there to hide the delimiter have to come off first.
		e, diags := ParseExprString(unescapeDelim(repl, t.Delim))
		p.diags = append(p.diags, diags...)
		n.Repl = e
	} else {
		n.Repl = p.interpString(repl, t)
	}
	n.SetSpan(t.Pos, endOf(t))
	return n
}

func (p *parser) transFromToken(t token.Token) *ast.Trans {
	n := &ast.Trans{}
	n.SearchList = rawBody(t)
	if len(t.Parts) > 1 {
		n.ReplList = t.Parts[1]
	}
	n.Mods = t.Mods
	n.SetSpan(t.Pos, endOf(t))
	return n
}

// ---------------------------------------------------------------------------
// Barewords, calls, builtins

// namedUnary lists builtins that take at most one term-precedence argument.
var namedUnary = map[string]bool{
	"defined": true, "ref": true, "scalar": true, "length": true,
	"lc": true, "uc": true, "lcfirst": true, "ucfirst": true, "fc": true,
	"chr": true, "ord": true, "int": true, "hex": true, "oct": true,
	"abs": true, "sqrt": true, "log": true, "exp": true, "sin": true, "cos": true,
	"quotemeta": true, "readlink": true, "rmdir": true, "stat": true, "lstat": true,
	"exists": true, "delete": true, "each": true, "keys": true, "values": true,
	"rand": true, "srand": true, "localtime": true, "gmtime": true, "caller": true,
	"sleep": true, "umask": true, "chroot": true, "fileno": true, "getpgrp": true,
	"pos": true, "readdir": true, "closedir": true, "rewinddir": true, "telldir": true,
	"chdir": true, "shift": true, "pop": true, "undef": true, "study": true,
	"eof": true, "tell": true, "wantarray": true, "chomp": false, // chomp is a list op
}

// nullary builtins that never take arguments.
var nullary = map[string]bool{
	"wantarray": true, "time": true, "times": true, "wait": true, "getppid": true,
	"__PACKAGE__": true, "__FILE__": true, "__LINE__": true, "__SUB__": true,
}

var builtinNames = map[string]bool{}

func init() {
	for _, s := range []string{
		"print", "printf", "say", "push", "unshift", "pop", "shift", "splice",
		"split", "join", "sprintf", "sort", "map", "grep", "reverse", "die",
		"warn", "open", "close", "read", "seek", "tell", "binmode", "eof",
		"chomp", "chop", "chmod", "chown", "unlink", "mkdir", "rmdir", "rename",
		"utime", "kill", "system", "exec", "waitpid", "pack", "unpack", "exit",
		"defined", "ref", "scalar", "length", "lc", "uc", "lcfirst", "ucfirst",
		"chr", "ord", "int", "hex", "oct", "abs", "sqrt", "log", "exp", "sin",
		"cos", "atan2", "quotemeta", "exists", "delete", "each", "keys", "values",
		"rand", "srand", "localtime", "gmtime", "time", "times", "sleep", "umask",
		"wantarray", "caller", "bless", "local", "my", "our", "return", "last",
		"next", "redo", "goto", "sub", "do", "eval", "require", "use", "no",
		"index", "rindex", "substr", "sprintf", "lstat", "stat", "opendir",
		"readdir", "closedir", "rewinddir", "telldir", "seekdir", "chdir",
		"glob", "readlink", "symlink", "link", "fileno", "flock", "truncate",
		"select", "fc", "study", "pos", "undef", "tie", "untie", "tied",
	} {
		builtinNames[s] = true
	}
}

func isBuiltin(s string) bool { return builtinNames[s] }

// blockFuncs are library functions conventionally called as `f { ... } LIST`.
// Perl allows this because their prototypes start with &; we recognise the
// well-known ones by name so a script that uses them without declaring a
// prototype in the same file still parses.
var blockFuncs = map[string]bool{
	"first": true, "any": true, "all": true, "none": true, "notall": true,
	"reduce": true, "reductions": true, "pairmap": true, "pairgrep": true,
	"pairfirst": true, "max_by": true, "min_by": true, "sort_by": true,
	"nsort_by": true, "uniq_by": true, "partition_by": true,
	"try": true, "catch": true, "finally": true,
}

// takesBlock reports whether a bareword call may be followed by a block
// argument.
func (p *parser) takesBlock(name string) bool {
	if blockFuncs[name] {
		return true
	}
	if proto, ok := p.subProtos[name]; ok && strings.HasPrefix(proto, "&") {
		return true
	}
	return false
}

// parseIdentTerm parses a term that begins with a bareword.
func (p *parser) parseIdentTerm() ast.Expr {
	t := p.cur()
	start := t.Pos
	name := t.Text

	switch name {
	case "my", "our", "local", "state":
		return p.parseMy()
	case "sub":
		p.next()
		body := p.parseBlockBody()
		n := &ast.AnonSub{Body: body}
		setSpan(n, start, p.prevEnd())
		return n
	case "do", "eval":
		if p.peekAt(1).Kind == token.LBrace {
			p.next()
			body := p.parseBlockBody()
			n := &ast.Call{Name: name, Block: body}
			setSpan(n, start, p.prevEnd())
			return n
		}
		if name == "eval" {
			p.next()
			var args []ast.Expr
			if p.startsTerm() {
				args = []ast.Expr{p.parseExpr(precNamedUnary)}
			}
			n := &ast.Call{Name: "eval", Args: args}
			setSpan(n, start, p.prevEnd())
			return n
		}
		// do EXPR (do FILE) is rare; treat as call.
		p.next()
		e := p.parseExpr(precNamedUnary)
		n := &ast.Call{Name: "do", Args: []ast.Expr{e}}
		setSpan(n, start, p.prevEnd())
		return n
	case "sort", "map", "grep":
		return p.parseSortMapGrep()
	case "print", "printf", "say":
		return p.parsePrint()
	case "return":
		p.next()
		var args []ast.Expr
		if p.startsTerm() {
			args = p.parseCommaList(token.Semi)
		}
		n := &ast.Call{Name: "return", Args: args}
		setSpan(n, start, p.prevEnd())
		return n
	case "last", "next", "redo":
		p.next()
		n := &ast.Call{Name: name}
		if p.kind() == token.Ident && isLabelName(p.cur().Text) {
			n.Args = []ast.Expr{p.bareword(p.cur())}
			p.next()
		}
		setSpan(n, start, p.prevEnd())
		return n
	}

	if nullary[name] {
		p.next()
		n := &ast.Call{Name: name}
		// These take no arguments, but writing the empty parentheses is
		// common and legal: wantarray(), time().
		if p.kind() == token.LParen && p.peekAt(1).Kind == token.RParen {
			p.next()
			p.next()
			n.Paren = true
		}
		setSpan(n, start, p.prevEnd())
		return n
	}

	// NAME BLOCK LIST: the List::Util callback shape, and user subs declared
	// with a leading & prototype.
	if p.peekAt(1).Kind == token.LBrace && p.takesBlock(name) {
		p.next()
		body := p.parseBlockBody()
		n := &ast.Call{Name: name, Block: body}
		p.accept(token.Comma)
		if p.startsTerm() {
			n.Args = p.parseCommaList(token.Semi)
		}
		setSpan(n, start, p.prevEnd())
		return n
	}

	p.next()

	// Class->method: bareword (possibly Foo::Bar) directly before ->.
	if p.kind() == token.Arrow {
		bw := &ast.FileHandle{Name: name}
		setSpan(bw, start, endOf(t))
		return bw // parsePostfix's Arrow case builds the MethodCall
	}

	// Parenthesised call.
	if p.kind() == token.LParen {
		p.next()
		var args []ast.Expr
		if isFileHandleTaker(name) && p.kind() == token.Ident && looksLikeHandle(p.cur().Text) &&
			(p.peekAt(1).Kind == token.Comma || p.peekAt(1).Kind == token.RParen) {
			fh := &ast.FileHandle{Name: p.cur().Text}
			setSpan(fh, p.cur().Pos, endOf(p.cur()))
			args = append(args, fh)
			p.next()
			p.accept(token.Comma)
		}
		args = append(args, p.parseCommaList(token.RParen)...)
		p.expect(token.RParen, ")")
		n := &ast.Call{Name: name, Args: args, Paren: true}
		setSpan(n, start, p.prevEnd())
		return n
	}

	// Named unary operators bind one argument tightly.
	if v, ok := namedUnary[name]; ok && v {
		var args []ast.Expr
		if p.startsTerm() {
			args = []ast.Expr{p.parseExpr(precNamedUnary)}
		}
		n := &ast.Call{Name: name, Args: args}
		setSpan(n, start, p.prevEnd())
		return n
	}

	// List operator (builtin or user sub) without parens.
	if p.startsTerm() {
		var args []ast.Expr
		if isFileHandleTaker(name) && p.kind() == token.Ident && looksLikeHandle(p.cur().Text) {
			fh := &ast.FileHandle{Name: p.cur().Text}
			setSpan(fh, p.cur().Pos, endOf(p.cur()))
			args = append(args, fh)
			p.next()
			p.accept(token.Comma)
		}
		args = append(args, p.parseCommaList(token.Semi)...)
		n := &ast.Call{Name: name, Args: args}
		setSpan(n, start, p.prevEnd())
		return n
	}

	// Naked bareword: sub call with no args, or a string-ish bareword.
	bw := &ast.Call{Name: name}
	setSpan(bw, start, endOf(t))
	return bw
}

func (p *parser) bareword(t token.Token) ast.Expr {
	n := &ast.StrLit{Value: t.Text}
	setSpan(n, t.Pos, endOf(t))
	return n
}

func isFileHandleTaker(name string) bool {
	switch name {
	case "open", "close", "binmode", "eof", "fileno", "seek", "tell", "read",
		"opendir", "readdir", "closedir", "rewinddir", "telldir", "seekdir",
		"flock", "truncate", "stat", "select":
		return true
	}
	return false
}

// looksLikeHandle: bareword filehandles are conventionally all-caps.
func looksLikeHandle(s string) bool {
	if s == "" || isBuiltin(s) {
		return false
	}
	return strings.ToUpper(s) == s
}

// parseMy parses my/our/local/state declarations as expressions.
func (p *parser) parseMy() ast.Expr {
	t := p.cur()
	start := t.Pos
	p.next()
	n := &ast.My{Keyword: t.Text}
	if p.kind() == token.LParen {
		n.Paren = true
		p.next()
		for p.kind() != token.RParen && p.kind() != token.EOF {
			if p.kind() == token.Comma {
				p.next()
				continue
			}
			n.Vars = append(n.Vars, p.parseTerm())
		}
		p.expect(token.RParen, ")")
	} else {
		// local can take arbitrary lvalues like $SIG{INT} or $h{k}.
		n.Vars = append(n.Vars, p.parseTerm())
	}
	setSpan(n, start, p.prevEnd())
	return n
}

// parsePrint handles print/printf/say with their optional filehandle slot.
func (p *parser) parsePrint() ast.Expr {
	t := p.cur()
	start := t.Pos
	name := t.Text
	p.next()

	n := &ast.Call{Name: name}
	paren := false
	if p.kind() == token.LParen {
		// Could be print(...) or print (LIST) — treat as call parens.
		paren = true
		p.next()
	}

	// Filehandle slot: bareword handle, {$fh} block, or $fh followed by a
	// term with no comma.
	switch {
	case p.kind() == token.Ident && looksLikeHandle(p.cur().Text) &&
		p.peekAt(1).Kind != token.Comma && p.peekAt(1).Kind != token.FatComma &&
		p.peekAt(1).Kind != token.Arrow && !isBinOpNext(p.peekAt(1).Kind):
		fh := &ast.FileHandle{Name: p.cur().Text}
		setSpan(fh, p.cur().Pos, endOf(p.cur()))
		n.Args = append(n.Args, fh)
		p.next()
	case p.kind() == token.LBrace:
		p.next()
		h := p.parseExpr(precLowest)
		p.expect(token.RBrace, "}")
		n.Args = append(n.Args, h)
	case p.kind() == token.ScalarVar && p.peekAt(1).Kind != token.EOF &&
		startsTermKind(p.peekAt(1).Kind) && p.peekAt(1).Kind != token.LBracket &&
		p.peekAt(1).Kind != token.LBrace:
		fh := p.parseVariable()
		n.Args = append(n.Args, fh)
	}

	stop := token.Semi
	if paren {
		stop = token.RParen
	}
	n.Args = append(n.Args, p.parseCommaList(stop)...)
	if paren {
		p.expect(token.RParen, ")")
	}
	n.Paren = paren
	setSpan(n, start, p.prevEnd())
	return n
}

func isBinOpNext(k token.Kind) bool {
	_, ok := binaryOp(k)
	return ok || k == token.Semi || k == token.RParen
}

func startsTermKind(k token.Kind) bool {
	switch k {
	case token.Number, token.Version, token.StrSingle, token.StrDouble, token.Heredoc,
		token.StrBacktick, token.QwList, token.ScalarVar, token.ArrayVar, token.HashVar,
		token.ArrayLen, token.LParen, token.Ident, token.Backslash, token.LBracket:
		return true
	}
	return false
}

// parseSortMapGrep handles sort/map/grep with BLOCK, SUBNAME and EXPR forms.
func (p *parser) parseSortMapGrep() ast.Expr {
	t := p.cur()
	start := t.Pos
	name := t.Text
	p.next()

	n := &ast.Call{Name: name}
	paren := p.accept(token.LParen)
	stop := token.Semi
	if paren {
		stop = token.RParen
	}

	switch {
	case p.kind() == token.LBrace && p.blockNotHash():
		body := p.parseBlockBody()
		n.Block = body
		n.Args = p.parseCommaList(stop)
	case name == "sort" && p.kind() == token.Ident && !isBuiltin(p.cur().Text) &&
		p.peekAt(1).Kind != token.Comma && p.peekAt(1).Kind != token.LParen &&
		startsTermKind(p.peekAt(1).Kind):
		n.SortSub = p.cur().Text
		p.next()
		n.Args = p.parseCommaList(stop)
	default:
		if name == "sort" {
			n.Args = p.parseCommaList(stop)
		} else {
			// map EXPR, LIST
			first := p.parseExpr(precAssign)
			n.Args = append(n.Args, first)
			if p.accept(token.Comma) {
				n.Args = append(n.Args, p.parseCommaList(stop)...)
			}
		}
	}
	if paren {
		p.expect(token.RParen, ")")
	}
	n.Paren = paren
	setSpan(n, start, p.prevEnd())
	return n
}

// blockNotHash decides whether { after map/grep/sort opens a block rather
// than an anonymous hash, using first-token lookahead.
func (p *parser) blockNotHash() bool {
	t1 := p.peekAt(1)
	t2 := p.peekAt(2)
	if t1.Kind == token.Semi {
		return true
	}
	if (t1.Kind == token.Ident || t1.Kind == token.StrSingle || t1.Kind == token.StrDouble) &&
		(t2.Kind == token.FatComma || t2.Kind == token.Comma) {
		return false // hash constructor
	}
	return true
}

// decodeSingleQuoted resolves \\ and \' in a single-quoted body.
func decodeSingleQuoted(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '\\' || s[i+1] == '\'') {
			i++
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}
