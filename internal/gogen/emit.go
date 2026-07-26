package gogen

import (
	"fmt"
	"go/format"
	"strings"

	"perl2go/internal/ir"
)

// Mode selects which of the two renderings of one IR tree to produce.
type Mode int

const (
	// Clean produces an ordinary Go program. Nothing in the output refers to
	// where the code came from.
	Clean Mode = iota
	// Annotated produces the same program with the explanatory notes rendered
	// as comments.
	Annotated
)

// Emitter renders IR as Go source text.
type Emitter struct {
	mode    Mode
	imports *Imports
	sb      strings.Builder
	indent  int
	// atLineStart tracks whether the next write begins a line, so indentation
	// is applied exactly once per line.
	atLineStart bool
}

// New returns an emitter for the given rendering mode.
func New(mode Mode) *Emitter {
	return &Emitter{mode: mode, imports: NewImports(), atLineStart: true}
}

// Imports exposes the import set, which is only complete after emission.
func (e *Emitter) Imports() *Imports { return e.imports }

// File renders a whole file and formats it. The returned error names the
// offending source when formatting fails, because that means the emitter
// produced something that is not Go and that is a bug worth seeing.
func (e *Emitter) File(f *ir.File) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("gogen: no file to render")
	}
	// Each declaration is rendered on its own so that one which writes nothing
	// (an import request, say) does not leave a stray blank line behind.
	var parts []string
	for _, d := range f.Decls {
		sub := &Emitter{mode: e.mode, imports: e.imports, atLineStart: true}
		sub.decl(d)
		if text := sub.sb.String(); strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}

	var head strings.Builder
	for _, line := range f.Doc {
		head.WriteString(commentLine(line))
		head.WriteString("\n")
	}
	pkg := f.Package
	if pkg == "" {
		// A file without a package clause is not Go at all, so the emitter
		// picks the only name that is always sensible rather than writing
		// something that cannot be compiled.
		pkg = "main"
	}
	head.WriteString("package " + pkg + "\n\n")
	if imp := e.imports.Render(); imp != "" {
		head.WriteString(imp)
		head.WriteString("\n")
	}
	head.WriteString(strings.Join(parts, "\n"))

	src := head.String()
	out, err := format.Source([]byte(src))
	if err != nil {
		return []byte(src), fmt.Errorf("generated Go for %s does not parse: %w", f.Name, err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Low-level writing

func (e *Emitter) w(s string) {
	if s == "" {
		return
	}
	if e.atLineStart {
		e.sb.WriteString(strings.Repeat("\t", e.indent))
		e.atLineStart = false
	}
	e.sb.WriteString(s)
}

func (e *Emitter) wf(format string, args ...any) { e.w(fmt.Sprintf(format, args...)) }

func (e *Emitter) nl() {
	e.sb.WriteString("\n")
	e.atLineStart = true
}

func (e *Emitter) line(s string) {
	e.w(s)
	e.nl()
}

func (e *Emitter) in()  { e.indent++ }
func (e *Emitter) out() { e.indent-- }

func commentLine(s string) string {
	if s == "" {
		return "//"
	}
	return "// " + s
}

// comment writes a possibly multi-line comment block.
func (e *Emitter) comment(lines []string) {
	for _, l := range lines {
		for _, sub := range strings.Split(l, "\n") {
			e.line(commentLine(sub))
		}
	}
}

// notes writes a node's annotations, but only in annotated mode.
func (e *Emitter) notes(n ir.Annotated) {
	if e.mode != Annotated {
		return
	}
	m := metaOf(n)
	if m == nil {
		return
	}
	if m.Prov.Valid() && m.Prov.Text != "" {
		for i, l := range strings.Split(m.Prov.Text, "\n") {
			if i == 0 {
				e.line(commentLine("Perl: " + l))
			} else {
				e.line(commentLine("      " + l))
			}
		}
	}
	for _, note := range m.Notes {
		e.comment(wrap(note.Text, 72))
	}
}

// todo writes the TODO marker for a node that carries one. It is written in
// both modes, because the whole point is that the tool never hides work it
// could not do. Only the wording differs.
func (e *Emitter) todo(t ir.Todo) {
	if e.mode == Annotated {
		text := t.Message
		if t.Code != "" {
			text = t.Code + ": " + text
		}
		lines := wrap("TODO: "+text, 72)
		e.comment(lines)
		if t.Perl != "" {
			for _, l := range strings.Split(t.Perl, "\n") {
				e.line(commentLine("  " + l))
			}
		}
		return
	}
	short := t.Short
	if short == "" {
		short = "not implemented"
	}
	e.comment(wrap("TODO: "+short, 72))
}

// wrap breaks text into lines no longer than width, on word boundaries.
func wrap(s string, width int) []string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil
	}
	var out []string
	cur := fields[0]
	for _, f := range fields[1:] {
		if len(cur)+1+len(f) > width {
			out = append(out, cur)
			cur = f
			continue
		}
		cur += " " + f
	}
	return append(out, cur)
}

func metaOf(n ir.Annotated) *ir.Meta {
	if n == nil {
		return nil
	}
	return ir.MetaOf(n)
}

// typ renders a type and registers whatever import it needs.
func (e *Emitter) typ(t *ir.Type) string {
	return t.Go(func(path string) { e.imports.Add(path) })
}
