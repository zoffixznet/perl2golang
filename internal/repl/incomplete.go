package repl

import (
	"strings"

	"perl2go/internal/perl/lexer"
	"perl2go/internal/perl/token"
)

// pending describes a snippet that is not finished yet: something is open and
// a further line could close it.
//
// The REPL decides when to stop reading from the text itself rather than
// asking the user to signal continuation, because a Perl developer already
// knows when a statement is finished and should not have to say so twice.
type pending struct {
	// what names the open construct in the words a reminder uses, for
	// example "sub body" or "parenthesis".
	what string
	// line is the 1-based line, within the snippet, where it opened.
	line int
}

// continuation reports whether src is an unfinished fragment.
//
// It works from the same lexer the converter uses, so the REPL never accepts a
// fragment the file path would reject. Three things mean "keep reading":
// a quote-like construct that ran off the end of the text, a bracket with no
// partner, and a trailing token that cannot end a statement. A fourth covers
// compound statements whose block has not been opened yet, which is what
// happens the moment somebody types `sub trim` and presses return.
func continuation(src string) (pending, bool) {
	if strings.TrimSpace(src) == "" {
		return pending{}, false
	}
	toks, diags := lexer.Lex([]byte(src))

	// A string, heredoc, pattern or substitution that never closed. In a file
	// this is an error; at a prompt it is somebody halfway through typing.
	for _, d := range diags {
		if what, ok := unterminated(d.Msg); ok {
			return pending{what: what, line: d.Pos.Line}, true
		}
	}

	stack, head, open := scanBrackets(toks)
	if len(stack) > 0 {
		return pending{what: stack[0].what, line: stack[0].line}, true
	}

	// A statement that ends on an operator wants a right-hand side.
	if last, ok := lastSignificant(toks); ok && dangles(last.Kind) {
		return pending{what: "an operator with nothing after it", line: last.Pos.Line}, true
	}

	// `if ($x > 1)` and `sub trim` are complete tokens and an incomplete
	// statement. The leading keyword of the unterminated trailing statement is
	// the only thing that distinguishes them from `print "x" if $y`.
	if block, ok := blockKeyword(head); ok {
		return pending{what: block, line: open}, true
	}
	return pending{}, false
}

// bracket is one unmatched opening delimiter.
type bracket struct {
	what string
	line int
}

// scanBrackets walks the token stream and returns the delimiters still open at
// the end, the leading keyword of the trailing unterminated statement, and the
// line that statement started on.
//
// Braces inside strings, patterns and heredoc bodies never reach here: the
// lexer has already folded each of those into one token, which is why counting
// characters would be wrong and counting tokens is right.
func scanBrackets(toks []token.Token) (stack []bracket, head string, headLine int) {
	// saved holds the enclosing statement's keyword for each open delimiter,
	// so that the `{` in `if (...) {` still knows it belongs to an `if`.
	var saved []string
	var savedLine []int
	started := false

	for _, t := range toks {
		switch t.Kind {
		case token.Comment, token.Pod, token.EOF:
			continue
		}
		switch t.Kind {
		case token.LParen, token.LBracket, token.LBrace:
			stack = append(stack, bracket{what: describe(t.Kind, head), line: t.Pos.Line})
			saved = append(saved, head)
			savedLine = append(savedLine, headLine)
			head, headLine, started = "", 0, false
		case token.RParen, token.RBracket, token.RBrace:
			if n := len(stack); n > 0 {
				stack = stack[:n-1]
				head, headLine = saved[n-1], savedLine[n-1]
				saved, savedLine = saved[:n-1], savedLine[:n-1]
				started = true
			}
			// A closing brace ends the statement that owned the block, so
			// `sub f { ... }` is finished and `sub f {` is not.
			if t.Kind == token.RBrace {
				head, headLine, started = "", 0, false
			}
		case token.Semi:
			head, headLine, started = "", 0, false
		default:
			if !started {
				started = true
				headLine = t.Pos.Line
				if t.Kind == token.Ident {
					head = t.Text
				}
			}
		}
	}
	return stack, head, headLine
}

// describe names an open delimiter for the reminder line. A brace is named
// after the statement that opened it, because "sub body" tells the reader
// where they are and "brace" does not.
func describe(kind token.Kind, head string) string {
	switch kind {
	case token.LParen:
		return "parenthesis"
	case token.LBracket:
		return "bracket"
	}
	switch head {
	case "sub":
		return "sub body"
	case "if", "unless", "while", "until", "for", "foreach", "else", "elsif", "do":
		return head + " block"
	}
	return "block"
}

// blockKeyword reports whether an unterminated statement beginning with head
// is still waiting for its block.
func blockKeyword(head string) (string, bool) {
	switch head {
	case "sub":
		return "sub declaration", true
	case "if", "unless", "while", "until", "for", "foreach", "elsif":
		return head + " statement", true
	}
	return "", false
}

// unterminated maps a lexer diagnostic onto the construct it left open. Only
// the messages that mean "the text ended too early" count; an unexpected
// character is a mistake no further line will fix.
func unterminated(msg string) (string, bool) {
	switch {
	case strings.Contains(msg, "not found before end of file"):
		return "heredoc", true
	case strings.HasPrefix(msg, "unterminated heredoc"):
		return "heredoc", true
	case strings.HasPrefix(msg, "unterminated string"):
		return "string", true
	case strings.HasPrefix(msg, "unterminated pattern"):
		return "pattern", true
	case strings.HasPrefix(msg, "unterminated replacement"),
		strings.HasPrefix(msg, "missing replacement part"):
		return "substitution", true
	case strings.HasPrefix(msg, "unterminated"):
		return "quote-like construct", true
	}
	return "", false
}

// lastSignificant returns the final token that carries meaning.
func lastSignificant(toks []token.Token) (token.Token, bool) {
	for i := len(toks) - 1; i >= 0; i-- {
		switch toks[i].Kind {
		case token.EOF, token.Comment, token.Pod:
			continue
		}
		return toks[i], true
	}
	return token.Token{}, false
}

// dangles reports whether a token cannot be the last thing in a statement.
// Everything here is an operator that needs an operand to its right.
func dangles(k token.Kind) bool {
	switch k {
	case token.Assign, token.OpAssign, token.Arrow, token.FatComma, token.Comma,
		token.DoubleColon, token.Question, token.Backslash,
		token.Plus, token.Minus, token.Star, token.Slash, token.Percent,
		token.StarStar, token.Dot, token.DotDot, token.DotDotDot, token.Repeat,
		token.NumEq, token.NumNe, token.NumLt, token.NumGt, token.NumLe,
		token.NumGe, token.NumCmp,
		token.StrEq, token.StrNe, token.StrLt, token.StrGt, token.StrLe,
		token.StrGe, token.StrCmp,
		token.AndAnd, token.OrOr, token.DefinedOr, token.Not,
		token.AndLow, token.OrLow, token.NotLow, token.XorLow,
		token.BitAnd, token.BitOr, token.BitXor, token.BitNot,
		token.ShiftLeft, token.ShiftRight,
		token.MatchBind, token.NotMatchBind:
		return true
	}
	return false
}
