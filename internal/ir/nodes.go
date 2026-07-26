package ir

// File is one emitted Go source file.
type File struct {
	// Name is the file's base name, for example "main.go".
	Name string
	// Package is the package clause.
	Package string
	// Doc is the package doc comment, one entry per line, without markers.
	Doc []string
	// Decls are the top-level declarations in emission order.
	Decls []Decl
}

// Program is the complete set of files the converter produced for one input,
// plus the support code they need.
type Program struct {
	// Module is the Go module path of the generated project.
	Module string
	// Files are the emitted source files.
	Files []*File
	// Helpers names the runtime support functions the code calls.
	Helpers []string
	// Concepts are the teaching concept ids triggered while building this
	// program, in first-trigger order.
	Concepts []string
	// Todos collects every unconverted or approximated construct.
	Todos []Todo
}

// ---------------------------------------------------------------------------
// Declarations

// Decl is a top-level declaration.
type Decl interface {
	Annotated
	declNode()
}

// FuncDecl is a function or method declaration.
type FuncDecl struct {
	Meta
	Name string
	// Recv is the method receiver, nil for a plain function.
	Recv *Param
	// Params are the declared parameters.
	Params []Param
	// Results are the result types. A trailing Error result is what a Perl
	// `die` becomes.
	Results []*Type
	// ResultNames optionally names the results.
	ResultNames []string
	Body        *Block
	// Doc is the Go doc comment, present in both output variants.
	Doc []string
}

// Param is one parameter or receiver.
type Param struct {
	Name string
	Type *Type
	// Variadic marks the final ...T parameter.
	Variadic bool
}

// VarDecl declares one or more package-level or local variables.
type VarDecl struct {
	Meta
	Names []string
	Type  *Type
	// Values are the initialisers; empty means the zero value.
	Values []Expr
	// Const emits a const declaration instead of var.
	Const bool
	Doc   []string
}

// TypeDecl declares a named struct type.
type TypeDecl struct {
	Meta
	Name   string
	Fields []Field
	Doc    []string
}

// Field is one struct field.
type Field struct {
	Name string
	Type *Type
	// Tag is the raw struct tag text without backquotes.
	Tag string
	// Doc is a comment line above the field.
	Doc string
}

// ImportDecl is a package-level comment-only marker used when the emitter has
// to force an import that no expression mentions. It emits nothing itself.
type ImportDecl struct {
	Meta
	Path string
}

// RawDecl is verbatim Go source at declaration level. It exists for the
// runtime support helpers, which are already-written Go.
type RawDecl struct {
	Meta
	Source string
	Doc    []string
}

func (*FuncDecl) declNode()   {}
func (*VarDecl) declNode()    {}
func (*TypeDecl) declNode()   {}
func (*ImportDecl) declNode() {}
func (*RawDecl) declNode()    {}

// ---------------------------------------------------------------------------
// Statements

// Stmt is a statement.
type Stmt interface {
	Annotated
	stmtNode()
}

// Block is a statement list.
type Block struct {
	Meta
	Stmts []Stmt
}

// Add appends statements to the block, skipping nils.
func (b *Block) Add(stmts ...Stmt) {
	for _, s := range stmts {
		if s != nil {
			b.Stmts = append(b.Stmts, s)
		}
	}
}

// Assign is an assignment or short variable declaration.
type Assign struct {
	Meta
	LHS []Expr
	RHS []Expr
	// Op is "=", ":=", "+=", "|=" and so on.
	Op string
}

// DeclStmt declares local variables with var.
type DeclStmt struct {
	Meta
	Names  []string
	Type   *Type
	Values []Expr
}

// ExprStmt evaluates an expression for effect.
type ExprStmt struct {
	Meta
	X Expr
}

// IncDec is x++ or x--.
type IncDec struct {
	Meta
	X   Expr
	Dec bool
}

// If is an if/else chain. Init is the optional statement before the condition.
type If struct {
	Meta
	Init Stmt
	Cond Expr
	Then *Block
	// Else is either another *If or a *Block.
	Else Stmt
}

// For is a Go for statement. All three header parts are optional; with all
// three nil it is an infinite loop.
type For struct {
	Meta
	Init  Stmt
	Cond  Expr
	Post  Stmt
	Body  *Block
	Label string
}

// Range is a for ... range statement.
type Range struct {
	Meta
	Key   Expr // nil or the blank identifier
	Value Expr // nil when only the key is used
	X     Expr
	// Define emits := instead of =.
	Define bool
	Body   *Block
	Label  string
}

// Return returns from a function.
type Return struct {
	Meta
	Results []Expr
}

// Branch is break, continue, or goto.
type Branch struct {
	Meta
	// Kind is "break", "continue", or "goto".
	Kind  string
	Label string
}

// Labeled attaches a label to a statement.
type Labeled struct {
	Meta
	Label string
	Stmt  Stmt
}

// Switch is a switch statement. A switch with no tag is Go's if/else chain
// alternative and is what a long Perl elsif ladder becomes.
type Switch struct {
	Meta
	Init  Stmt
	Tag   Expr
	Cases []Case
}

// Case is one arm of a switch. An empty Values slice is the default arm.
type Case struct {
	Values []Expr
	Body   *Block
}

// Defer defers a call.
type Defer struct {
	Meta
	Call Expr
}

// Go starts a goroutine.
type Go struct {
	Meta
	Call Expr
}

// BlockStmt is a bare block, used where Perl had one for scoping.
type BlockStmt struct {
	Meta
	Body *Block
}

// CommentStmt is a standalone comment carried over from the Perl source. It is
// emitted in both variants because the developer wrote it.
type CommentStmt struct {
	Meta
	Lines []string
}

// TodoStmt marks a construct that could not be converted. It emits a TODO
// comment plus, when Panic is set, a call that fails loudly rather than
// silently doing the wrong thing.
type TodoStmt struct {
	Meta
	Info Todo
	// Panic makes the emitted code fail at run time instead of silently
	// continuing past a construct that was dropped.
	Panic bool
}

// RawStmt is verbatim Go source. It is a last resort and always carries a
// note explaining itself.
type RawStmt struct {
	Meta
	Source string
}

func (*Block) stmtNode()       {}
func (*Assign) stmtNode()      {}
func (*DeclStmt) stmtNode()    {}
func (*ExprStmt) stmtNode()    {}
func (*IncDec) stmtNode()      {}
func (*If) stmtNode()          {}
func (*For) stmtNode()         {}
func (*Range) stmtNode()       {}
func (*Return) stmtNode()      {}
func (*Branch) stmtNode()      {}
func (*Labeled) stmtNode()     {}
func (*Switch) stmtNode()      {}
func (*Defer) stmtNode()       {}
func (*Go) stmtNode()          {}
func (*BlockStmt) stmtNode()   {}
func (*CommentStmt) stmtNode() {}
func (*TodoStmt) stmtNode()    {}
func (*RawStmt) stmtNode()     {}

// ---------------------------------------------------------------------------
// Expressions

// Expr is an expression. Type reports the Go type the expression produces,
// which the lowering passes rely on to decide conversions.
type Expr interface {
	Annotated
	exprNode()
	Type() *Type
}

// typed is embedded by expressions that carry a resolved type.
type typed struct {
	T *Type
}

func (t typed) Type() *Type { return t.T }

// Ident is a variable, function, or package name.
type Ident struct {
	Meta
	typed
	Name string
}

// LitKind distinguishes literal spellings.
type LitKind int

const (
	// LitInt, LitFloat, LitString and LitBool are written out by the emitter
	// exactly as Value gives them, already in Go syntax.
	LitInt LitKind = iota
	LitFloat
	LitString
	LitBool
	// LitNil is the untyped nil.
	LitNil
	// LitRaw is a pre-rendered Go expression such as a composite literal
	// spelled by hand.
	LitRaw
)

// Lit is a literal value already in Go spelling.
type Lit struct {
	Meta
	typed
	Kind  LitKind
	Value string
}

// Call is a function or method call.
type Call struct {
	Meta
	typed
	Fun  Expr
	Args []Expr
	// Ellipsis emits the trailing ... on the final argument.
	Ellipsis bool
}

// Selector is x.Sel.
type Selector struct {
	Meta
	typed
	X   Expr
	Sel string
	// Import records the package path when X is a package identifier, so the
	// emitter can register it.
	Import string
}

// Index is x[i].
type Index struct {
	Meta
	typed
	X     Expr
	Index Expr
}

// IndexComma is v, ok := m[k], the comma-ok form. It is only valid as the RHS
// of an Assign with two LHS entries.
type IndexComma struct {
	Meta
	typed
	X     Expr
	Index Expr
}

// SliceExpr is x[lo:hi].
type SliceExpr struct {
	Meta
	typed
	X    Expr
	Low  Expr
	High Expr
}

// Binary is a binary operation.
type Binary struct {
	Meta
	typed
	Op string
	L  Expr
	R  Expr
}

// Unary is a prefix operation: !, -, &, *, <-.
type Unary struct {
	Meta
	typed
	Op string
	X  Expr
}

// Paren forces parentheses. The emitter also adds them where precedence
// requires, so this is only for readability decisions.
type Paren struct {
	Meta
	typed
	X Expr
}

// CompositeLit is T{...}.
type CompositeLit struct {
	Meta
	typed
	// LitType is the composite's type. When nil the emitter omits it, which is
	// valid inside a nested literal.
	LitType *Type
	Elems   []Expr
	// KeyValues, when set, pairs with Elems to emit Key: Value entries.
	Keys []Expr
}

// FuncLit is a function literal.
type FuncLit struct {
	Meta
	typed
	Params  []Param
	Results []*Type
	Body    *Block
}

// TypeAssert is x.(T).
type TypeAssert struct {
	Meta
	typed
	X      Expr
	Assert *Type
}

// Conversion is T(x).
type Conversion struct {
	Meta
	typed
	To *Type
	X  Expr
}

// RawExpr is verbatim Go. Used for the few places where spelling the tree out
// would be more obscure than the text.
type RawExpr struct {
	Meta
	typed
	Source string
}

func (*Ident) exprNode()        {}
func (*Lit) exprNode()          {}
func (*Call) exprNode()         {}
func (*Selector) exprNode()     {}
func (*Index) exprNode()        {}
func (*IndexComma) exprNode()   {}
func (*SliceExpr) exprNode()    {}
func (*Binary) exprNode()       {}
func (*Unary) exprNode()        {}
func (*Paren) exprNode()        {}
func (*CompositeLit) exprNode() {}
func (*FuncLit) exprNode()      {}
func (*TypeAssert) exprNode()   {}
func (*Conversion) exprNode()   {}
func (*RawExpr) exprNode()      {}

// ---------------------------------------------------------------------------
// Constructors for the shapes the lowering passes build constantly

// NewIdent builds a typed identifier.
func NewIdent(name string, t *Type) *Ident { return &Ident{typed: typed{T: t}, Name: name} }

// Str builds a Go string literal from an already-quoted spelling.
func Str(quoted string) *Lit {
	return &Lit{typed: typed{T: TString}, Kind: LitString, Value: quoted}
}

// IntLit builds an integer literal.
func IntLit(text string) *Lit { return &Lit{typed: typed{T: TInt}, Kind: LitInt, Value: text} }

// FloatLit builds a floating-point literal.
func FloatLit(text string) *Lit {
	return &Lit{typed: typed{T: TFloat}, Kind: LitFloat, Value: text}
}

// BoolLit builds a boolean literal.
func BoolLit(v bool) *Lit {
	s := "false"
	if v {
		s = "true"
	}
	return &Lit{typed: typed{T: TBool}, Kind: LitBool, Value: s}
}

// Nil builds the untyped nil with the given static type.
func Nil(t *Type) *Lit { return &Lit{typed: typed{T: t}, Kind: LitNil, Value: "nil"} }

// Pkg builds a package-qualified selector, for example fmt.Println.
func Pkg(pkgPath, pkgName, sel string, t *Type) *Selector {
	return &Selector{
		typed:  typed{T: t},
		X:      &Ident{Name: pkgName},
		Sel:    sel,
		Import: pkgPath,
	}
}

// CallOf builds a call with a known result type.
func CallOf(fn Expr, t *Type, args ...Expr) *Call {
	return &Call{typed: typed{T: t}, Fun: fn, Args: args}
}

// Bin builds a binary expression.
func Bin(op string, l, r Expr, t *Type) *Binary {
	return &Binary{typed: typed{T: t}, Op: op, L: l, R: r}
}

// Un builds a unary expression.
func Un(op string, x Expr, t *Type) *Unary {
	return &Unary{typed: typed{T: t}, Op: op, X: x}
}

// Raw builds a verbatim Go expression with a declared type.
func Raw(src string, t *Type) *RawExpr {
	return &RawExpr{typed: typed{T: t}, Source: src}
}
