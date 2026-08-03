package diag

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"perl2golang/internal/report"
)

// Options control how a diagnostic is drawn.
type Options struct {
	// Color turns SGR sequences on. The coloured and the plain rendering are
	// byte-identical once the sequences are stripped, so colour is never load
	// bearing and a golden test only needs the plain form.
	Color bool
	// Width is the column the footer bodies wrap at. Zero means 80.
	Width int
}

// SGR sequences, 8-colour only, so this survives a terminal from 1990 and a CI
// log from today.
const (
	sgrBold   = "1"
	sgrDim    = "2"
	sgrBlue   = "34"
	sgrRefuse = "1;31"
	sgrWarn   = "1;33"
	sgrNote   = "1;34"
)

// defaultWidth is the column footer bodies wrap at when nothing says otherwise.
const defaultWidth = 80

// Render writes one diagnostic in the block form:
//
//	refused[P2G4004]: lookahead `(?!#)` is not available in Go's `regexp` package
//	  --> logwatch.pl:88:27
//	   |
//	88 |     next unless $line =~ /^(?!#)\s*(\S+)\s+(\d+)/;
//	   |                           ^^^^^
//	   |
//	   = not converted: the match site is marked and yields no match
//	   = try: drop the lookahead and test the prefix with `strings.HasPrefix`
//	   = learn: regexp-is-re2   (perl2golang explain regexp-is-re2)
//
// srcLines is the input file split into lines, with line n at index n-1. It may
// be nil, in which case the excerpt block is left out and the footers follow the
// location line. A position that does not exist in srcLines is treated the same
// way: a diagnostic with a wrong position still has to print.
//
// The `converted:` and `cost:` footers come from the registry, so they say the
// same thing everywhere the code appears.
func Render(out io.Writer, e report.Entry, file string, srcLines []string, o Options) error {
	var b strings.Builder
	st := styler{on: o.Color}
	width := o.Width
	if width <= 0 {
		width = defaultWidth
	}

	message := e.Message
	if message == "" {
		message = e.Short
	}
	head := e.Severity.String()
	if e.Code != "" {
		head += "[" + e.Code + "]"
	}
	b.WriteString(st.s(severityColour(e.Severity), e.Severity.String()))
	if e.Code != "" {
		b.WriteString(st.s(sgrDim, "["+e.Code+"]"))
	}
	b.WriteString(":")
	// A message long enough to wrap is a message that wants cutting, but it
	// still has to print inside the width, with the continuations indented far
	// enough to read as one sentence.
	first, rest := splitFirst(wrapFirst(message, width-len(head)-2, width-4))
	if first != "" {
		b.WriteByte(' ')
		b.WriteString(st.s(sgrBold, first))
	}
	b.WriteByte('\n')
	for _, line := range rest {
		b.WriteString("    ")
		b.WriteString(st.s(sgrBold, line))
		b.WriteByte('\n')
	}

	// An entry from a module the script pulled in names its own file, and
	// the source to quote from is not the one this call was given.
	if e.File != "" && e.File != file {
		file, srcLines = e.File, nil
	}

	gw := gutterWidth(e.Line)
	pad := strings.Repeat(" ", gw)
	b.WriteString(pad)
	b.WriteString(st.s(sgrBlue, "-->"))
	b.WriteByte(' ')
	b.WriteString(location(file, e.Line, e.Col))
	b.WriteByte('\n')

	if src, ok := sourceLine(srcLines, e.Line); ok {
		bar := st.s(sgrBlue, "|")
		blank := pad + " " + bar + "\n"
		b.WriteString(blank)
		b.WriteString(fmt.Sprintf("%*d ", gw, e.Line))
		b.WriteString(bar)
		if src != "" {
			b.WriteString(" " + src)
		}
		b.WriteByte('\n')
		col, n := caretRun(src, e)
		b.WriteString(pad + " " + bar + " ")
		b.WriteString(strings.Repeat(" ", col-1))
		b.WriteString(st.s(severityColour(e.Severity), strings.Repeat("^", n)))
		b.WriteByte('\n')
		b.WriteString(blank)
	}

	label := "converted"
	if e.Severity == report.Refuse {
		label = "not converted"
	}
	reg, _ := Lookup(Code(e.Code))
	footer(&b, st, gw, width, label, reg.Converted)
	footer(&b, st, gw, width, "cost", reg.Cost)
	footer(&b, st, gw, width, "try", e.Advice)
	for _, id := range e.Concepts {
		learn(&b, st, gw, width, id)
	}

	_, err := io.WriteString(out, b.String())
	return err
}

// Compact renders one diagnostic as a single line in the shape editors already
// parse: file:line:col: severity[CODE]: message.
func Compact(e report.Entry, file string) string {
	message := e.Message
	if message == "" {
		message = e.Short
	}
	if e.File != "" {
		file = e.File
	}
	return location(file, e.Line, e.Col) + ": " + e.Severity.String() + "[" + e.Code + "]: " + message
}

// footer writes one `= label: body` line, wrapping the body at the render width
// and indenting the continuations to where the body starts.
func footer(b *strings.Builder, st styler, gutter, width int, label, body string) {
	if body == "" {
		return
	}
	lead := strings.Repeat(" ", gutter+1) + "= " + label + ": "
	indent := strings.Repeat(" ", len(lead))
	for i, line := range wrapText(body, width-len(lead)) {
		if i == 0 {
			b.WriteString(strings.Repeat(" ", gutter+1))
			b.WriteString("= ")
			b.WriteString(st.s(sgrBold, label+":"))
			b.WriteByte(' ')
		} else {
			b.WriteString(indent)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
}

// learn writes the `= learn:` line for one teaching concept, keeping a wide gap
// between the concept id and the command that prints it, and moving the command
// to its own line when the two do not fit side by side.
func learn(b *strings.Builder, st styler, gutter, width int, id string) {
	const gap = "   "
	lead := strings.Repeat(" ", gutter+1) + "= learn: "
	hint := "(perl2golang explain " + id + ")"
	b.WriteString(strings.Repeat(" ", gutter+1))
	b.WriteString("= ")
	b.WriteString(st.s(sgrBold, "learn:"))
	b.WriteByte(' ')
	b.WriteString(id)
	if len(lead)+len(id)+len(gap)+len(hint) <= width {
		b.WriteString(gap + hint + "\n")
		return
	}
	b.WriteString("\n" + strings.Repeat(" ", len(lead)) + hint + "\n")
}

// wrapText breaks text on spaces so that no line is wider than width. A word
// longer than width takes a line of its own rather than being cut.
func wrapText(text string, width int) []string {
	return wrapFirst(text, width, width)
}

// wrapFirst wraps text where the first line has less room than the rest, which
// is what a header that starts after `warning[P2G4004]: ` needs.
func wrapFirst(text string, first, rest int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	if first < 1 {
		first = 1
	}
	if rest < 1 {
		rest = 1
	}
	lines := []string{}
	cur, width := words[0], first
	for _, w := range words[1:] {
		if utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w) <= width {
			cur += " " + w
			continue
		}
		lines = append(lines, cur)
		cur, width = w, rest
	}
	return append(lines, cur)
}

// splitFirst separates the first line from the continuations.
func splitFirst(lines []string) (string, []string) {
	if len(lines) == 0 {
		return "", nil
	}
	return lines[0], lines[1:]
}

// location renders the address of a diagnostic, dropping the parts that are not
// known. An entry with no position still prints the file it came from.
func location(file string, line, col int) string {
	if file == "" {
		file = "<input>"
	}
	switch {
	case line > 0 && col > 0:
		return fmt.Sprintf("%s:%d:%d", file, line, col)
	case line > 0:
		return fmt.Sprintf("%s:%d", file, line)
	default:
		return file
	}
}

// gutterWidth is the width of the line-number column, at least two digits so
// that short files and long files line up the same way.
func gutterWidth(line int) int {
	w := len(fmt.Sprint(line))
	if line < 0 {
		w = 1
	}
	if w < 2 {
		return 2
	}
	return w
}

// sourceLine returns the excerpt line for a position, with tabs expanded to one
// space each so that a column counted in runes is also a column on screen.
func sourceLine(srcLines []string, line int) (string, bool) {
	if line < 1 || line > len(srcLines) {
		return "", false
	}
	s := strings.TrimRight(srcLines[line-1], "\r\n")
	return strings.ReplaceAll(s, "\t", " "), true
}

// caretRun decides where the carets start and how many there are. The run
// covers the Perl text the entry carries when that text is on one line;
// anything else gets a single caret, because a run that guessed would point at
// the wrong thing.
func caretRun(src string, e report.Entry) (col, n int) {
	col = e.Col
	if col < 1 {
		col = 1
	}
	n = 1
	if e.Perl != "" && !strings.ContainsAny(e.Perl, "\r\n") {
		n = utf8.RuneCountInString(e.Perl)
	}
	if n < 1 {
		n = 1
	}
	if avail := utf8.RuneCountInString(src) - (col - 1); avail >= 1 && n > avail {
		n = avail
	}
	return col, n
}

// styler adds SGR sequences, or does not. Every styled span is wrapped whole,
// so removing the sequences gives the plain rendering back byte for byte.
type styler struct{ on bool }

func (st styler) s(code, text string) string {
	if !st.on || code == "" || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// severityColour is the 8-colour SGR code for a severity word and for the caret
// run underneath it.
func severityColour(s report.Severity) string {
	switch s {
	case report.Refuse:
		return sgrRefuse
	case report.Warn:
		return sgrWarn
	default:
		return sgrNote
	}
}

// sgrPattern matches the escape sequences styler writes.
var sgrPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripSGR removes every SGR sequence, which turns a coloured rendering into
// the plain one.
func stripSGR(s string) string {
	return sgrPattern.ReplaceAllString(s, "")
}

// ColorEnabled decides whether output to w should carry colour.
//
// flag is the value of --color: "never" and "always" are answers on their own,
// and "auto" or an empty string ask the environment. NO_COLOR set to any value,
// including the empty string, turns colour off, as does TERM=dumb and a writer
// that is not a character device.
func ColorEnabled(w io.Writer, flag string) bool {
	switch flag {
	case "never":
		return false
	case "always":
		return true
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTerminal(w)
}

// isTerminal reports whether w is a character device, which is the closest a
// program can get to "someone is looking at this" without a dependency.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
