package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// Kind classifies where a binding came from, which decides how the emitter
// declares it.
type Kind int

const (
	// KindLocal is a `my` variable inside a block.
	KindLocal Kind = iota
	// KindGlobal is a package variable: `our`, an undeclared name, or one
	// declared at file scope and used inside a sub.
	KindGlobal
	// KindParam is a subroutine parameter recovered from @_.
	KindParam
	// KindLoop is a foreach loop variable.
	KindLoop
	// KindSpecial is one of Perl's own variables, such as $_ or @ARGV.
	KindSpecial
)

// Binding is one resolved Perl variable.
//
// Resolution happens before type inference and before lowering, so that every
// use of a name points at the same record. That is what lets inference collect
// evidence from a whole scope and what lets the report say something honest
// about each variable.
type Binding struct {
	// Perl is the source spelling with its sigil, for example "$count".
	Perl string
	// Sigil is '$', '@', '%', or '&'.
	Sigil rune
	// Go is the identifier the generated code uses.
	Go string
	// Type is the inferred Go type, filled in by inference.
	Type *ir.Type
	// Dynamic is true when inference gave up and fell back to `any`.
	Dynamic bool
	// Reason explains a dynamic fallback, for the report.
	Reason string
	Kind   Kind
	Line   int
	// Reads and Writes count uses, so the emitter can pick var versus :=
	// and can drop a variable that is written but never read.
	Reads  int
	Writes int
	// Used records the first pass's read count, which is the whole-file
	// answer the second pass needs before it has finished counting again.
	Used int
	// Evidence is every type the inference pass observed for this binding.
	Evidence []*ir.Type
	// Captured marks a binding referenced from inside a nested closure.
	Captured bool
	// Closed marks a filehandle the program closes explicitly, so the
	// generated code does not also defer a close.
	Closed bool
	// Init is the value a package-level binding is declared with, used where
	// one of Perl's own variables has to become a real Go variable.
	Init ir.Expr
	// Doc and Explain are the doc comment and the annotation for such a
	// binding, which deserves better than a generic one.
	Doc     string
	Explain string
	// Pos names the companion variable holding this scalar's match
	// position, for the code that walks it with a global match. Perl keeps
	// that position on the variable itself; Go needs somewhere to put it.
	Pos *Binding
	// Groups and NamedGroups describe the pattern a qr// assigned here, so
	// that a match against the variable knows what its captures are called.
	Groups      int
	NamedGroups map[string]int
}

// declared reports whether the binding is a real Go declaration the emitter
// must produce (as opposed to something like $_ that maps onto a Go construct).
func (b *Binding) declared() bool { return b.Kind != KindSpecial }

// Sub is one resolved subroutine.
type Sub struct {
	Name string
	Go   string
	Decl *ast.SubDecl
	// Params are the bindings recovered from the leading `my (...) = @_`
	// or `shift` idioms.
	Params []*Binding
	// Results are the Go result types. An empty slice is a void function.
	Results []*ir.Type
	// ResultNames optionally names them, used when a sub returns a list.
	ResultNames []string
	// UsesRawArgs is true when the sub touches @_ in a way the parameter
	// recovery could not model, so it takes a variadic slice instead.
	UsesRawArgs bool
	// ReturnsNothing records that every return is bare.
	ReturnsNothing bool
	// Recursive marks a sub that calls itself.
	Recursive bool
	// CallSites counts calls found in the file.
	CallSites int
	// Line is the declaration line.
	Line int
	// ResultEvidence records the shape of every return seen on the first
	// pass, so the second pass can commit to one signature.
	ResultEvidence [][]*ir.Type
	// Variadic is true when the sub takes its arguments as a slice.
	Variadic bool
	// VarArgs is the binding for the variadic parameter, if any.
	VarArgs *Binding
	// Comparator marks a sub used as `sort byname @list`, which reads its
	// two values out of the package globals $a and $b. Go passes them as
	// parameters, so such a sub gets a different signature entirely.
	Comparator bool
	// CmpElem is the element type the comparator was used on.
	CmpElem *ir.Type
	// LitType is the type object of the function literal an anonymous sub
	// produced on the first pass. It is refreshed once the signature is
	// settled, in place, because whatever holds the literal was inferred
	// from this very object.
	LitType *ir.Type
	// irDecl is the lowered function, filled in on each pass.
	irDecl *ir.FuncDecl
	// Doc is the comment block above the declaration.
	Doc []string

	// Pkg is the Perl package the sub was declared in.
	Pkg string
	// Class is the class it belongs to, nil for a plain function.
	Class *Class
	// Kind says whether it became a method, a constructor, a class method,
	// or an ordinary function.
	Kind SubKind
	// Recv is the receiver binding of a method.
	Recv *Binding
	// Accessor is the field a sub that only reads one hash key stands for.
	// Such a sub is not emitted at all: the field takes its place.
	Accessor *ClassField
	// Setter records that the accessor also writes when handed a value.
	Setter bool
	// Named is the binding of a `%args` named-argument hash, which becomes
	// one Go parameter per key the sub reads.
	Named       *Binding
	NamedParams []*Binding
	namedBy     map[string]*Binding
	// SelfVar names the variable a constructor blesses, which is the one
	// that has to be built as the struct rather than as a map.
	SelfVar string
	// Inherited is the ancestor's constructor a synthesised one calls, for a
	// class that declares no `sub new` of its own.
	Inherited *Sub
	// File is the source the sub was written in.
	File *SourceFile
	// ClassParam is the parameter holding the class name, for a constructor
	// whose body reads it. Perl passes the class as the first argument and
	// most constructors only pass it straight to bless; one that also tests
	// it or builds a default out of it needs it written down.
	ClassParam *Binding
}

// scope is one lexical level.
type scope struct {
	parent *scope
	vars   map[string]*Binding
	// fn is the sub this scope belongs to, nil at file level.
	fn *Sub
}

func newScope(parent *scope) *scope {
	s := &scope{parent: parent, vars: map[string]*Binding{}}
	if parent != nil {
		s.fn = parent.fn
	}
	return s
}

func (s *scope) lookup(key string) (*Binding, bool) {
	for cur := s; cur != nil; cur = cur.parent {
		if b, ok := cur.vars[key]; ok {
			return b, true
		}
	}
	return nil, false
}

// lookupLocal searches only this level, which is what a redeclaration check
// needs.
func (s *scope) lookupLocal(key string) (*Binding, bool) {
	b, ok := s.vars[key]
	return b, ok
}

func (s *scope) define(key string, b *Binding) { s.vars[key] = b }

// varKey is the map key for a variable: sigil plus name, so @x and %x and $x
// are three different bindings exactly as Perl treats them.
func varKey(sigil rune, name string) string { return string(sigil) + name }

// specialVars are Perl's own variables. They are recognised so that a use of
// one is never mistaken for an undeclared global, and so that the lowering can
// map each onto its Go equivalent.
var specialVars = map[string]bool{
	"$_": true, "@_": true, "@ARGV": true, "%ENV": true, "$0": true,
	"$!": true, "$@": true, "$/": true, "$\\": true, "$,": true, "$\"": true,
	"$;": true, "$a": true, "$b": true, "@INC": true, "%INC": true,
	"$$": true, "$?": true, "$^W": true, "$^O": true, "$|": true,
}
