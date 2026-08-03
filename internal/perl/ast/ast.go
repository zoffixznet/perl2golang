// Package ast defines the abstract syntax tree for the subset of Perl 5
// that perl2golang understands. Every node carries source positions so that
// diagnostics, provenance comments, and the conversion report can point
// back at the original code.
package ast

import "perl2golang/internal/perl/token"

// Node is the interface implemented by all AST nodes.
type Node interface {
	Pos() token.Pos
	End() token.Pos
}

// Stmt is implemented by statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is implemented by expression nodes.
type Expr interface {
	Node
	exprNode()
}

type span struct {
	From token.Pos
	To   token.Pos
}

func (s span) Pos() token.Pos { return s.From }
func (s span) End() token.Pos { return s.To }

// SetSpan records the node's source range. It is promoted onto every node
// type via embedding so builders outside this package can set positions.
func (s *span) SetSpan(from, to token.Pos) { s.From, s.To = from, to }

// Program is a whole parsed file.
type Program struct {
	span
	Stmts []Stmt
	// Data holds the raw text following __END__ or __DATA__, if any.
	Data    string
	HasData bool
	// Source is the complete original source, kept for provenance quoting.
	Source string
}

// Comment is a single # comment or a POD block attached to a statement.
type Comment struct {
	span
	Text string // raw text including leading '#' or the whole POD block
	Pod  bool
}

// ---------------------------------------------------------------------------
// Statements

// ExprStmt is an expression evaluated for effect: a call, an assignment...
type ExprStmt struct {
	span
	X Expr
	// Lead are comments on the lines immediately above; Line is a trailing
	// comment on the same line. The same two fields appear on all
	// statements via StmtComments.
	StmtComments
}

// StmtComments carries comments attached to a statement.
type StmtComments struct {
	Lead []*Comment
	Line *Comment
}

// If is an if/elsif/else chain or an unless statement.
type If struct {
	span
	Unless   bool
	Cond     Expr
	Then     []Stmt
	ElseIfs  []ElseIf
	Else     []Stmt
	Modifier bool // came from `EXPR if COND` form
	StmtComments
}

// ElseIf is one elsif clause.
type ElseIf struct {
	Cond Expr
	Then []Stmt
}

// While is while/until, including the modifier forms and do-while.
type While struct {
	span
	Until    bool
	Cond     Expr
	Body     []Stmt
	Label    string
	Modifier bool
	DoWhile  bool // do { ... } while COND: body runs before first test
	StmtComments
}

// ForC is a C-style for (init; cond; post) loop.
type ForC struct {
	span
	Init  Expr // may be nil
	Cond  Expr // may be nil
	Post  Expr // may be nil
	Body  []Stmt
	Label string
	StmtComments
}

// Foreach is foreach/for over a list.
type Foreach struct {
	span
	Var      Expr // loop variable; nil means implicit $_
	MyVar    bool // loop variable declared with my
	List     []Expr
	Body     []Stmt
	Label    string
	Modifier bool
	StmtComments
}

// Block is a bare block.
type Block struct {
	span
	Body  []Stmt
	Label string
	StmtComments
}

// SubDecl is a named subroutine declaration.
type SubDecl struct {
	span
	Name  string
	Proto string // raw prototype text, "" if none
	Body  []Stmt
	StmtComments
}

// PackageDecl is a package statement (block form keeps Body non-nil).
type PackageDecl struct {
	span
	Name string
	Body []Stmt // nil for `package Foo;` statement form
	StmtComments
}

// Use is a use/no statement.
type Use struct {
	span
	No     bool
	Module string // module name, or "" for `use 5.010` style
	Args   []Expr // raw import arguments
	StmtComments
}

// Return is a return statement.
type Return struct {
	span
	Exprs []Expr // empty for bare return
	StmtComments
}

// LoopCtl is last/next/redo.
type LoopCtl struct {
	span
	Op    string // "last", "next", "redo"
	Label string
	StmtComments
}

// Untranslated is a region the parser could not understand. The converter
// reports it and emits a marked placeholder rather than guessing.
type Untranslated struct {
	span
	Reason string
	Raw    string // the raw source of the skipped region
	StmtComments
}

func (*ExprStmt) stmtNode()     {}
func (*If) stmtNode()           {}
func (*While) stmtNode()        {}
func (*ForC) stmtNode()         {}
func (*Foreach) stmtNode()      {}
func (*Block) stmtNode()        {}
func (*SubDecl) stmtNode()      {}
func (*PackageDecl) stmtNode()  {}
func (*Use) stmtNode()          {}
func (*Return) stmtNode()       {}
func (*LoopCtl) stmtNode()      {}
func (*Untranslated) stmtNode() {}

// ---------------------------------------------------------------------------
// Expressions

// NumberLit is a numeric literal; Text preserves the source spelling.
type NumberLit struct {
	span
	Text string
}

// StrLit is a non-interpolating string with its value fully resolved.
type StrLit struct {
	span
	Value string
}

// InterpLit is an interpolating string: a sequence of literal segments and
// embedded expressions, in order.
type InterpLit struct {
	span
	Parts []Expr // StrLit segments interleaved with variable/deref exprs
}

// QwLit is a qw() word list.
type QwLit struct {
	span
	Words []string
}

// Var is a variable: $x, @y, %z, $Foo::bar, and the punctuation variables.
type Var struct {
	span
	Sigil rune   // '$', '@', '%', '&', '*'
	Name  string // without sigil: "x", "Foo::bar", "_", "!", "1", "^W", "#name" for $#name
}

// My declares lexical variables; Perl treats it as an expression.
type My struct {
	span
	Keyword string // "my", "our", "local", "state"
	Vars    []Expr // Var or deref targets; more than one means my ($a, $b)
	Paren   bool   // was written with parentheses (list context assignment)
}

// Assign is assignment, including compound ops. Op is "=" or "+=", ".=", "//=", etc.
type Assign struct {
	span
	Op  string
	LHS Expr
	RHS Expr
}

// BinOp is a binary operator expression; Op is the source operator text
// ("+", ".", "==", "eq", "&&", "and", "..", "x", ...).
type BinOp struct {
	span
	Op string
	L  Expr
	R  Expr
}

// UnOp is a unary operator: !, not, -, +, ~, \, defined, ++/-- (Postfix set).
type UnOp struct {
	span
	Op      string
	X       Expr
	Postfix bool
}

// Ternary is COND ? A : B.
type Ternary struct {
	span
	Cond Expr
	A    Expr
	B    Expr
}

// List is a parenthesised list, possibly empty: (1, 2, 3) or ().
type List struct {
	span
	Elems []Expr
}

// Call is a function or builtin call. Builtins and user subs are not
// distinguished here; analysis resolves that.
type Call struct {
	span
	Name  string
	Args  []Expr
	Amp   bool // called as &name
	Paren bool // written with parentheses
	// Block is a leading block argument for sort/map/grep-style calls.
	Block []Stmt
	// SortSub is a comparator sub name for `sort bynum @list`.
	SortSub string
}

// MethodCall is $obj->method(@args) or Class->method(@args).
type MethodCall struct {
	span
	Invocant Expr
	Method   string // empty when Dynamic is set
	Dynamic  Expr   // $obj->$name(...) form
	Args     []Expr
	Paren    bool
}

// FuncCallRef is invocation through a code ref: $cr->(@args) or &$cr(@args).
type FuncCallRef struct {
	span
	Ref  Expr
	Args []Expr
}

// Index is array element access: $a[i], $r->[i], ${$r}[i].
type Index struct {
	span
	Base  Expr // Var (the @-entity as $-element access) or an expression ref
	Idx   Expr
	Arrow bool // accessed via ->
}

// HashIndex is hash element access: $h{k}, $r->{k}.
type HashIndex struct {
	span
	Base  Expr
	Key   Expr
	Arrow bool
}

// Slice is @a[..] / @h{..} / list slices.
type Slice struct {
	span
	Base Expr
	Idx  []Expr
	Hash bool // hash slice @h{...}
}

// Deref is a dereference: $$r, @$r, %$r, @{expr}, $#{expr}, $#$r.
type Deref struct {
	span
	Sigil rune // '$', '@', '%', '&', '#' (for $#{...})
	X     Expr
}

// RefGen is \expr or \(@list).
type RefGen struct {
	span
	X Expr
}

// AnonArray is [ ... ].
type AnonArray struct {
	span
	Elems []Expr
}

// AnonHash is { ... }.
type AnonHash struct {
	span
	Elems []Expr // flat key, value, key, value list
}

// AnonSub is sub { ... }.
type AnonSub struct {
	span
	Body []Stmt
}

// Match is $x =~ /re/ or a bare /re/ (implicit $_). Bound is nil for bare.
type Match struct {
	span
	Bound   Expr // nil means $_
	Negate  bool // !~
	Pattern *Regex
	// PatternExpr is set instead of Pattern for $x =~ $re forms.
	PatternExpr Expr
}

// Subst is s/// applied to Bound (nil means $_).
type Subst struct {
	span
	Bound   Expr
	Negate  bool
	Pattern *Regex
	// Repl is the replacement: an InterpLit/StrLit normally, or arbitrary
	// expression(s) under /e.
	Repl     Expr
	ReplRaw  string // raw replacement text
	EvalRepl bool   // /e
}

// Trans is tr/// or y/// applied to Bound (nil means $_).
type Trans struct {
	span
	Bound      Expr
	Negate     bool
	SearchList string
	ReplList   string
	Mods       string // cdsr
}

// Regex is a regular expression literal, kept raw plus its parsed
// interpolation parts.
type Regex struct {
	span
	Raw  string // pattern source text exactly as written
	Mods string // modifier letters
	// Parts is the interpolation decomposition (StrLit segments and
	// variable expressions); a single StrLit means no interpolation.
	Parts []Expr
}

// QrExpr is qr//.
type QrExpr struct {
	span
	Pattern *Regex
}

// Readline is <FH>, <$fh>, <>, <STDIN>.
type Readline struct {
	span
	Handle string // "STDIN", "FH", "" for <> and <<>>
	Var    Expr   // non-nil for <$fh>
}

// GlobExpr is <*.pl> or glob("...").
type GlobExpr struct {
	span
	Pattern Expr
}

// FileTest is -e $file and friends.
type FileTest struct {
	span
	Op  byte // 'e', 'f', 'd', ...
	Arg Expr // nil means $_
}

// BacktickCmd is `cmd` or qx//.
type BacktickCmd struct {
	span
	Parts []Expr // interpolation decomposition
}

// FileHandle is a bareword filehandle in argument position (print STDERR ...).
type FileHandle struct {
	span
	Name string
}

// BadExpr is an expression region the parser could not understand.
type BadExpr struct {
	span
	Reason string
	Raw    string
}

func (*NumberLit) exprNode()   {}
func (*StrLit) exprNode()      {}
func (*InterpLit) exprNode()   {}
func (*QwLit) exprNode()       {}
func (*Var) exprNode()         {}
func (*My) exprNode()          {}
func (*Assign) exprNode()      {}
func (*BinOp) exprNode()       {}
func (*UnOp) exprNode()        {}
func (*Ternary) exprNode()     {}
func (*List) exprNode()        {}
func (*Call) exprNode()        {}
func (*MethodCall) exprNode()  {}
func (*FuncCallRef) exprNode() {}
func (*Index) exprNode()       {}
func (*HashIndex) exprNode()   {}
func (*Slice) exprNode()       {}
func (*Deref) exprNode()       {}
func (*RefGen) exprNode()      {}
func (*AnonArray) exprNode()   {}
func (*AnonHash) exprNode()    {}
func (*AnonSub) exprNode()     {}
func (*Match) exprNode()       {}
func (*Subst) exprNode()       {}
func (*Trans) exprNode()       {}
func (*Regex) exprNode()       {}
func (*QrExpr) exprNode()      {}
func (*Readline) exprNode()    {}
func (*GlobExpr) exprNode()    {}
func (*FileTest) exprNode()    {}
func (*BacktickCmd) exprNode() {}
func (*FileHandle) exprNode()  {}
func (*BadExpr) exprNode()     {}
