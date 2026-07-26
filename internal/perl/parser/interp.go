package parser

import (
	"strconv"
	"strings"

	"perl2go/internal/perl/ast"
	"perl2go/internal/perl/token"
)

// interpString builds a StrLit or InterpLit from a double-quotish body.
func (p *parser) interpString(raw string, t token.Token) ast.Expr {
	parts := p.interpParts(raw, t, true)
	if len(parts) == 1 {
		if s, ok := parts[0].(*ast.StrLit); ok {
			return s
		}
	}
	if len(parts) == 0 {
		n := &ast.StrLit{Value: ""}
		n.SetSpan(t.Pos, endOf(t))
		return n
	}
	n := &ast.InterpLit{Parts: parts}
	n.SetSpan(t.Pos, endOf(t))
	return n
}

// interpParts splits an interpolating body into literal and expression
// parts. When escapes is true, backslash escapes in literal segments are
// resolved (double-quote semantics).
func (p *parser) interpParts(raw string, t token.Token, escapes bool) []ast.Expr {
	var parts []ast.Expr
	var lit strings.Builder

	flushLit := func() {
		if lit.Len() > 0 {
			n := &ast.StrLit{Value: lit.String()}
			n.SetSpan(t.Pos, endOf(t))
			parts = append(parts, n)
			lit.Reset()
		}
	}

	i := 0
	for i < len(raw) {
		c := raw[i]
		if c == '\\' && i+1 < len(raw) {
			if escapes {
				consumed, text := decodeEscape(raw[i:])
				lit.WriteString(text)
				i += consumed
			} else {
				lit.WriteByte(c)
				lit.WriteByte(raw[i+1])
				i += 2
			}
			continue
		}
		if c == '$' || c == '@' {
			end := scanInterpVar(raw, i)
			if end > i {
				expr := raw[i:end]
				e, diags := ParseExprString(expr)
				p.diags = append(p.diags, diags...)
				if e != nil {
					flushLit()
					parts = append(parts, e)
				} else {
					lit.WriteString(expr)
				}
				i = end
				continue
			}
		}
		lit.WriteByte(c)
		i++
	}
	flushLit()
	return parts
}

// regexParts splits a regex body into literal chunks and interpolated
// variables, leaving all regex syntax untouched.
func (p *parser) regexParts(raw string, t token.Token) []ast.Expr {
	var parts []ast.Expr
	var lit strings.Builder
	flushLit := func() {
		if lit.Len() > 0 {
			n := &ast.StrLit{Value: lit.String()}
			n.SetSpan(t.Pos, endOf(t))
			parts = append(parts, n)
			lit.Reset()
		}
	}
	i := 0
	for i < len(raw) {
		c := raw[i]
		if c == '\\' && i+1 < len(raw) {
			lit.WriteByte(c)
			lit.WriteByte(raw[i+1])
			i += 2
			continue
		}
		if c == '$' && !regexDollarIsAnchor(raw, i) || c == '@' && interpStartsVar(raw, i) {
			end := scanInterpVar(raw, i)
			if end > i {
				expr := raw[i:end]
				e, diags := ParseExprString(expr)
				p.diags = append(p.diags, diags...)
				if e != nil {
					flushLit()
					parts = append(parts, e)
					i = end
					continue
				}
			}
		}
		lit.WriteByte(c)
		i++
	}
	flushLit()
	if len(parts) == 0 {
		n := &ast.StrLit{Value: ""}
		n.SetSpan(t.Pos, endOf(t))
		parts = append(parts, n)
	}
	return parts
}

// regexDollarIsAnchor reports whether the $ at i is an end anchor rather
// than an interpolation.
func regexDollarIsAnchor(raw string, i int) bool {
	if i+1 >= len(raw) {
		return true
	}
	c := raw[i+1]
	if c == ')' || c == '|' || c == '$' || c == '\n' || c == ' ' || c == '\t' {
		return true
	}
	// $ before a quantifier or closing class is an anchor too.
	if c == '*' || c == '+' || c == '?' || c == ']' {
		return true
	}
	return !isInterpStartByte(c)
}

func interpStartsVar(raw string, i int) bool {
	if i+1 >= len(raw) {
		return false
	}
	c := raw[i+1]
	return c == '{' || c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isInterpStartByte(c byte) bool {
	switch {
	case c == '{', c == '_', c == '$':
		return true
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	// Punctuation variables that interpolate in double-quoted strings.
	switch c {
	case '!', '&', '`', '\'', '+', '.', ',', ';', '/', '\\', '0':
		return true
	}
	return false
}

// scanInterpVar returns the end offset of the interpolated variable
// expression starting at i (which points at $ or @), or i when the sigil
// does not begin an interpolation.
func scanInterpVar(raw string, i int) int {
	j := i + 1
	if j >= len(raw) {
		return i
	}
	sig := raw[i]
	c := raw[j]

	switch {
	case c == '{':
		// ${name}, ${ expr }, @{[ ... ]}
		depth := 0
		k := j
		for k < len(raw) {
			switch raw[k] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					j = k + 1
					return scanInterpSubscripts(raw, j)
				}
			}
			k++
		}
		return i
	case c == '$':
		// $$name deref, or plain $$ (pid) when nothing follows.
		if j+1 < len(raw) && isNameByte(raw[j+1]) {
			j++
			for j < len(raw) && isNameByte(raw[j]) {
				j++
			}
			return scanInterpSubscripts(raw, j)
		}
		if sig == '$' {
			return j + 1 // $$
		}
		return i
	case c == '#':
		// $#name or $#{...}
		if sig != '$' {
			return i
		}
		j++
		if j < len(raw) && raw[j] == '{' {
			depth := 0
			for k := j; k < len(raw); k++ {
				switch raw[k] {
				case '{':
					depth++
				case '}':
					depth--
					if depth == 0 {
						return k + 1
					}
				}
			}
			return i
		}
		start := j
		j = scanQualifiedName(raw, j)
		if j == start {
			return i
		}
		return j
	case isNameStartByte(c):
		j = scanQualifiedName(raw, j)
		return scanInterpSubscripts(raw, j)
	case c >= '0' && c <= '9':
		if sig != '$' {
			return i
		}
		for j < len(raw) && raw[j] >= '0' && raw[j] <= '9' {
			j++
		}
		return j
	default:
		if sig == '$' && isPunctVarByte(c) {
			return j + 1
		}
		return i
	}
}

// scanQualifiedName scans a variable name, which may be package qualified.
// A single colon is not part of a name, so "$path: " interpolates $path and
// leaves the colon as literal text; only a doubled colon continues the name.
func scanQualifiedName(raw string, j int) int {
	for j < len(raw) {
		c := raw[j]
		if c == ':' {
			if j+1 < len(raw) && raw[j+1] == ':' && j+2 < len(raw) && isNameStartByte(raw[j+2]) {
				j += 2
				continue
			}
			return j
		}
		if !isNameByte(c) {
			return j
		}
		j++
	}
	return j
}

// scanInterpSubscripts extends a variable with [..], {..} and -> chains.
func scanInterpSubscripts(raw string, j int) int {
	for j < len(raw) {
		c := raw[j]
		switch {
		case c == '[':
			k, ok := matchBalanced(raw, j, '[', ']')
			if !ok {
				return j
			}
			j = k
		case c == '{':
			k, ok := matchBalanced(raw, j, '{', '}')
			if !ok {
				return j
			}
			j = k
		case c == '-' && j+1 < len(raw) && raw[j+1] == '>' &&
			j+2 < len(raw) && (raw[j+2] == '[' || raw[j+2] == '{'):
			j += 2
		default:
			return j
		}
	}
	return j
}

func matchBalanced(raw string, i int, open, close byte) (int, bool) {
	depth := 0
	for j := i; j < len(raw); j++ {
		switch raw[j] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return j + 1, true
			}
		}
	}
	return i, false
}

func isNameStartByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isNameByte(c byte) bool {
	return isNameStartByte(c) || (c >= '0' && c <= '9') || c == ':'
}

func isPunctVarByte(c byte) bool {
	switch c {
	case '!', '@', '&', '`', '\'', '+', '.', ',', ';', '/', '\\', '"', '0', '^', '<', '>', '(', ')', '[', ']', '?', '$', '-', '_', '|', '%', '=', '~', ':':
		return true
	}
	return false
}

// decodeEscape resolves one backslash escape in double-quote context,
// returning the number of source bytes consumed and the resulting text.
func decodeEscape(s string) (int, string) {
	if len(s) < 2 {
		return 1, s[:1]
	}
	switch s[1] {
	case 'n':
		return 2, "\n"
	case 't':
		return 2, "\t"
	case 'r':
		return 2, "\r"
	case 'f':
		return 2, "\f"
	case 'b':
		return 2, "\b"
	case 'a':
		return 2, "\a"
	case 'e':
		return 2, "\x1b"
	case '0':
		return 2, "\x00"
	case 'x':
		if len(s) > 3 && s[2] == '{' {
			end := strings.IndexByte(s, '}')
			if end > 0 {
				if v, err := strconv.ParseInt(s[3:end], 16, 32); err == nil {
					return end + 1, string(rune(v))
				}
			}
			return 2, ""
		}
		n := 0
		for n < 2 && len(s) > 2+n && isHexByte(s[2+n]) {
			n++
		}
		if n > 0 {
			v, _ := strconv.ParseInt(s[2:2+n], 16, 32)
			return 2 + n, string(rune(v))
		}
		return 2, ""
	case 'c':
		if len(s) > 2 {
			c := s[2] &^ 0x20 // uppercase
			return 3, string(rune(c ^ 0x40))
		}
		return 2, ""
	case 'Q', 'E', 'L', 'U', 'l', 'u':
		// Case and quotemeta modifiers are dropped from the literal; the
		// converter reports them via analysis when they matter.
		return 2, ""
	default:
		return 2, s[1:2]
	}
}

func isHexByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
