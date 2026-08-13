package teach

import (
	"fmt"
	"path"
	"strings"
)

// Markdown plumbing for the generated documentation bundle. Nothing here knows
// what a conversion is; it is the small amount of machinery the document
// builders in docgen.go stand on.

// md accumulates one Markdown document. Its methods keep the blank lines
// between blocks consistent, which is what makes the output render the same way
// in a terminal pager as on a web page.
type md struct {
	b strings.Builder
}

// h writes a heading at the given level.
func (m *md) h(level int, format string, args ...any) {
	m.gap()
	m.b.WriteString(strings.Repeat("#", level))
	m.b.WriteByte(' ')
	fmt.Fprintf(&m.b, format, args...)
	m.b.WriteByte('\n')
}

// p writes one paragraph.
func (m *md) p(format string, args ...any) {
	text := strings.TrimSpace(fmt.Sprintf(format, args...))
	if text == "" {
		return
	}
	m.gap()
	m.b.WriteString(text)
	m.b.WriteByte('\n')
}

// raw writes a block of already-formatted Markdown.
func (m *md) raw(text string) {
	text = strings.TrimRight(text, "\n")
	if strings.TrimSpace(text) == "" {
		return
	}
	m.gap()
	m.b.WriteString(text)
	m.b.WriteByte('\n')
}

// bullet writes one item of an unordered list. Consecutive bullets form one
// list, because gap only separates blocks that are not both list items.
func (m *md) bullet(format string, args ...any) {
	m.listItem("- " + strings.TrimSpace(fmt.Sprintf(format, args...)))
}

// numbered writes one item of an ordered list.
func (m *md) numbered(n int, format string, args ...any) {
	m.listItem(fmt.Sprintf("%d. ", n) + strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func (m *md) listItem(line string) {
	if s := m.b.String(); s != "" && !endsWithListItem(s) {
		m.gap()
	}
	m.b.WriteString(line)
	m.b.WriteByte('\n')
}

// table writes a header row and its body rows. Callers keep tables narrow; a
// table wider than a terminal is worse than a list.
func (m *md) table(header []string, rows [][]string) {
	m.gap()
	m.b.WriteString("| " + strings.Join(header, " | ") + " |\n")
	m.b.WriteString("|" + strings.Repeat(" --- |", len(header)) + "\n")
	for _, row := range rows {
		m.b.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
}

// fence writes a fenced code block, widening the fence if the body contains one.
func (m *md) fence(lang, body string) {
	body = strings.TrimRight(body, "\n")
	if strings.TrimSpace(body) == "" {
		return
	}
	f := fenceFor(body)
	m.gap()
	m.b.WriteString(f + lang + "\n")
	m.b.WriteString(body)
	m.b.WriteString("\n" + f + "\n")
}

// rule writes a horizontal rule, used to separate the long repeated sections of
// the walkthrough.
func (m *md) rule() {
	m.gap()
	m.b.WriteString("---\n")
}

// gap makes sure the next block starts after exactly one blank line.
func (m *md) gap() {
	s := m.b.String()
	if s == "" {
		return
	}
	if !strings.HasSuffix(s, "\n\n") {
		m.b.WriteByte('\n')
	}
}

// String returns the document, ending in exactly one newline.
func (m *md) String() string {
	return strings.TrimRight(m.b.String(), "\n") + "\n"
}

// endsWithListItem reports whether the last line written is a list item, so
// that a following item joins the same list instead of starting a new one.
func endsWithListItem(s string) bool {
	s = strings.TrimSuffix(s, "\n")
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	if strings.HasPrefix(s, "- ") {
		return true
	}
	digits := 0
	for digits < len(s) && s[digits] >= '0' && s[digits] <= '9' {
		digits++
	}
	return digits > 0 && strings.HasPrefix(s[digits:], ". ")
}

// fenceFor returns a fence long enough to quote body, which may itself contain
// fenced blocks (a concept body quoted inside another document, for instance).
func fenceFor(body string) string {
	longest := 0
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		n := 0
		for n < len(trimmed) && trimmed[n] == '`' {
			n++
		}
		if n > longest {
			longest = n
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

// rel returns the link target to write in fromFile so that it points at toFile,
// where both are bundle keys relative to the generated project root.
func rel(fromFile, toFile string) string {
	from := splitDir(path.Dir(fromFile))
	to := splitDir(path.Dir(toFile))

	common := 0
	for common < len(from) && common < len(to) && from[common] == to[common] {
		common++
	}
	parts := make([]string, 0, len(from)-common+len(to)-common+1)
	for range from[common:] {
		parts = append(parts, "..")
	}
	parts = append(parts, to[common:]...)
	parts = append(parts, path.Base(toFile))
	return path.Join(parts...)
}

func splitDir(dir string) []string {
	if dir == "." || dir == "" || dir == "/" {
		return nil
	}
	return strings.Split(strings.Trim(dir, "/"), "/")
}

// link renders a Markdown link.
func link(text, target string) string {
	return "[" + text + "](" + target + ")"
}

// mdLink is one Markdown link found in a document.
type mdLink struct {
	text   string
	target string
	start  int // byte offset of the opening bracket
	end    int // byte offset just past the closing parenthesis
}

// findLinks returns the Markdown links in a document, ignoring anything inside
// a fenced block or an inline code span. Both exclusions matter: Go generics
// inside inline code (`func Max[T cmp.Ordered](...)`) look exactly like a link
// to a naive scan.
func findLinks(doc string) []mdLink {
	var out []mdLink
	for _, span := range proseSpans(doc) {
		text := doc[span[0]:span[1]]
		for i := 0; i < len(text); i++ {
			if text[i] != '[' {
				continue
			}
			close := strings.IndexByte(text[i:], ']')
			if close < 0 {
				break
			}
			close += i
			if close+1 >= len(text) || text[close+1] != '(' {
				continue
			}
			end := strings.IndexByte(text[close:], ')')
			if end < 0 {
				break
			}
			end += close
			label := text[i+1 : close]
			if strings.ContainsAny(label, "[]\n") {
				continue
			}
			out = append(out, mdLink{
				text:   label,
				target: text[close+2 : end],
				start:  span[0] + i,
				end:    span[0] + end + 1,
			})
			i = end
		}
	}
	return out
}

// proseSpans returns the byte ranges of doc that are ordinary prose: not inside
// a fenced code block and not inside an inline code span.
func proseSpans(doc string) [][2]int {
	var spans [][2]int
	start := 0
	offset := 0
	fence := ""
	inCode := false

	for _, line := range strings.SplitAfter(doc, "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\n"))
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			offset += len(line)
			start = offset
			continue
		}
		if marker := leadingBackticks(trimmed); marker >= 3 {
			// A fence opens here; everything before it on earlier lines is prose.
			spans = appendSpan(spans, start, offset)
			fence = trimmed[:marker]
			inCode = false
			offset += len(line)
			start = offset
			continue
		}
		// Split the line at inline code spans.
		for i := 0; i < len(line); i++ {
			if line[i] != '`' {
				continue
			}
			if inCode {
				inCode = false
				start = offset + i + 1
				continue
			}
			inCode = true
			spans = appendSpan(spans, start, offset+i)
		}
		offset += len(line)
		if inCode {
			// An unterminated span ends at the line break, as in Markdown.
			inCode = false
			start = offset
		}
	}
	return appendSpan(spans, start, len(doc))
}

func appendSpan(spans [][2]int, from, to int) [][2]int {
	if to > from {
		spans = append(spans, [2]int{from, to})
	}
	return spans
}

func leadingBackticks(s string) int {
	n := 0
	for n < len(s) && s[n] == '`' {
		n++
	}
	return n
}

// sentence shortens text to a single line suitable for an index entry: the
// first sentence or clause, hard-capped so that no entry wraps more than twice
// in a terminal.
func sentence(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return ""
	}

	const (
		minCut = 40
		maxLen = 200
	)
	// Cut at the first sentence or clause end that is outside a code span and
	// is not an abbreviation's full stop.
	ticks := 0
	for i := 0; i+1 < len(text) && i < maxLen; i++ {
		switch {
		case text[i] == '`':
			ticks++
		case ticks%2 == 1, i < minCut, text[i+1] != ' ':
			// inside code, too early, or not the end of a word
		case text[i] == '.' && abbreviated(text[:i]):
			// "3 a.m. is", "e.g. this": not a sentence end
		case strings.ContainsRune(".;:!?", rune(text[i])):
			return balanceTicks(strings.TrimRight(text[:i], " ,;:")) + "."
		}
	}
	if len(text) <= maxLen {
		if strings.HasSuffix(text, ".") {
			return text
		}
		return balanceTicks(text) + "."
	}
	trimmed := text[:maxLen]
	// A sentence that runs long usually has a clause break before the cap,
	// and ending at one reads as a summary where a mid-phrase ellipsis reads
	// as an accident.
	if i := strings.Index(trimmed, " - "); i >= minCut {
		head := strings.TrimRight(trimmed[:i], " ,;:")
		if !strings.HasSuffix(head, ".") {
			head += "."
		}
		return balanceTicks(head)
	}
	if i := strings.LastIndexByte(trimmed, ' '); i > minCut {
		trimmed = trimmed[:i]
	}
	return balanceTicks(strings.TrimRight(trimmed, " ,;:.")) + "..."
}

// abbreviated reports whether text ends in an abbreviation rather than a
// sentence, so that "at 3 a.m." and "e.g." do not cut a summary in half.
func abbreviated(text string) bool {
	word := text
	if i := strings.LastIndexAny(word, " ("); i >= 0 {
		word = word[i+1:]
	}
	return len(word) <= 2 || strings.Contains(word, ".")
}

// balanceTicks closes a code span left open by truncation, so a shortened line
// cannot swallow the rest of a document.
func balanceTicks(s string) string {
	if strings.Count(s, "`")%2 == 1 {
		return s + "`"
	}
	return s
}

// plural returns "" for one and "s" otherwise, for counted nouns.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// countLines returns the number of lines in a source text.
func countLines(src string) int {
	src = strings.TrimRight(src, "\n")
	if src == "" {
		return 0
	}
	return strings.Count(src, "\n") + 1
}
