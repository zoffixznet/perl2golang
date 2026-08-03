package gogen

import (
	"strings"

	"perl2golang/internal/ir"
)

// decl writes one top-level declaration.
//
// The doc comment comes first and is written in both renderings, because it is
// documentation the reader keeps. The provenance and the teaching notes come
// after it, separated by a blank line so the doc comment stays attached to the
// declaration as a Go doc comment rather than merging with the commentary.
func (e *Emitter) decl(d ir.Decl) {
	if d == nil {
		return
	}
	switch d := d.(type) {
	case *ir.ImportDecl:
		// An import declaration is a request, not output: it registers the
		// path so the import block includes it, and writes nothing.
		e.imports.Add(d.Path)

	case *ir.FuncDecl:
		e.declHeader(d, d.Doc)
		e.funcDecl(d)

	case *ir.VarDecl:
		e.declHeader(d, d.Doc)
		keyword := "var"
		if d.Const {
			keyword = "const"
		}
		if spec := e.varSpec(keyword, d.Names, d.Type, d.Values); spec != "" {
			e.line(spec)
		}

	case *ir.TypeDecl:
		e.declHeader(d, d.Doc)
		e.typeDecl(d)

	case *ir.RawDecl:
		e.declHeader(d, d.Doc)
		e.rawLines(d.Source)
	}
}

// declHeader writes the annotations and the doc comment of a declaration.
func (e *Emitter) declHeader(d ir.Decl, doc []string) {
	// Asked before the prologue writes, because writing a note is what makes
	// it no longer new.
	annotated := e.hasVisibleNotes(d)
	e.prologue(d)
	if len(doc) > 0 && annotated {
		e.nl()
	}
	e.comment(doc)
}

func (e *Emitter) funcDecl(d *ir.FuncDecl) {
	e.w("func ")
	if d.Recv != nil {
		e.w("(" + e.param(*d.Recv) + ") ")
	}
	name := d.Name
	if name == "" {
		name = "_"
	}
	e.w(name)
	e.w("(" + e.params(d.Params) + ")")
	e.w(e.results(d.Results, d.ResultNames))
	e.w(" ")
	e.block(d.Body)
	e.nl()
}

func (e *Emitter) typeDecl(d *ir.TypeDecl) {
	name := d.Name
	if name == "" {
		name = "_"
	}
	if len(d.Fields) == 0 {
		e.line("type " + name + " struct{}")
		return
	}
	e.line("type " + name + " struct {")
	e.in()
	for _, f := range d.Fields {
		if f.Doc != "" {
			e.comment([]string{f.Doc})
		}
		if line := e.field(f); line != "" {
			e.line(line)
		}
	}
	e.out()
	e.line("}")
}

// field renders one struct field. A field with no name is an embedded type.
func (e *Emitter) field(f ir.Field) string {
	typ := e.typ(f.Type)
	if typ == "" {
		typ = "any"
	}
	line := typ
	if f.Name != "" {
		line = f.Name + " " + typ
	}
	if f.Tag != "" {
		line += " `" + strings.ReplaceAll(f.Tag, "`", "'") + "`"
	}
	return line
}
