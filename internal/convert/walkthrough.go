package convert

import (
	"strings"

	"perl2go/internal/gogen"
	"perl2go/internal/ir"
	"perl2go/internal/lower"
	"perl2go/internal/teach"
)

// walkthrough turns the lowered program into the ordered tour that the
// walkthrough document renders.
//
// The tour is built from the IR rather than from the two text outputs, because
// the IR is the only place that has both halves: the Perl each node came from
// and the explanation of the Go it became. Grouping is by the developer's own
// comments first, because where they drew a line is where the file's sections
// actually are.
func walkthrough(low *lower.Result, src []byte) []teach.Segment {
	lines := strings.Split(string(src), "\n")
	var out []teach.Segment

	if len(low.Program.Files) == 0 {
		return nil
	}
	file := low.Program.Files[0]

	for _, d := range file.Decls {
		fn, isFunc := d.(*ir.FuncDecl)
		if !isFunc {
			if seg, ok := declSegment(d, lines); ok {
				out = append(out, seg)
			}
			continue
		}
		if fn.Name == "main" {
			out = append(out, mainSegments(fn, lines)...)
			continue
		}
		if seg, ok := declSegment(fn, lines); ok {
			seg.Title = "The " + fn.Name + " function"
			out = append(out, seg)
		}
	}
	return out
}

// declSegment builds one segment for a whole declaration.
func declSegment(d ir.Decl, lines []string) (teach.Segment, bool) {
	m := ir.MetaOf(d)
	if m == nil || !m.Prov.Valid() {
		return teach.Segment{}, false
	}
	from, to := spanOf([]ir.Stmt{}, m.Prov.Line, m.Prov.Text)
	explain, concepts := collectNotes([]ir.Annotated{d})
	return teach.Segment{
		Title:    titleFor(d),
		PerlFrom: from,
		PerlTo:   to,
		Perl:     sourceLines(lines, from, to),
		Go:       strings.TrimRight(gogen.RenderDecl(gogen.Clean, d), "\n"),
		Explain:  explain,
		Concepts: concepts,
	}, true
}

// mainSegments breaks the body of main into readable regions.
func mainSegments(fn *ir.FuncDecl, lines []string) []teach.Segment {
	if fn.Body == nil {
		return nil
	}
	var out []teach.Segment
	var group []ir.Stmt
	title := ""

	flush := func() {
		if len(group) == 0 {
			return
		}
		if seg, ok := groupSegment(group, title, lines); ok {
			out = append(out, seg)
		}
		group = nil
		title = ""
	}

	for _, st := range fn.Body.Stmts {
		// A comment the developer wrote marks the start of a new region, and
		// its text is the best title available.
		if c, ok := st.(*ir.CommentStmt); ok {
			flush()
			if len(c.Lines) > 0 {
				title = strings.TrimSpace(c.Lines[0])
			}
			continue
		}
		group = append(group, st)
		if len(group) >= 8 {
			flush()
		}
	}
	flush()
	return out
}

// groupSegment renders one region.
func groupSegment(group []ir.Stmt, title string, lines []string) (teach.Segment, bool) {
	from, to := 0, 0
	for _, st := range group {
		m := ir.MetaOf(st)
		if m == nil || !m.Prov.Valid() {
			continue
		}
		start := m.Prov.Line
		end := start + strings.Count(m.Prov.Text, "\n")
		if from == 0 || start < from {
			from = start
		}
		if end > to {
			to = end
		}
	}
	if from == 0 {
		return teach.Segment{}, false
	}

	var goText strings.Builder
	for i, st := range group {
		if i > 0 {
			goText.WriteString("\n")
		}
		goText.WriteString(strings.TrimRight(gogen.RenderStmt(gogen.Clean, st), "\n"))
	}

	annotated := make([]ir.Annotated, 0, len(group))
	for _, st := range group {
		annotated = append(annotated, st)
	}
	explain, concepts := collectNotes(annotated)

	if title == "" {
		title = defaultTitle(group, from, to)
	}
	return teach.Segment{
		Title:    title,
		PerlFrom: from,
		PerlTo:   to,
		Perl:     sourceLines(lines, from, to),
		Go:       goText.String(),
		Explain:  explain,
		Concepts: concepts,
	}, true
}

// spanOf works out the source range a declaration covers.
func spanOf(_ []ir.Stmt, line int, text string) (int, int) {
	return line, line + strings.Count(text, "\n")
}

// sourceLines returns the original source for an inclusive 1-based range.
func sourceLines(lines []string, from, to int) string {
	if from < 1 {
		from = 1
	}
	if to > len(lines) {
		to = len(lines)
	}
	if from > to || from > len(lines) {
		return ""
	}
	return strings.Join(lines[from-1:to], "\n")
}

// titleFor names a declaration in the tour.
func titleFor(d ir.Decl) string {
	switch n := d.(type) {
	case *ir.FuncDecl:
		return "The " + n.Name + " function"
	case *ir.VarDecl:
		if n.Const {
			return "The " + strings.Join(n.Names, ", ") + " constant"
		}
		return "The " + strings.Join(n.Names, ", ") + " variable"
	case *ir.TypeDecl:
		return "The " + n.Name + " type"
	}
	return "Declarations"
}

// defaultTitle names a region that had no comment above it.
func defaultTitle(group []ir.Stmt, from, to int) string {
	if len(group) > 0 {
		switch group[0].(type) {
		case *ir.For, *ir.Range:
			return "The loop at line " + itoa(from)
		case *ir.If:
			return "The branch at line " + itoa(from)
		case *ir.Return:
			return "Returning at line " + itoa(from)
		}
	}
	if from == to {
		return "Line " + itoa(from)
	}
	return "Lines " + itoa(from) + " to " + itoa(to)
}

// collectNotes walks a set of IR nodes and gathers their explanations in
// order, without repeats.
func collectNotes(nodes []ir.Annotated) (string, []string) {
	var texts []string
	var concepts []string
	seenText := map[string]bool{}
	seenConcept := map[string]bool{}

	add := func(n ir.Annotated) {
		m := ir.MetaOf(n)
		if m == nil {
			return
		}
		for _, note := range m.Notes {
			if note.Text == "" || seenText[note.Text] {
				continue
			}
			seenText[note.Text] = true
			texts = append(texts, note.Text)
			for _, c := range note.Concepts {
				if seenConcept[c] {
					continue
				}
				seenConcept[c] = true
				concepts = append(concepts, c)
			}
		}
		if m.Todo != nil {
			line := "Not converted: " + m.Todo.Message
			if !seenText[line] {
				seenText[line] = true
				texts = append(texts, line)
			}
		}
	}

	for _, n := range nodes {
		walkIR(n, add)
	}

	// A region with a dozen notes is a wall of text nobody reads. The first
	// few carry the region; the rest are still in the annotated program.
	const maxNotes = 6
	if len(texts) > maxNotes {
		texts = texts[:maxNotes]
	}
	return strings.Join(texts, "\n\n"), concepts
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
