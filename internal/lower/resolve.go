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
	// Pkg is the package a global binding was created under, which is what
	// lets $TextUtil::CALLS and %CALLS inside package TextUtil resolve to
	// one variable rather than two.
	Pkg string
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
	// MinLen is a lower bound on an array's length, taken from the literal
	// lists assigned to it whole, and reset to zero by anything that can
	// shorten it. It is what lets a write at a constant index be emitted as a
	// plain index expression, which is the readable form, rather than as a
	// growth the array does not need.
	MinLen int
	// lenKnown records that MinLen has been set at least once, so the first
	// observation replaces the zero rather than being folded into it.
	lenKnown bool
	// NilElems marks a container the program stored undef into. Perl's undef
	// is a value an element can hold, so such a container's element type has
	// to have room for "nothing here", which for a Go scalar means a pointer.
	NilElems bool
	// Definite marks a scalar every assignment to which gives it a value
	// that cannot be undef: a literal, some arithmetic, a length. It is what
	// separates a variable whose `defined` test has a real answer from one
	// whose answer is always yes, and the test is deliberately conservative,
	// because saying yes wrongly turns an approximation into a wrong answer.
	Definite bool
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
	// Bounds records that this is a loop variable counting over one array's
	// indices, which is what lets `$a[$i]` inside `for my $i (0 .. $#a)`
	// stay a plain Go index expression instead of a tolerant read.
	Bounds *indexBounds
}

// indexBounds says which array a counting loop's variable is an index into,
// and how far its range reaches.
//
// Top is the largest value the variable takes, written as an offset from the
// array's length: `0 .. $#a` counts up to len(a)-1 and so has a Top of -1,
// while `1 .. @a` counts up to len(a) and has a Top of 0. Low is the smallest
// value it takes. Between them they answer, for an index written as the
// variable plus or minus a constant, whether every value it can hold is one
// the array already has.
type indexBounds struct {
	Arr *Binding
	Low int
	Top int
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
	// listReturn caches whether the sub hands back a list rather than one
	// value: 0 not yet asked, 1 yes, 2 no. Go has one shape for both, so the
	// answer comes from the source and is worth working out only once.
	listReturn int8
	// CallSites counts calls found in the file.
	CallSites int
	// Line is the declaration line.
	Line int
	// ResultEvidence records the shape of every return seen on the first
	// pass, so the second pass can commit to one signature.
	ResultEvidence [][]*ir.Type
	// NilResults marks the result positions some return leaves undef, so
	// the settled type gets room for the absence: a position seen both as
	// text and as undef is a *string, where a plain string would turn the
	// undef into an empty string the caller cannot tell from a real one.
	NilResults map[int]bool
	// TailSpill records that some return ends in an array, as
	// `return ($cost, @path)` does. The Go function returns the array as its
	// last result, and every call site in list position splices that result
	// into the surrounding list instead of storing it as one element.
	TailSpill bool
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
	// Uniform marks a closure that shares a collection with other closures
	// and therefore has to take everything through @_ and answer with one
	// value of no fixed type.
	Uniform bool
	// Promoted marks an accessor that had to become a real method after all,
	// because something calls it on a value whose class is only known while
	// the program runs and an interface cannot promise a field.
	Promoted bool
	// Named is the binding of a `%args` named-argument hash, which becomes
	// one Go parameter per key the sub reads.
	Named       *Binding
	NamedParams []*Binding
	namedBy     map[string]*Binding
	// Prologue holds statements the parameter recovery synthesised to run
	// before the body: a `%args` the sub walks as data is rebuilt here from
	// the variadic pair list.
	Prologue []ir.Stmt
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
	// destroys are the objects whose destructor runs at this scope's
	// closing brace, in declaration order; they are called in reverse.
	destroys []scopeDestroy
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
	"$^X": true,
}
