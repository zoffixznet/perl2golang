// Package lexer turns Perl 5 source text into tokens without executing any
// of it. It is a byte-oriented scanner built around the classic
// expect-term/expect-operator state machine that Perl's own toolchain uses
// to decide what a `/`, `<` or `?` means at any given point.
//
// The lexer is best-effort by design: when it hits something it cannot
// tokenize it records a diagnostic, emits an Illegal token, resynchronises
// at the next line and keeps going, so one bad construct never hides the
// rest of the file.
package lexer

import (
	"fmt"
	"sort"
	"strings"

	"perl2golang/internal/perl/token"
)

// Diag is a lexical diagnostic with a source position.
type Diag struct {
	Pos token.Pos
	Msg string
}

func (d Diag) String() string { return fmt.Sprintf("%s: %s", d.Pos, d.Msg) }

// Lex tokenizes src. The returned slice always ends with an EOF token.
// Heredoc bodies are resolved into their tokens before Lex returns.
func Lex(src []byte) ([]token.Token, []Diag) {
	lx := &lexer{src: src, expect: expectTerm}
	lx.buildLineIndex()
	lx.run()
	return lx.toks, lx.diags
}

type expectState int

const (
	expectTerm     expectState = iota // a value may start here: / is a regex
	expectOperator                    // a value just ended: / is division
)

type braceKind int

const (
	braceBlock braceKind = iota
	braceSubscript
	braceAnonHash
	// braceValueBlock is a block that produces a value: do {...},
	// eval {...}, sub {...}. After its } an operator may follow.
	braceValueBlock
)

type lexer struct {
	src        []byte
	pos        int
	lineStarts []int
	expect     expectState
	toks       []token.Token
	diags      []Diag

	// pending holds indexes into toks of heredoc tokens whose bodies have
	// not been read yet; they are consumed in order at the next newline.
	pending []int

	braces []braceKind
	// lastClose remembers what the most recent } or ] closed, so a
	// following { can be classified as a chained subscript.
	lastCloseSubscript bool

	// prevKind/prevText describe the last significant (non-trivia) token.
	prevKind token.Kind
	prevText string
}

func (lx *lexer) buildLineIndex() {
	lx.lineStarts = append(lx.lineStarts, 0)
	for i, b := range lx.src {
		if b == '\n' {
			lx.lineStarts = append(lx.lineStarts, i+1)
		}
	}
}

func (lx *lexer) posAt(off int) token.Pos {
	line := sort.Search(len(lx.lineStarts), func(i int) bool { return lx.lineStarts[i] > off })
	return token.Pos{Offset: off, Line: line, Col: off - lx.lineStarts[line-1] + 1}
}

func (lx *lexer) errorf(off int, format string, args ...any) {
	lx.diags = append(lx.diags, Diag{Pos: lx.posAt(off), Msg: fmt.Sprintf(format, args...)})
}

func (lx *lexer) emit(t token.Token) {
	lx.toks = append(lx.toks, t)
	if t.Kind != token.Comment && t.Kind != token.Pod {
		lx.prevKind = t.Kind
		lx.prevText = t.Text
	}
}

func (lx *lexer) tok(kind token.Kind, start int) token.Token {
	return token.Token{Kind: kind, Text: string(lx.src[start:lx.pos]), Pos: lx.posAt(start)}
}

func (lx *lexer) emitSimple(kind token.Kind, start int, state expectState) {
	lx.emit(lx.tok(kind, start))
	lx.expect = state
}

func (lx *lexer) byte(i int) byte {
	if i < 0 || i >= len(lx.src) {
		return 0
	}
	return lx.src[i]
}

func (lx *lexer) run() {
	for {
		lx.skipSpace()
		if lx.pos >= len(lx.src) {
			break
		}
		start := lx.pos
		if !lx.scanToken() {
			// Unrecoverable at this byte: skip to end of line.
			lx.errorf(start, "unexpected character %q", lx.src[start])
			for lx.pos < len(lx.src) && lx.src[lx.pos] != '\n' {
				lx.pos++
			}
			lx.emit(lx.tok(token.Illegal, start))
			lx.expect = expectTerm
		}
	}
	if len(lx.pending) > 0 {
		for _, idx := range lx.pending {
			lx.errorf(lx.toks[idx].Pos.Offset, "unterminated heredoc <<%s", lx.toks[idx].Tag)
		}
		lx.pending = nil
	}
	lx.toks = append(lx.toks, token.Token{Kind: token.EOF, Pos: lx.posAt(len(lx.src))})
}

// skipSpace consumes whitespace; crossing a newline triggers consumption of
// any pending heredoc bodies.
func (lx *lexer) skipSpace() {
	for lx.pos < len(lx.src) {
		b := lx.src[lx.pos]
		if b == '\n' {
			lx.pos++
			if len(lx.pending) > 0 {
				lx.readHeredocBodies()
			}
			continue
		}
		if b == ' ' || b == '\t' || b == '\r' || b == '\f' {
			lx.pos++
			continue
		}
		return
	}
}

func (lx *lexer) atLineStart() bool {
	return lx.pos == 0 || lx.src[lx.pos-1] == '\n'
}

// restOfLine returns src from pos through the next newline (inclusive) and
// advances past it.
func (lx *lexer) restOfLine() string {
	start := lx.pos
	for lx.pos < len(lx.src) && lx.src[lx.pos] != '\n' {
		lx.pos++
	}
	if lx.pos < len(lx.src) {
		lx.pos++
	}
	return string(lx.src[start:lx.pos])
}

func isIdentStart(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

func isIdentByte(b byte) bool {
	return isIdentStart(b) || b >= '0' && b <= '9'
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// scanToken lexes one token at lx.pos. It returns false when the byte cannot
// begin any token.
func (lx *lexer) scanToken() bool {
	b := lx.src[lx.pos]

	// POD: only at column 0 where a statement may start.
	if b == '=' && lx.atLineStart() && lx.expect == expectTerm && isIdentStart(lx.byte(lx.pos+1)) {
		lx.scanPod()
		return true
	}

	switch {
	case b == '#':
		lx.scanComment()
		return true
	case b == '$':
		return lx.scanDollar()
	case b == '@':
		return lx.scanAt()
	case b == '%':
		return lx.scanPercent()
	case b == '&':
		return lx.scanAmp()
	case b == '*':
		return lx.scanStar()
	case isDigit(b):
		lx.scanNumber()
		return true
	case b == '.' && isDigit(lx.byte(lx.pos+1)) && lx.expect == expectTerm:
		lx.scanNumber()
		return true
	case isIdentStart(b):
		return lx.scanWord()
	case b == '\'':
		lx.scanDelimited(token.StrSingle, lx.pos, '\'', false)
		return true
	case b == '"':
		lx.scanDelimited(token.StrDouble, lx.pos, '"', true)
		return true
	case b == '`':
		lx.scanDelimited(token.StrBacktick, lx.pos, '`', true)
		return true
	}
	return lx.scanOperator()
}

func (lx *lexer) scanComment() {
	start := lx.pos
	for lx.pos < len(lx.src) && lx.src[lx.pos] != '\n' {
		lx.pos++
	}
	lx.emit(lx.tok(token.Comment, start))
}

func (lx *lexer) scanPod() {
	start := lx.pos
	for {
		line := lx.restOfLine()
		if strings.HasPrefix(line, "=cut") || lx.pos >= len(lx.src) {
			break
		}
	}
	lx.emit(lx.tok(token.Pod, start))
}

// magicPunct is the set of punctuation characters that form a variable when
// preceded by a sigil.
const magicPunct = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`|~"

func (lx *lexer) scanDollar() bool {
	start := lx.pos
	lx.pos++ // $
	b := lx.byte(lx.pos)

	// $# forms: array last-index.
	if b == '#' {
		nb := lx.byte(lx.pos + 1)
		switch {
		case isIdentStart(nb):
			lx.pos++
			lx.scanVarName()
			lx.emitSimple(token.ArrayLen, start, expectOperator)
			return true
		case nb == '{' || nb == '$':
			lx.pos++
			lx.emitSimple(token.ArrayLen, start, expectTerm)
			return true
		default: // plain $# magic var
			lx.pos++
			lx.emitSimple(token.ScalarVar, start, expectOperator)
			return true
		}
	}

	// ${name}, ${^NAME}, or bare-sigil deref ${ expr }.
	if b == '{' {
		if end, ok := lx.braceName(lx.pos + 1); ok {
			lx.pos = end
			lx.emitSimple(token.ScalarVar, start, expectOperator)
			return true
		}
		lx.emitSimple(token.ScalarVar, start, expectTerm) // bare "$"
		return true
	}

	// $$: PID unless it starts a deref chain ($$x, $${..., $$$...).
	if b == '$' {
		nb := lx.byte(lx.pos + 1)
		if isIdentStart(nb) || nb == '{' || nb == '$' {
			lx.emitSimple(token.ScalarVar, start, expectTerm) // bare "$"
			return true
		}
		lx.pos++
		lx.emitSimple(token.ScalarVar, start, expectOperator)
		return true
	}

	// $^X caret vars.
	if b == '^' && lx.byte(lx.pos+1) >= 'A' && lx.byte(lx.pos+1) <= 'Z' {
		lx.pos += 2
		for isIdentByte(lx.byte(lx.pos)) {
			lx.pos++
		}
		lx.emitSimple(token.ScalarVar, start, expectOperator)
		return true
	}

	// $1, $2, ... capture vars.
	if isDigit(b) {
		for isDigit(lx.byte(lx.pos)) {
			lx.pos++
		}
		lx.emitSimple(token.ScalarVar, start, expectOperator)
		return true
	}

	// Ordinary name, possibly package-qualified.
	if isIdentStart(b) {
		lx.scanVarName()
		lx.emitSimple(token.ScalarVar, start, expectOperator)
		return true
	}

	// Punctuation variables.
	if b != 0 && strings.IndexByte(magicPunct, b) >= 0 {
		lx.pos++
		lx.emitSimple(token.ScalarVar, start, expectOperator)
		return true
	}

	lx.emitSimple(token.ScalarVar, start, expectTerm) // lone $, let parser cope
	return true
}

// braceName recognises {name}, {name::name} and {^NAME} immediately after a
// sigil, returning the offset just past the closing brace.
func (lx *lexer) braceName(at int) (int, bool) {
	i := at
	if lx.byte(i) == '^' {
		i++
	}
	if !isIdentStart(lx.byte(i)) {
		return 0, false
	}
	for isIdentByte(lx.byte(i)) || lx.byte(i) == ':' && lx.byte(i+1) == ':' {
		if lx.byte(i) == ':' {
			i += 2
			continue
		}
		i++
	}
	if lx.byte(i) != '}' {
		return 0, false
	}
	return i + 1, true
}

func (lx *lexer) scanAt() bool {
	start := lx.pos
	lx.pos++ // @
	b := lx.byte(lx.pos)
	switch {
	case b == '$' || b == '{':
		lx.emitSimple(token.ArrayVar, start, expectTerm) // bare "@"
	case isIdentStart(b):
		lx.scanVarName()
		lx.emitSimple(token.ArrayVar, start, expectOperator)
	case b == '_' || b == '-' || b == '+':
		lx.pos++
		lx.emitSimple(token.ArrayVar, start, expectOperator)
	default:
		lx.emitSimple(token.ArrayVar, start, expectTerm)
	}
	return true
}

func (lx *lexer) scanPercent() bool {
	if lx.expect == expectOperator {
		return lx.scanOperator()
	}
	start := lx.pos
	lx.pos++ // %
	b := lx.byte(lx.pos)
	switch {
	case b == '$' || b == '{':
		lx.emitSimple(token.HashVar, start, expectTerm) // bare "%"
	case isIdentStart(b):
		lx.scanVarName()
		lx.emitSimple(token.HashVar, start, expectOperator)
	case b == '+' || b == '-' || b == '!':
		lx.pos++
		lx.emitSimple(token.HashVar, start, expectOperator)
	case b == '^' && isIdentStart(lx.byte(lx.pos+1)):
		lx.pos += 2
		lx.scanVarName()
		lx.emitSimple(token.HashVar, start, expectOperator)
	default:
		return lx.scanOperatorAt(start)
	}
	return true
}

func (lx *lexer) scanAmp() bool {
	if lx.expect == expectOperator || lx.byte(lx.pos+1) == '&' {
		return lx.scanOperator()
	}
	start := lx.pos
	lx.pos++ // &
	b := lx.byte(lx.pos)
	switch {
	case b == '$' || b == '{':
		lx.emitSimple(token.FuncVar, start, expectTerm) // bare "&"
	case isIdentStart(b):
		lx.scanVarName()
		lx.emitSimple(token.FuncVar, start, expectOperator)
	default:
		return lx.scanOperatorAt(start)
	}
	return true
}

func (lx *lexer) scanStar() bool {
	if lx.expect == expectOperator {
		return lx.scanOperator()
	}
	start := lx.pos
	lx.pos++ // *
	b := lx.byte(lx.pos)
	switch {
	case b == '{':
		if end, ok := lx.braceName(lx.pos + 1); ok {
			lx.pos = end
			lx.emitSimple(token.GlobVar, start, expectOperator)
			return true
		}
		lx.emitSimple(token.GlobVar, start, expectTerm)
	case isIdentStart(b):
		lx.scanVarName()
		lx.emitSimple(token.GlobVar, start, expectOperator)
	default:
		return lx.scanOperatorAt(start)
	}
	return true
}

// scanIdentBody consumes identifier characters including :: separators.
func (lx *lexer) scanIdentBody() {
	for {
		b := lx.byte(lx.pos)
		if isIdentByte(b) {
			lx.pos++
			continue
		}
		if b == ':' && lx.byte(lx.pos+1) == ':' {
			lx.pos += 2
			continue
		}
		return
	}
}

// scanVarName consumes a variable name after a sigil. Unlike bareword
// scanning it also accepts the archaic ' package separator ($main'var),
// which must never apply to barewords or tr'x'y' would lex wrong.
func (lx *lexer) scanVarName() {
	for {
		lx.scanIdentBody()
		if lx.byte(lx.pos) == '\'' && isIdentStart(lx.byte(lx.pos+1)) && isIdentByte(lx.byte(lx.pos-1)) {
			lx.pos += 2
			continue
		}
		return
	}
}

func (lx *lexer) scanNumber() {
	start := lx.pos
	b := lx.src[lx.pos]
	if b == '0' && (lx.byte(lx.pos+1) == 'x' || lx.byte(lx.pos+1) == 'X') {
		lx.pos += 2
		for isHex(lx.byte(lx.pos)) || lx.byte(lx.pos) == '_' {
			lx.pos++
		}
		lx.emitSimple(token.Number, start, expectOperator)
		return
	}
	if b == '0' && (lx.byte(lx.pos+1) == 'b' || lx.byte(lx.pos+1) == 'B') {
		lx.pos += 2
		for lx.byte(lx.pos) == '0' || lx.byte(lx.pos) == '1' || lx.byte(lx.pos) == '_' {
			lx.pos++
		}
		lx.emitSimple(token.Number, start, expectOperator)
		return
	}
	if b == '0' && (lx.byte(lx.pos+1) == 'o' || lx.byte(lx.pos+1) == 'O') {
		lx.pos += 2
		for lx.byte(lx.pos) >= '0' && lx.byte(lx.pos) <= '7' || lx.byte(lx.pos) == '_' {
			lx.pos++
		}
		lx.emitSimple(token.Number, start, expectOperator)
		return
	}

	digits := func() {
		for isDigit(lx.byte(lx.pos)) || lx.byte(lx.pos) == '_' {
			lx.pos++
		}
	}
	digits()
	dots := 0
	if lx.byte(lx.pos) == '.' && isDigit(lx.byte(lx.pos+1)) {
		dots++
		lx.pos++
		digits()
		// A second dotted group makes it a v-string: 5.36.1.
		for lx.byte(lx.pos) == '.' && isDigit(lx.byte(lx.pos+1)) {
			dots++
			lx.pos++
			digits()
		}
	}
	if dots >= 2 {
		lx.emitSimple(token.Version, start, expectOperator)
		return
	}
	if e := lx.byte(lx.pos); e == 'e' || e == 'E' {
		i := lx.pos + 1
		if lx.byte(i) == '+' || lx.byte(i) == '-' {
			i++
		}
		if isDigit(lx.byte(i)) {
			lx.pos = i
			digits()
		}
	}
	lx.emitSimple(token.Number, start, expectOperator)
}

func isHex(b byte) bool {
	return isDigit(b) || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

// quoteOps names the quote-like operators.
var quoteOps = map[string]bool{
	"q": true, "qq": true, "qw": true, "qx": true,
	"m": true, "s": true, "y": true, "tr": true, "qr": true,
}

// termKeywords leave the lexer expecting a term, so a following / begins a
// regex. This covers control keywords, named unary and list operators.
var termKeywords = map[string]bool{
	"if": true, "elsif": true, "unless": true, "else": true, "while": true,
	"until": true, "for": true, "foreach": true, "do": true, "sub": true,
	"my": true, "our": true, "local": true, "state": true, "return": true,
	"print": true, "printf": true, "say": true, "push": true, "pop": true,
	"shift": true, "unshift": true, "splice": true, "split": true,
	"join": true, "grep": true, "map": true, "sort": true, "reverse": true,
	"keys": true, "values": true, "each": true, "delete": true,
	"exists": true, "defined": true, "scalar": true, "ref": true,
	"bless": true, "die": true, "warn": true, "eval": true, "lc": true,
	"uc": true, "lcfirst": true, "ucfirst": true, "fc": true, "length": true,
	"substr": true, "index": true, "rindex": true, "sprintf": true,
	"chomp": true, "chop": true, "chr": true, "ord": true, "abs": true,
	"int": true, "sqrt": true, "log": true, "exp": true, "sin": true,
	"cos": true, "atan2": true, "hex": true, "oct": true, "rand": true,
	"srand": true, "open": true, "close": true, "read": true,
	"binmode": true, "eof": true, "seek": true, "tell": true,
	"unlink": true, "mkdir": true, "rmdir": true, "opendir": true,
	"readdir": true, "closedir": true, "chdir": true, "stat": true,
	"lstat": true, "rename": true, "system": true, "exec": true,
	"exit": true, "sleep": true, "localtime": true, "gmtime": true,
	"last": true, "next": true, "redo": true, "goto": true,
	"require": true, "use": true, "no": true, "package": true,
	"wait": true, "waitpid": true, "kill": true, "sprintf ": false,
	"lock": true, "chmod": true, "chown": true, "glob": true,
	"readline": true, "select": true, "pos": true, "quotemeta": true,
	"sprintf\t": false, "uc\t": false, "wantarray": true, "caller": true,
}

// wordOps maps identifier-shaped operators to their kinds when the lexer
// expects an operator.
var wordOps = map[string]token.Kind{
	"eq": token.StrEq, "ne": token.StrNe, "lt": token.StrLt,
	"gt": token.StrGt, "le": token.StrLe, "ge": token.StrGe,
	"cmp": token.StrCmp, "and": token.AndLow, "or": token.OrLow,
	"xor": token.XorLow, "x": token.Repeat,
}

func (lx *lexer) scanWord() bool {
	start := lx.pos

	// "x3" after a value is the repetition operator followed by a count.
	if lx.expect == expectOperator && lx.src[lx.pos] == 'x' && isDigit(lx.byte(lx.pos+1)) &&
		lx.prevKind != token.Arrow {
		lx.pos++
		lx.emitSimple(token.Repeat, start, expectTerm)
		return true
	}

	// __END__ / __DATA__ on their own line.
	if lx.atLineStart() {
		rest := lx.src[lx.pos:]
		for _, marker := range []string{"__END__", "__DATA__"} {
			if strings.HasPrefix(string(rest), marker) {
				after := lx.byte(lx.pos + len(marker))
				if after == 0 || after == '\n' || after == '\r' {
					lx.pos = len(lx.src)
					t := lx.tok(token.Data, start)
					body := t.Text
					if i := strings.IndexByte(body, '\n'); i >= 0 {
						t.Parts = []string{body[i+1:]}
					} else {
						t.Parts = []string{""}
					}
					t.Tag = marker
					lx.emit(t)
					return true
				}
			}
		}
	}

	lx.scanIdentBody()
	word := string(lx.src[start:lx.pos])

	// Identifier-shaped operators when a value just ended.
	if lx.expect == expectOperator && lx.prevKind != token.Arrow {
		if kind, ok := wordOps[word]; ok {
			if word == "x" && lx.byte(lx.pos) == '=' && lx.byte(lx.pos+1) != '=' {
				lx.pos++
				lx.emitSimple(token.OpAssign, start, expectTerm)
				return true
			}
			lx.emitSimple(kind, start, expectTerm)
			return true
		}
	}
	if word == "not" {
		lx.emitSimple(token.NotLow, start, expectTerm)
		return true
	}

	// Quote-like operators, unless this position makes them plain names.
	// A bareword directly before one is a list operator (`use Foo qw(a b)`,
	// `mysub qq{x}`), so operator position after an identifier still allows
	// a quote-like construct to start.
	quotePos := lx.expect == expectTerm || lx.prevKind == token.Ident
	if quoteOps[word] && quotePos && lx.prevKind != token.Arrow && lx.prevText != "sub" {
		if delim, at, ok := lx.quoteDelimiter(); ok {
			lx.scanQuoteLike(word, start, delim, at)
			return true
		}
	}

	// v-strings: v5, v5.36.0.
	if word[0] == 'v' && len(word) > 1 && allDigits(word[1:]) && lx.byte(lx.pos) == '.' {
		for lx.byte(lx.pos) == '.' && isDigit(lx.byte(lx.pos+1)) {
			lx.pos++
			for isDigit(lx.byte(lx.pos)) {
				lx.pos++
			}
		}
		lx.emitSimple(token.Version, start, expectOperator)
		return true
	}

	lx.emit(lx.tok(token.Ident, start))
	if termKeywords[word] {
		lx.expect = expectTerm
	} else {
		lx.expect = expectOperator
	}
	return true
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return len(s) > 0
}

// quoteDelimiter decides whether the just-scanned quote-op word is followed
// by a usable delimiter. It returns the delimiter and its offset.
func (lx *lexer) quoteDelimiter() (byte, int, bool) {
	i := lx.pos
	sawSpace := false
	for lx.byte(i) == ' ' || lx.byte(i) == '\t' || lx.byte(i) == '\n' {
		sawSpace = true
		i++
	}
	b := lx.byte(i)
	if b == 0 {
		return 0, 0, false
	}
	// Hash-key and fat-comma positions: q => 1, { q => 1 }, $h{q}.
	if b == '=' && lx.byte(i+1) == '>' {
		return 0, 0, false
	}
	if b == '}' && lx.prevKind == token.LBrace {
		return 0, 0, false
	}
	if b == '=' && (lx.byte(i+1) == '=' || lx.byte(i+1) == '~' || sawSpace) {
		return 0, 0, false
	}
	if b == ',' || b == ';' || b == ')' || b == ']' {
		return 0, 0, false
	}
	if b == '#' && sawSpace {
		return 0, 0, false // comment, not delimiter
	}
	if isIdentByte(b) {
		if !sawSpace {
			return 0, 0, false // part of a longer word was already split off
		}
		return b, i, true // alphanumeric delimiter: q qhelloq
	}
	return b, i, true
}

var pairedDelim = map[byte]byte{'(': ')', '[': ']', '{': '}', '<': '>'}

// scanQuoteLike lexes a q/qq/qw/qx/m/s/y/tr/qr construct whose delimiter is
// at offset at.
func (lx *lexer) scanQuoteLike(op string, start int, delim byte, at int) {
	lx.pos = at
	part1, ok := lx.scanDelimitedPart(delim)
	if !ok {
		lx.errorf(start, "unterminated %s%c...%c construct", op, delim, closerOf(delim))
		lx.emit(lx.tok(token.Illegal, start))
		lx.expect = expectTerm
		return
	}

	threePart := op == "s" || op == "tr" || op == "y"
	var part2 string
	if threePart {
		if _, paired := pairedDelim[delim]; paired {
			// Bracketing delimiter: a second, possibly different,
			// delimited part follows after optional space/comments.
			lx.skipSpaceAndComments()
			d2 := lx.byte(lx.pos)
			if d2 == 0 {
				lx.errorf(start, "missing replacement part of %s construct", op)
				lx.emit(lx.tok(token.Illegal, start))
				return
			}
			p2, ok2 := lx.scanDelimitedPart(d2)
			if !ok2 {
				lx.errorf(start, "unterminated replacement in %s construct", op)
				lx.emit(lx.tok(token.Illegal, start))
				return
			}
			part2 = p2
		} else {
			// Same delimiter: the closer of part one opens part two.
			lx.pos-- // reuse the closing delimiter as the opener
			p2, ok2 := lx.scanDelimitedPart(delim)
			if !ok2 {
				lx.errorf(start, "unterminated replacement in %s construct", op)
				lx.emit(lx.tok(token.Illegal, start))
				return
			}
			part2 = p2
		}
	}

	mods := lx.scanModifiers()

	t := lx.tok(kindOfQuoteOp(op), start)
	t.Mods = mods
	t.Delim = delim
	switch op {
	case "s", "tr", "y":
		t.Parts = []string{part1, part2}
	default:
		t.Parts = []string{part1}
	}
	t.Interp = quoteInterpolates(op, delim, mods)
	lx.emit(t)
	lx.expect = expectOperator
}

func closerOf(delim byte) byte {
	if c, ok := pairedDelim[delim]; ok {
		return c
	}
	return delim
}

func kindOfQuoteOp(op string) token.Kind {
	switch op {
	case "q":
		return token.StrSingle
	case "qq":
		return token.StrDouble
	case "qx":
		return token.StrBacktick
	case "qw":
		return token.QwList
	case "m":
		return token.Match
	case "s":
		return token.Substitute
	case "y", "tr":
		return token.Transliterate
	case "qr":
		return token.QuoteRegex
	}
	return token.Illegal
}

func quoteInterpolates(op string, delim byte, mods string) bool {
	if delim == '\'' {
		return false
	}
	switch op {
	case "q", "qw", "tr", "y":
		return false
	}
	return true
}

// scanDelimitedPart consumes a delimited body starting at the opening
// delimiter at lx.pos and returns the raw body. The scan understands only
// backslash escapes and, for bracketing delimiters, nesting. When the
// delimiter is a backslash nothing at all is skipped.
func (lx *lexer) scanDelimitedPart(delim byte) (string, bool) {
	open := delim
	close := closerOf(delim)
	lx.pos++ // opening delimiter
	bodyStart := lx.pos
	depth := 1
	for lx.pos < len(lx.src) {
		b := lx.src[lx.pos]
		if b == '\\' && delim != '\\' {
			lx.pos += 2
			continue
		}
		if b == close {
			depth--
			if depth == 0 {
				body := string(lx.src[bodyStart:lx.pos])
				lx.pos++
				return body, true
			}
		} else if b == open && open != close {
			depth++
		}
		lx.pos++
	}
	return "", false
}

func (lx *lexer) skipSpaceAndComments() {
	for lx.pos < len(lx.src) {
		b := lx.src[lx.pos]
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			lx.pos++
			continue
		}
		if b == '#' {
			for lx.pos < len(lx.src) && lx.src[lx.pos] != '\n' {
				lx.pos++
			}
			continue
		}
		return
	}
}

func (lx *lexer) scanModifiers() string {
	start := lx.pos
	for {
		b := lx.byte(lx.pos)
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' {
			lx.pos++
			continue
		}
		return string(lx.src[start:lx.pos])
	}
}

// scanDelimited lexes a plain '...', "..." or `...` string.
func (lx *lexer) scanDelimited(kind token.Kind, start int, delim byte, interp bool) {
	body, ok := lx.scanDelimitedPart(delim)
	if !ok {
		lx.errorf(start, "unterminated string starting with %c", delim)
		lx.emit(lx.tok(token.Illegal, start))
		return
	}
	t := lx.tok(kind, start)
	t.Parts = []string{body}
	t.Interp = interp
	lx.emit(t)
	lx.expect = expectOperator
}

var fileTestLetters = "erwxoRWXOfdlpSbcugktTBACMsz"

func (lx *lexer) scanOperator() bool { return lx.scanOperatorAt(lx.pos) }

// scanOperatorAt lexes operators and punctuation. start may be behind
// lx.pos when a sigil scan fell through.
func (lx *lexer) scanOperatorAt(start int) bool {
	lx.pos = start
	b := lx.src[lx.pos]
	nb := lx.byte(lx.pos + 1)

	two := string(lx.src[lx.pos:min(lx.pos+2, len(lx.src))])
	three := string(lx.src[lx.pos:min(lx.pos+3, len(lx.src))])

	// Filetest operators in term position: -e "file".
	if b == '-' && lx.expect == expectTerm &&
		strings.IndexByte(fileTestLetters, nb) >= 0 && !isIdentByte(lx.byte(lx.pos+2)) {
		lx.pos += 2
		lx.emitSimple(token.FileTest, start, expectTerm)
		return true
	}

	// Heredocs and readline/glob in term position.
	if b == '<' && lx.expect == expectTerm {
		if lx.scanAngle(start) {
			return true
		}
	}

	// / in term position starts a match. After an unknown bareword perl's
	// answer depends on prototypes; without them we read it as division
	// and say so.
	if b == '/' && lx.expect == expectTerm {
		lx.scanBareMatch(start)
		return true
	}
	if b == '/' && nb != '/' && lx.prevKind == token.Ident &&
		!termKeywords[lx.prevText] && !quoteOps[lx.prevText] {
		lx.errorf(start, "ambiguous / after bareword %q: assuming division; a prototype could make it a pattern", lx.prevText)
	}

	switch three {
	case "**=", "||=", "&&=", "//=", "<<=", ">>=", "...", "<=>":
		lx.pos += 3
		switch three {
		case "...":
			lx.emitSimple(token.DotDotDot, start, expectTerm)
		case "<=>":
			lx.emitSimple(token.NumCmp, start, expectTerm)
		default:
			lx.emitSimple(token.OpAssign, start, expectTerm)
		}
		return true
	}

	switch two {
	case "=>":
		lx.pos += 2
		lx.emitSimple(token.FatComma, start, expectTerm)
		return true
	case "=~":
		lx.pos += 2
		lx.emitSimple(token.MatchBind, start, expectTerm)
		return true
	case "!~":
		lx.pos += 2
		lx.emitSimple(token.NotMatchBind, start, expectTerm)
		return true
	case "==":
		lx.pos += 2
		lx.emitSimple(token.NumEq, start, expectTerm)
		return true
	case "!=":
		lx.pos += 2
		lx.emitSimple(token.NumNe, start, expectTerm)
		return true
	case "<=":
		lx.pos += 2
		lx.emitSimple(token.NumLe, start, expectTerm)
		return true
	case ">=":
		lx.pos += 2
		lx.emitSimple(token.NumGe, start, expectTerm)
		return true
	case "->":
		lx.pos += 2
		lx.emitSimple(token.Arrow, start, expectTerm)
		return true
	case "++":
		lx.pos += 2
		// Postfix after a value keeps operator state; prefix expects a term.
		if lx.expect == expectOperator {
			lx.emitSimple(token.PlusPlus, start, expectOperator)
		} else {
			lx.emitSimple(token.PlusPlus, start, expectTerm)
		}
		return true
	case "--":
		lx.pos += 2
		if lx.expect == expectOperator {
			lx.emitSimple(token.MinusMinus, start, expectOperator)
		} else {
			lx.emitSimple(token.MinusMinus, start, expectTerm)
		}
		return true
	case "**":
		lx.pos += 2
		lx.emitSimple(token.StarStar, start, expectTerm)
		return true
	case "&&":
		lx.pos += 2
		lx.emitSimple(token.AndAnd, start, expectTerm)
		return true
	case "||":
		lx.pos += 2
		lx.emitSimple(token.OrOr, start, expectTerm)
		return true
	case "//":
		if lx.expect == expectTerm {
			lx.scanBareMatch(start) // empty pattern match
			return true
		}
		lx.pos += 2
		lx.emitSimple(token.DefinedOr, start, expectTerm)
		return true
	case "..":
		lx.pos += 2
		lx.emitSimple(token.DotDot, start, expectTerm)
		return true
	case "<<":
		lx.pos += 2
		lx.emitSimple(token.ShiftLeft, start, expectTerm)
		return true
	case ">>":
		lx.pos += 2
		lx.emitSimple(token.ShiftRight, start, expectTerm)
		return true
	case "::":
		lx.pos += 2
		lx.emitSimple(token.DoubleColon, start, expectTerm)
		return true
	}

	if nb == '=' && strings.IndexByte("+-*/.%|&^x", b) >= 0 && lx.byte(lx.pos+2) != '=' {
		// Compound assignment: += -= *= /= .= %= |= &= ^= x=
		if !(b == '/' && lx.expect == expectTerm) {
			lx.pos += 2
			lx.emitSimple(token.OpAssign, start, expectTerm)
			return true
		}
	}

	simple := map[byte]struct {
		kind  token.Kind
		state expectState
	}{
		'+':  {token.Plus, expectTerm},
		'-':  {token.Minus, expectTerm},
		'*':  {token.Star, expectTerm},
		'/':  {token.Slash, expectTerm},
		'%':  {token.Percent, expectTerm},
		'.':  {token.Dot, expectTerm},
		',':  {token.Comma, expectTerm},
		';':  {token.Semi, expectTerm},
		'!':  {token.Not, expectTerm},
		'~':  {token.BitNot, expectTerm},
		'?':  {token.Question, expectTerm},
		':':  {token.Colon, expectTerm},
		'=':  {token.Assign, expectTerm},
		'<':  {token.NumLt, expectTerm},
		'>':  {token.NumGt, expectTerm},
		'&':  {token.BitAnd, expectTerm},
		'|':  {token.BitOr, expectTerm},
		'^':  {token.BitXor, expectTerm},
		'\\': {token.Backslash, expectTerm},
	}
	if ent, ok := simple[b]; ok {
		lx.pos++
		lx.emitSimple(ent.kind, start, ent.state)
		return true
	}

	switch b {
	case '(':
		lx.pos++
		lx.emitSimple(token.LParen, start, expectTerm)
		return true
	case ')':
		lx.pos++
		lx.emitSimple(token.RParen, start, expectOperator)
		return true
	case '[':
		lx.pos++
		lx.emitSimple(token.LBracket, start, expectTerm)
		return true
	case ']':
		lx.pos++
		lx.lastCloseSubscript = true
		lx.emitSimple(token.RBracket, start, expectOperator)
		return true
	case '{':
		kind := lx.classifyBrace()
		lx.braces = append(lx.braces, kind)
		lx.pos++
		lx.emitSimple(token.LBrace, start, expectTerm)
		return true
	case '}':
		var kind braceKind
		if len(lx.braces) > 0 {
			kind = lx.braces[len(lx.braces)-1]
			lx.braces = lx.braces[:len(lx.braces)-1]
		}
		lx.pos++
		lx.lastCloseSubscript = kind == braceSubscript
		if kind == braceBlock {
			lx.emitSimple(token.RBrace, start, expectTerm)
		} else {
			// Subscripts, anonymous hashes and value blocks (do/eval/
			// sub) all leave a value behind.
			lx.emitSimple(token.RBrace, start, expectOperator)
		}
		return true
	}
	return false
}

// classifyBrace decides whether a { opens a subscript/deref, an anonymous
// hash, or a code block, going by the preceding token. Only the expect state
// after the matching } depends on this.
func (lx *lexer) classifyBrace() braceKind {
	switch lx.prevKind {
	case token.Arrow:
		return braceSubscript
	case token.ScalarVar, token.ArrayVar, token.HashVar, token.FuncVar,
		token.GlobVar, token.ArrayLen:
		return braceSubscript
	case token.RBrace, token.RBracket:
		if lx.lastCloseSubscript {
			return braceSubscript
		}
	case token.Assign, token.OpAssign, token.LParen, token.LBracket,
		token.Comma, token.FatComma, token.Question, token.Colon:
		return braceAnonHash
	case token.Ident:
		switch lx.prevText {
		case "do", "eval", "sub":
			return braceValueBlock
		}
	}
	return braceBlock
}

// scanBareMatch lexes /pattern/mods at term position.
func (lx *lexer) scanBareMatch(start int) {
	body, ok := lx.scanDelimitedPart('/')
	if !ok {
		lx.errorf(start, "unterminated pattern match")
		lx.emit(lx.tok(token.Illegal, start))
		return
	}
	mods := lx.scanModifiers()
	t := lx.tok(token.Match, start)
	t.Parts = []string{body}
	t.Mods = mods
	t.Delim = '/'
	t.Interp = true
	lx.emit(t)
	lx.expect = expectOperator
}

// scanAngle handles <, <<HEREDOC, <FH>, <$fh>, <>, <<>> and <*.glob> in term
// position. Returns false when the construct is really a comparison.
func (lx *lexer) scanAngle(start int) bool {
	// <<>> secure diamond.
	if strings.HasPrefix(string(lx.src[lx.pos:]), "<<>>") {
		lx.pos += 4
		lx.emitSimple(token.Readline, start, expectOperator)
		return true
	}

	if lx.byte(lx.pos+1) == '<' {
		if lx.scanHeredocMarker(start) {
			return true
		}
		return false // let operator table emit ShiftLeft
	}

	// Find > on the same line.
	end := -1
	for i := lx.pos + 1; i < len(lx.src); i++ {
		c := lx.src[i]
		if c == '\n' {
			break
		}
		if c == '>' {
			end = i
			break
		}
	}
	if end < 0 {
		return false
	}
	content := string(lx.src[lx.pos+1 : end])
	if isReadlineContent(content) {
		lx.pos = end + 1
		lx.emitSimple(token.Readline, start, expectOperator)
		return true
	}
	if content != "" && !strings.ContainsAny(content, " \t") {
		lx.pos = end + 1
		lx.emitSimple(token.Glob, start, expectOperator)
		return true
	}
	return false
}

func isReadlineContent(s string) bool {
	if s == "" {
		return true
	}
	i := 0
	if s[0] == '$' {
		i = 1
	}
	if i == len(s) {
		return i == 1 // "<$>" is not a readline; "<>" handled above
	}
	for ; i < len(s); i++ {
		if !isIdentByte(s[i]) && s[i] != ':' {
			return false
		}
	}
	return true
}

// scanHeredocMarker recognises <<TAG, <<~TAG, <<"TAG", <<'TAG', <<`TAG`,
// <<\TAG and the same with a space before a quoted tag. A space before a
// bare identifier means left-shift.
func (lx *lexer) scanHeredocMarker(start int) bool {
	i := lx.pos + 2
	indented := false
	if lx.byte(i) == '~' {
		indented = true
		i++
	}
	j := i
	sawSpace := false
	for lx.byte(j) == ' ' || lx.byte(j) == '\t' {
		sawSpace = true
		j++
	}
	b := lx.byte(j)

	var tag string
	interp := true
	switch {
	case b == '"' || b == '\'' || b == '`':
		endQ := j + 1
		for endQ < len(lx.src) && lx.src[endQ] != b && lx.src[endQ] != '\n' {
			endQ++
		}
		if lx.byte(endQ) != b {
			return false
		}
		tag = string(lx.src[j+1 : endQ])
		interp = b != '\''
		lx.pos = endQ + 1
	case b == '\\' && isIdentStart(lx.byte(j+1)):
		j++
		k := j
		for isIdentByte(lx.byte(k)) {
			k++
		}
		tag = string(lx.src[j:k])
		interp = false
		lx.pos = k
	case isIdentStart(b) && !sawSpace:
		k := j
		for isIdentByte(lx.byte(k)) {
			k++
		}
		tag = string(lx.src[j:k])
		lx.pos = k
	default:
		return false
	}

	t := lx.tok(token.Heredoc, start)
	t.Tag = tag
	t.Interp = interp
	t.Indented = indented
	t.Parts = []string{""}
	lx.emit(t)
	lx.pending = append(lx.pending, len(lx.toks)-1)
	lx.expect = expectOperator
	return true
}

// readHeredocBodies consumes the bodies of all pending heredocs starting at
// the current position (just after a newline).
func (lx *lexer) readHeredocBodies() {
	pending := lx.pending
	lx.pending = nil
	for _, idx := range pending {
		t := &lx.toks[idx]
		bodyStart := lx.pos
		var body strings.Builder
		terminated := false
		var termLine string
		for lx.pos < len(lx.src) {
			lineStart := lx.pos
			line := lx.restOfLine()
			trimmed := strings.TrimRight(line, "\n")
			if t.Indented {
				if strings.TrimLeft(trimmed, " \t") == t.Tag {
					termLine = trimmed
					terminated = true
					break
				}
			} else if trimmed == t.Tag {
				terminated = true
				break
			}
			body.WriteString(line)
			_ = lineStart
		}
		if !terminated {
			lx.errorf(t.Pos.Offset, "heredoc terminator %q not found before end of file", t.Tag)
		}
		text := body.String()
		if t.Indented {
			text = dedentHeredoc(lx, t.Pos.Offset, text, strings.TrimSuffix(termLine, t.Tag))
		}
		t.Parts[0] = text
		t.BodyStart = bodyStart
		t.BodyEnd = lx.pos
	}
}

// dedentHeredoc strips the terminator's indentation prefix from each body
// line of a <<~ heredoc.
func dedentHeredoc(lx *lexer, at int, body, prefix string) string {
	if body == "" {
		return body
	}
	lines := strings.SplitAfter(body, "\n")
	for i, line := range lines {
		switch {
		case line == "" || line == "\n":
			// empty line: nothing to strip
		case strings.HasPrefix(line, prefix):
			lines[i] = line[len(prefix):]
		default:
			stripped := strings.TrimLeft(line, " \t")
			if stripped != line {
				lx.errorf(at, "heredoc line does not begin with the terminator's indentation")
			}
			lines[i] = stripped
		}
	}
	return strings.Join(lines, "")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
