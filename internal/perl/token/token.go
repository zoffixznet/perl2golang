// Package token defines the lexical tokens of Perl 5 as understood by the
// perl2golang front end, along with source positions.
package token

import "fmt"

// Kind identifies the lexical class of a token.
type Kind int

const (
	EOF Kind = iota
	Illegal

	// Trivia. The lexer keeps these so the pipeline stays lossless; the
	// parser attaches them to statements.
	Comment // # ... to end of line
	Pod     // =pod ... =cut block, raw text

	// Identifiers and literals.
	Ident     // bareword: foo, Foo::Bar, and keywords (if, while, my, sub...)
	Number    // 42, 3.14, 0x1F, 0b101, 017, 1_000, 1e10
	Version   // v5.36 style version string
	ScalarVar // $x, $Foo::bar, $1, $_, $!, ${name}, ${^GLOBAL_PHASE}
	ArrayVar  // @x, @Foo::bar, @{name}
	HashVar   // %x, %Foo::bar
	FuncVar   // &x (code sigil)
	GlobVar   // *x
	ArrayLen  // $#x, $#{expr} lexes as ArrayLen with Text "$#x" or "$#"

	// Quote-like constructs. Text holds the entire raw source of the
	// construct; the Parts/Mods fields carry the decomposition.
	StrSingle     // 'abc', q(abc)
	StrDouble     // "abc", qq(abc)
	StrBacktick   // `cmd`, qx(cmd)
	QwList        // qw(a b c)
	Heredoc       // <<TAG and friends; Parts[0] is the body once resolved
	Match         // /re/, m/re/, ?re?
	Substitute    // s/re/repl/
	Transliterate // tr/a/b/, y/a/b/
	QuoteRegex    // qr/re/

	// Operators and punctuation.
	Assign      // =
	OpAssign    // += -= *= /= .= x= %= **= ||= &&= //= |= &= ^= <<= >>=
	Arrow       // ->
	FatComma    // =>
	Comma       // ,
	Semi        // ;
	Colon       // :
	DoubleColon // :: (only when free-standing; usually part of Ident)
	Question    // ?
	LParen      // (
	RParen      // )
	LBrace      // {
	RBrace      // }
	LBracket    // [
	RBracket    // ]
	Backslash   // \ (reference constructor)

	// Named operator kinds; Text carries the exact operator.
	Plus       // +
	Minus      // -
	Star       // *
	Slash      // /
	Percent    // %
	StarStar   // **
	Dot        // .
	DotDot     // ..
	DotDotDot  // ...
	Repeat     // x
	PlusPlus   // ++
	MinusMinus // --

	NumEq  // ==
	NumNe  // !=
	NumLt  // <
	NumGt  // >
	NumLe  // <=
	NumGe  // >=
	NumCmp // <=>

	StrEq  // eq
	StrNe  // ne
	StrLt  // lt
	StrGt  // gt
	StrLe  // le
	StrGe  // ge
	StrCmp // cmp

	AndAnd    // &&
	OrOr      // ||
	DefinedOr // //
	Not       // !
	AndLow    // and
	OrLow     // or
	NotLow    // not
	XorLow    // xor

	BitAnd     // &
	BitOr      // |
	BitXor     // ^
	BitNot     // ~
	ShiftLeft  // <<
	ShiftRight // >>

	MatchBind    // =~
	NotMatchBind // !~

	Readline // <FH>, <$fh>, <>, <<>>; Text is the raw construct
	Glob     // <*.pl> style glob

	FileTest // -e, -f, -d, ... ; Text is the two-char operator

	Data // __END__ / __DATA__ marker; Parts[0] holds the remainder of the file
)

// Pos is a position in the source, 1-based line and column, 0-based offset.
type Pos struct {
	Offset int
	Line   int
	Col    int
}

func (p Pos) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

// Token is one lexical token.
type Token struct {
	Kind Kind
	Text string // exact source text of the token
	Pos  Pos

	// Quote-like payload. For Match/QuoteRegex: Parts[0] is the pattern.
	// For Substitute: Parts[0] pattern, Parts[1] replacement. For
	// Transliterate: Parts[0] search list, Parts[1] replacement list.
	// For StrSingle/StrDouble/StrBacktick/QwList: Parts[0] is the raw body.
	// For Heredoc: Parts[0] is the body (filled in when the body line is
	// reached), and Tag holds the terminator.
	Parts []string
	Mods  string // trailing modifiers for regex-like constructs (gimsxe...)
	Tag   string // heredoc terminator
	// Delim is the opening delimiter of a quote-like construct, so that
	// consumers can undo the backslash escaping of that delimiter inside
	// the body.
	Delim byte
	// Interp reports whether the construct's body interpolates ("" and qq
	// yes, '' and q no, heredocs by quoting style, s/// replacement by /e).
	Interp bool
	// Indented is true for <<~ heredocs.
	Indented bool
	// BodyStart and BodyEnd delimit, as byte offsets into the source, the
	// raw body of a heredoc (including its terminator line). Both are zero
	// for every other kind. They exist so downstream consumers can quote
	// the original source precisely.
	BodyStart int
	BodyEnd   int
}

func (t Token) String() string {
	return fmt.Sprintf("%s %q at %s", t.Kind, t.Text, t.Pos)
}

var kindNames = map[Kind]string{
	EOF: "EOF", Illegal: "Illegal", Comment: "Comment", Pod: "Pod",
	Ident: "Ident", Number: "Number", Version: "Version",
	ScalarVar: "ScalarVar", ArrayVar: "ArrayVar", HashVar: "HashVar",
	FuncVar: "FuncVar", GlobVar: "GlobVar", ArrayLen: "ArrayLen",
	StrSingle: "StrSingle", StrDouble: "StrDouble", StrBacktick: "StrBacktick",
	QwList: "QwList", Heredoc: "Heredoc", Match: "Match",
	Substitute: "Substitute", Transliterate: "Transliterate", QuoteRegex: "QuoteRegex",
	Assign: "Assign", OpAssign: "OpAssign", Arrow: "Arrow", FatComma: "FatComma",
	Comma: "Comma", Semi: "Semi", Colon: "Colon", DoubleColon: "DoubleColon",
	Question: "Question", LParen: "LParen", RParen: "RParen",
	LBrace: "LBrace", RBrace: "RBrace", LBracket: "LBracket", RBracket: "RBracket",
	Backslash: "Backslash",
	Plus:      "Plus", Minus: "Minus", Star: "Star", Slash: "Slash", Percent: "Percent",
	StarStar: "StarStar", Dot: "Dot", DotDot: "DotDot", DotDotDot: "DotDotDot",
	Repeat: "Repeat", PlusPlus: "PlusPlus", MinusMinus: "MinusMinus",
	NumEq: "NumEq", NumNe: "NumNe", NumLt: "NumLt", NumGt: "NumGt",
	NumLe: "NumLe", NumGe: "NumGe", NumCmp: "NumCmp",
	StrEq: "StrEq", StrNe: "StrNe", StrLt: "StrLt", StrGt: "StrGt",
	StrLe: "StrLe", StrGe: "StrGe", StrCmp: "StrCmp",
	AndAnd: "AndAnd", OrOr: "OrOr", DefinedOr: "DefinedOr", Not: "Not",
	AndLow: "AndLow", OrLow: "OrLow", NotLow: "NotLow", XorLow: "XorLow",
	BitAnd: "BitAnd", BitOr: "BitOr", BitXor: "BitXor", BitNot: "BitNot",
	ShiftLeft: "ShiftLeft", ShiftRight: "ShiftRight",
	MatchBind: "MatchBind", NotMatchBind: "NotMatchBind",
	Readline: "Readline", Glob: "Glob", FileTest: "FileTest", Data: "Data",
}

func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}
