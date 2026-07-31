package teach

import "strings"

// terminalWidth is the column the prose is folded at. Prose that runs wider
// than this is measurably harder to read, and the extra room a wide window
// offers is better spent on having the code sit at its own indentation.
const terminalWidth = 76

// Terminal renders the concept for reading at a prompt rather than on a page.
//
// The knowledge base is Markdown, written to be read on a page. At a terminal
// the same text arrives as paragraphs hundreds of columns wide, wrapped
// arbitrarily by the window. So: prose is folded at a readable width, headings
// lose their hashes, and fenced code loses its fences and keeps its own
// indentation, which means selecting a block with a mouse copies exactly the
// code. A block the knowledge base marks as deliberately broken says so,
// because a sample that does not compile is only a lesson if the reader knows
// that is the point.
//
// Render is the other side of this: it produces the Markdown a conversion
// writes into docs/concepts/, which is what a pager, an editor or a browser
// wants.
func (c *Concept) Terminal() string {
	var b strings.Builder
	b.WriteString(c.Title + "\n\n")

	var para []string
	blank := false
	flush := func() {
		if len(para) > 0 {
			b.WriteString(WrapAt(strings.Join(para, " "), terminalWidth) + "\n")
			para = nil
			blank = false
		}
	}
	write := func(line string) {
		b.WriteString(line + "\n")
		blank = strings.TrimSpace(line) == ""
	}

	fence := ""
	inFence := false
	for _, line := range strings.Split(strings.TrimSpace(c.Body), "\n") {
		line = strings.TrimRight(line, " \t")
		if info, ok := strings.CutPrefix(strings.TrimSpace(line), "```"); ok {
			flush()
			if inFence {
				inFence = false
				write("")
				continue
			}
			inFence, fence = true, info
			if !blank {
				write("")
			}
			// The knowledge base marks its two kinds of deliberately broken
			// sample differently, and the difference is the lesson.
			switch {
			case strings.Contains(fence, "invalid"):
				write("(this one does not compile; that is what it is showing)")
			case strings.Contains(fence, "fails"):
				write("(this one compiles and then fails when it runs; that is the point)")
			}
			continue
		}
		if inFence {
			write("  " + line)
			continue
		}
		if strings.TrimSpace(line) == "" {
			flush()
			if !blank {
				write("")
			}
			continue
		}
		if heading, ok := strings.CutPrefix(line, "#"); ok {
			flush()
			if !blank {
				write("")
			}
			write(strings.TrimLeft(heading, "# "))
			continue
		}
		if marker, rest, ok := listItem(line); ok {
			flush()
			write(marker + WrapAt(rest, terminalWidth-2-len(marker)))
			continue
		}
		if strings.HasPrefix(line, ">") || strings.HasPrefix(line, "|") {
			flush()
			write(line)
			continue
		}
		para = append(para, strings.TrimSpace(line))
	}
	flush()
	return b.String()
}

// Opening returns a concept's "why you care" paragraph, folded for a terminal.
// It is the first paragraph of the body.
func (c *Concept) Opening() string {
	body := strings.TrimSpace(c.Body)
	if i := strings.Index(body, "\n\n"); i >= 0 {
		body = body[:i]
	}
	return WrapAt(body, terminalWidth)
}

// listItem splits a Markdown bullet or numbered item into its marker and its
// text, so the text can be folded and the marker kept.
func listItem(line string) (marker, rest string, ok bool) {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]
	for _, m := range []string{"- ", "* ", "+ "} {
		if body, found := strings.CutPrefix(trimmed, m); found {
			return indent + m, body, true
		}
	}
	for i, r := range trimmed {
		if r >= '0' && r <= '9' {
			continue
		}
		if (r == '.' || r == ')') && i > 0 && i+1 < len(trimmed) && trimmed[i+1] == ' ' {
			return indent + trimmed[:i+2], trimmed[i+2:], true
		}
		break
	}
	return "", "", false
}

// WrapAt folds text to a maximum line width, breaking only between words. A
// word longer than the width is left whole rather than cut, because a long
// word here is an identifier or a URL and breaking it makes it useless.
func WrapAt(text string, width int) string {
	if width < 20 {
		width = 20
	}
	var out []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			out = append(out, line)
			line = word
		}
	}
	if line != "" {
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
