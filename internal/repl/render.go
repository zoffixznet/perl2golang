package repl

import (
	"fmt"
	"io"
	"strings"

	"perl2go/internal/report"
)

// printer writes the session transcript.
//
// The whole session goes to one stream. A REPL transcript that interleaved two
// streams would read correctly on a terminal and scramble the moment anybody
// piped it into a file, and `perl2go repl < session.txt` is a supported way to
// demo the tool. Colour is decided once, by the caller, from whether that
// stream is a terminal.
type printer struct {
	out   io.Writer
	color bool
}

// The escape sequences, kept in one place. Nothing here is written unless the
// output is a terminal and NO_COLOR is unset.
const (
	sgrReset  = "\x1b[0m"
	sgrPrompt = "\x1b[1;34m"
	sgrDim    = "\x1b[2m"
	sgrWarn   = "\x1b[33m"
	sgrRefuse = "\x1b[31m"
	sgrTitle  = "\x1b[1m"
)

func (p *printer) style(code, text string) string {
	if !p.color || text == "" {
		return text
	}
	return code + text + sgrReset
}

func (p *printer) raw(s string) { fmt.Fprint(p.out, s) }

func (p *printer) line(s string) { fmt.Fprintln(p.out, s) }

func (p *printer) blank() { fmt.Fprintln(p.out) }

// note writes an aside: the dim parenthesised lines that say what the
// converter recognised, what it re-emitted, or what it could not do here.
func (p *printer) note(format string, args ...any) {
	p.line("  " + p.style(sgrDim, fmt.Sprintf(format, args...)))
}

// code writes generated Go, indented two spaces and otherwise untouched.
//
// No fence and no border: selecting the block with a mouse should copy exactly
// the code and nothing else.
func (p *printer) code(lines []string) {
	for _, l := range lines {
		p.line("  " + l)
	}
}

// title writes a heading for the longer meta commands.
func (p *printer) title(s string) {
	p.line("  " + p.style(sgrTitle, s))
}

// body writes prose or an example, indented to sit under a title.
func (p *printer) body(text string) {
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if strings.TrimSpace(l) == "" {
			p.blank()
			continue
		}
		p.line("  " + l)
	}
}

// compact writes one diagnostic as a single line: a severity mark, the code,
// and the short form. The full rendering with source and carets is one `:diag`
// away, which keeps the common case readable.
func (p *printer) compact(e report.Entry) {
	mark, colour := severityMark(e.Severity)
	short := e.Short
	if short == "" {
		short = e.Message
	}
	p.line("  " + p.style(colour, mark+" "+e.Code) + "  " + short)
	// A refusal is finished work rather than a failure, and finished work says
	// what to do next. The advice travels with the refusal instead of waiting
	// behind `:diag`, because a construct with no Go equivalent is exactly the
	// moment somebody needs the answer.
	if e.Severity == report.Refuse && e.Advice != "" {
		for _, l := range strings.Split(wrapAt(e.Advice, 72), "\n") {
			p.line("    " + p.style(sgrDim, l))
		}
	}
}

// wrapAt folds prose to a width.
func wrapAt(text string, width int) string {
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

func severityMark(s report.Severity) (mark, colour string) {
	switch s {
	case report.Refuse:
		return "!", sgrRefuse
	case report.Warn:
		return "~", sgrWarn
	default:
		return "-", sgrDim
	}
}

// prompt renders the primary or continuation prompt. Both are six columns so
// that a continued line sits under the line above it.
func (p *printer) prompt(continued bool) string {
	text := "perl> "
	if continued {
		text = " ...> "
	}
	return p.style(sgrPrompt, text)
}
