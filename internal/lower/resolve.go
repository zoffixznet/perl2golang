package lower

import (
	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
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
	// Evidence is every type the inference pass observed for this binding.
	Evidence []*ir.Type
	// Captured marks a binding referenced from inside a nested closure.
	Captured bool
	// Closed marks a filehandle the program closes explicitly, so the
	// generated code does not also defer a close.
	Closed bool
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
	// irDecl is the lowered function, filled in on each pass.
	irDecl *ir.FuncDecl
	// Doc is the comment block above the declaration.
	Doc []string
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
