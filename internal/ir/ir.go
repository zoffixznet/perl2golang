// Package ir defines the typed intermediate representation that sits between
// the Perl AST and the Go emitter.
//
// The IR is deliberately Go-shaped: every node corresponds to something the
// emitter can print directly. All of the work of deciding what a piece of Perl
// means happens before the IR is built, so the emitter never has to guess.
//
// Two things ride along on every node. Provenance records the Perl source that
// produced it, which the annotated output and the per-file walkthrough both
// need. Notes carry explanations that are rendered only into the annotated
// output, so the clean and annotated programs are two renderings of one tree
// and cannot drift apart.
package ir

import "strings"

// Provenance links an IR node back to the Perl it came from.
type Provenance struct {
	Line int
	Col  int
	Text string // the original Perl source, trimmed
}

// Valid reports whether the provenance points at real source.
func (p Provenance) Valid() bool { return p.Line > 0 }

// Note is an explanation attached to a node. Notes are rendered as comments
// in the annotated program and are invisible in the clean one.
type Note struct {
	// Text is the explanation, written as prose without a leading marker.
	Text string
	// Concepts are teaching concept ids this node illustrates.
	Concepts []string
}

// Todo marks something the converter could not translate faithfully. It is
// rendered into both programs, because hiding it would be dishonest.
//
// Short and Message exist separately because the clean program carries no
// conversion vocabulary at all: Short states the behaviour that is missing in
// the reader's own terms, while Message explains the whole story and is used
// by the report and the annotated program.
type Todo struct {
	// Code is the diagnostic code, for example "P2G1301".
	Code string
	// Short is a clean-safe one-line statement of what is not implemented.
	Short string
	// Message says what could not be converted and why.
	Message string
	// Perl is the original source of the construct.
	Perl string
	Prov Provenance
	// Spelled marks a Todo whose code stands in for the construct in a form
	// that names the diagnostic code and the missing behaviour itself, as the
	// notImplemented stand-in does. The wording is then written above the
	// first of them and left off the rest, because the mark is at every one of
	// them and a paragraph repeated thirty times buries the code between them.
	Spelled bool
}

// Meta is embedded in every node that can carry provenance and notes.
type Meta struct {
	Prov  Provenance
	Notes []Note
	Todo  *Todo
}

func (m *Meta) meta() *Meta { return m }

// Annotated is implemented by every node that can carry annotations.
type Annotated interface {
	meta() *Meta
}

// Note appends an annotation to a node.
func Annotate(n Annotated, text string, concepts ...string) {
	m := n.meta()
	m.Notes = append(m.Notes, Note{Text: text, Concepts: concepts})
}

// ---------------------------------------------------------------------------
// Types

// TypeKind enumerates the Go types the converter emits.
type TypeKind int

const (
	// Invalid is the zero value and never appears in a finished IR.
	Invalid TypeKind = iota
	// Void is the absence of a value, used for a function that returns nothing.
	Void
	Bool
	Int
	Float
	String
	// Slice is []Elem.
	Slice
	// Map is map[Key]Elem. Perl hash keys are always strings.
	Map
	// Pointer is *Elem.
	Pointer
	// Func is a function type.
	Func
	// Named is a declared type: a struct, an interface, or a stdlib type such
	// as os.File. Name holds the rendered name and Import the package it needs.
	Named
	// Any is Go's any. It is the documented fallback for a Perl scalar whose
	// type inference did not resolve, and its use is counted as a quality
	// metric.
	Any
	// Error is the error interface.
	Error
)

// Type is a Go type in the IR.
type Type struct {
	Kind TypeKind
	// Elem is the element type of a slice, map, or pointer.
	Elem *Type
	// Key is a map's key type. Nil means string.
	Key *Type
	// Params and Results describe a Func.
	Params  []*Type
	Results []*Type
	// Variadic marks a Func whose last parameter is variadic.
	Variadic bool
	// Name is the rendered name of a Named type, including any package
	// qualifier ("os.File", "Record").
	Name string
	// Import is the package path a Named type needs, empty for local types.
	Import string
}

// Common types, shared because they are immutable in practice.
var (
	TBool   = &Type{Kind: Bool}
	TInt    = &Type{Kind: Int}
	TFloat  = &Type{Kind: Float}
	TString = &Type{Kind: String}
	TAny    = &Type{Kind: Any}
	TError  = &Type{Kind: Error}
	TVoid   = &Type{Kind: Void}
)

// SliceOf returns []elem. An element type that carries no information becomes
// any, because a slice of nothing has no spelling in Go.
func SliceOf(elem *Type) *Type { return &Type{Kind: Slice, Elem: usable(elem)} }

// MapOf returns map[string]elem.
func MapOf(elem *Type) *Type { return &Type{Kind: Map, Elem: usable(elem)} }

// usable substitutes any for a type that cannot be written down.
func usable(t *Type) *Type {
	if t == nil || t.Kind == Void || t.Kind == Invalid {
		return TAny
	}
	return t
}

// PointerTo returns *elem.
func PointerTo(elem *Type) *Type { return &Type{Kind: Pointer, Elem: elem} }

// NamedType returns a named type that lives in the given import path.
func NamedType(name, importPath string) *Type {
	return &Type{Kind: Named, Name: name, Import: importPath}
}

// FuncOf builds a function type.
func FuncOf(params, results []*Type) *Type {
	return &Type{Kind: Func, Params: params, Results: results}
}

// Go renders the type as Go source. Imports needed by the type are reported
// through the collect callback, which may be nil.
func (t *Type) Go(collect func(path string)) string {
	if t == nil {
		return "any"
	}
	switch t.Kind {
	case Void, Invalid:
		return ""
	case Bool:
		return "bool"
	case Int:
		return "int"
	case Float:
		return "float64"
	case String:
		return "string"
	case Any:
		return "any"
	case Error:
		return "error"
	case Slice:
		return "[]" + t.Elem.Go(collect)
	case Map:
		key := "string"
		if t.Key != nil {
			key = t.Key.Go(collect)
		}
		return "map[" + key + "]" + t.Elem.Go(collect)
	case Pointer:
		return "*" + t.Elem.Go(collect)
	case Named:
		if t.Import != "" && collect != nil {
			collect(t.Import)
		}
		return t.Name
	case Func:
		var sb strings.Builder
		sb.WriteString("func(")
		for i, p := range t.Params {
			if i > 0 {
				sb.WriteString(", ")
			}
			if t.Variadic && i == len(t.Params)-1 {
				sb.WriteString("...")
			}
			sb.WriteString(p.Go(collect))
		}
		sb.WriteString(")")
		switch len(t.Results) {
		case 0:
		case 1:
			sb.WriteString(" " + t.Results[0].Go(collect))
		default:
			sb.WriteString(" (")
			for i, r := range t.Results {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(r.Go(collect))
			}
			sb.WriteString(")")
		}
		return sb.String()
	}
	return "any"
}

// String renders the type for diagnostics and the conversion report.
func (t *Type) String() string { return t.Go(nil) }

// Equal reports structural equality.
func (t *Type) Equal(o *Type) bool {
	if t == nil || o == nil {
		return t == o
	}
	if t.Kind != o.Kind || t.Name != o.Name || t.Import != o.Import {
		return false
	}
	if !t.Elem.Equal(o.Elem) || !t.Key.Equal(o.Key) {
		return false
	}
	if len(t.Params) != len(o.Params) || len(t.Results) != len(o.Results) {
		return false
	}
	for i := range t.Params {
		if !t.Params[i].Equal(o.Params[i]) {
			return false
		}
	}
	for i := range t.Results {
		if !t.Results[i].Equal(o.Results[i]) {
			return false
		}
	}
	return true
}

// IsNumeric reports whether arithmetic applies directly.
func (t *Type) IsNumeric() bool {
	return t != nil && (t.Kind == Int || t.Kind == Float)
}

// Zero returns the Go zero value literal for the type.
func (t *Type) Zero(collect func(string)) string {
	if t == nil {
		return "nil"
	}
	switch t.Kind {
	case Bool:
		return "false"
	case Int:
		return "0"
	case Float:
		return "0"
	case String:
		return `""`
	case Slice, Map, Pointer, Func, Any, Error:
		return "nil"
	case Named:
		return t.Go(collect) + "{}"
	}
	return "nil"
}

// MetaOf returns the annotation block of any IR node. It exists so packages
// outside ir can read provenance and notes without every node type having to
// expose a getter.
func MetaOf(n Annotated) *Meta {
	if n == nil {
		return nil
	}
	return n.meta()
}
