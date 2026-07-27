package repl

import (
	"bufio"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"
)

// errNoRawMode says this build or this terminal cannot be put into raw mode,
// which is not an error worth reporting as one: the session falls back to
// line-buffered reading and says so once.
var errNoRawMode = errors.New("raw mode is not available")

// editor is the line editor.
//
// It is written here rather than taken from a library on purpose. The two
// maintained Go readline packages are both low-activity dependencies for what
// is a few hundred lines, and this tool's whole pitch is that it has no
// dependencies and generates none. The feature list is deliberately short:
// move by character and by word, home and end, kill to the end of the line and
// kill a word, history up and down, and reverse search over history. Anything
// beyond that is what a real shell is for.
type editor struct {
	in      *os.File
	out     *os.File
	rd      *bufio.Reader
	hist    *history
	restore func()
	color   bool

	buf []rune
	pos int
	// idx walks the history; len(entries) means "the line being typed".
	idx int
	// stash keeps the typed line while history is being browsed.
	stash []rune
}

func newEditor(in, out *os.File, h *history, color bool) (*editor, error) {
	restore, err := makeRaw(in.Fd())
	if err != nil {
		return nil, err
	}
	return &editor{in: in, out: out, rd: bufio.NewReader(in), hist: h, restore: restore, color: color}, nil
}

func (e *editor) fallback() string { return "" }

func (e *editor) close() {
	if e.restore != nil {
		e.restore()
		e.restore = nil
	}
}

// Control bytes the editor acts on.
const (
	keyCtrlA = 1
	keyCtrlB = 2
	keyCtrlC = 3
	keyCtrlD = 4
	keyCtrlE = 5
	keyCtrlF = 6
	keyCtrlH = 8
	keyTab   = 9
	keyLF    = 10
	keyCtrlK = 11
	keyCtrlL = 12
	keyCR    = 13
	keyCtrlN = 14
	keyCtrlP = 16
	keyCtrlR = 18
	keyCtrlU = 21
	keyCtrlW = 23
	keyEsc   = 27
	keyDel   = 127
)

func (e *editor) readLine(prompt string) (string, error) {
	e.buf, e.pos = nil, 0
	e.idx, e.stash = e.hist.len(), nil
	e.render(prompt)

	for {
		r, _, err := e.rd.ReadRune()
		if err != nil {
			e.out.WriteString("\n")
			return "", io.EOF
		}
		switch r {
		case keyCR, keyLF:
			e.out.WriteString("\n")
			return string(e.buf), nil
		case keyCtrlC:
			e.out.WriteString("^C\n")
			return "", errInterrupt
		case keyCtrlD:
			if len(e.buf) == 0 {
				e.out.WriteString("\n")
				return "", io.EOF
			}
			e.delete()
		case keyCtrlA:
			e.pos = 0
		case keyCtrlE:
			e.pos = len(e.buf)
		case keyCtrlB:
			e.left()
		case keyCtrlF:
			e.right()
		case keyCtrlP:
			e.recall(-1)
		case keyCtrlN:
			e.recall(1)
		case keyCtrlK:
			e.buf = e.buf[:e.pos]
		case keyCtrlU:
			e.buf = append([]rune{}, e.buf[e.pos:]...)
			e.pos = 0
		case keyCtrlW:
			e.killWord()
		case keyCtrlL:
			e.out.WriteString("\x1b[H\x1b[2J")
		case keyCtrlR:
			e.search(prompt)
		case keyCtrlH, keyDel:
			e.backspace()
		case keyTab:
			// Tab indents. There is nothing to complete at this prompt:
			// completion would have to guess at Perl the session has not seen.
			e.insert(' ')
			e.insert(' ')
			e.insert(' ')
			e.insert(' ')
		case keyEsc:
			e.escape()
		default:
			if unicode.IsPrint(r) {
				e.insert(r)
			}
		}
		e.render(prompt)
	}
}

// escape decodes the sequences the arrow, home, end and delete keys send.
func (e *editor) escape() {
	first, _, err := e.rd.ReadRune()
	if err != nil {
		return
	}
	switch first {
	case 'b':
		e.pos = wordLeft(e.buf, e.pos)
		return
	case 'f':
		e.pos = wordRight(e.buf, e.pos)
		return
	case '[', 'O':
	default:
		return
	}
	// Collect until the final byte of a CSI sequence.
	var seq []rune
	for i := 0; i < 8; i++ {
		r, _, err := e.rd.ReadRune()
		if err != nil {
			return
		}
		seq = append(seq, r)
		if r >= '@' && r <= '~' {
			break
		}
	}
	switch string(seq) {
	case "A":
		e.recall(-1)
	case "B":
		e.recall(1)
	case "C":
		e.right()
	case "D":
		e.left()
	case "H", "1~", "7~":
		e.pos = 0
	case "F", "4~", "8~":
		e.pos = len(e.buf)
	case "3~":
		e.delete()
	case "1;5C", "1;3C":
		e.pos = wordRight(e.buf, e.pos)
	case "1;5D", "1;3D":
		e.pos = wordLeft(e.buf, e.pos)
	}
}

func (e *editor) insert(r rune) {
	e.buf = append(e.buf, 0)
	copy(e.buf[e.pos+1:], e.buf[e.pos:])
	e.buf[e.pos] = r
	e.pos++
}

func (e *editor) backspace() {
	if e.pos == 0 {
		return
	}
	e.buf = append(e.buf[:e.pos-1], e.buf[e.pos:]...)
	e.pos--
}

func (e *editor) delete() {
	if e.pos >= len(e.buf) {
		return
	}
	e.buf = append(e.buf[:e.pos], e.buf[e.pos+1:]...)
}

func (e *editor) left() {
	if e.pos > 0 {
		e.pos--
	}
}

func (e *editor) right() {
	if e.pos < len(e.buf) {
		e.pos++
	}
}

func (e *editor) killWord() {
	to := wordLeft(e.buf, e.pos)
	e.buf = append(append([]rune{}, e.buf[:to]...), e.buf[e.pos:]...)
	e.pos = to
}

func wordLeft(buf []rune, pos int) int {
	for pos > 0 && isWordBreak(buf[pos-1]) {
		pos--
	}
	for pos > 0 && !isWordBreak(buf[pos-1]) {
		pos--
	}
	return pos
}

func wordRight(buf []rune, pos int) int {
	for pos < len(buf) && isWordBreak(buf[pos]) {
		pos++
	}
	for pos < len(buf) && !isWordBreak(buf[pos]) {
		pos++
	}
	return pos
}

func isWordBreak(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
}

// recall steps through history. The line being typed is kept, so browsing away
// from it and back does not lose it.
func (e *editor) recall(step int) {
	if e.hist.len() == 0 {
		return
	}
	if e.idx == e.hist.len() {
		e.stash = append([]rune{}, e.buf...)
	}
	next := e.idx + step
	if next < 0 {
		next = 0
	}
	if next > e.hist.len() {
		next = e.hist.len()
	}
	e.idx = next
	if e.idx == e.hist.len() {
		e.buf = append([]rune{}, e.stash...)
	} else {
		e.buf = []rune(e.hist.at(e.idx))
	}
	e.pos = len(e.buf)
}

// search is Ctrl-R: type a substring, see the most recent entry containing it,
// press return to accept it or any editing key to leave the search where it is.
func (e *editor) search(prompt string) {
	var needle []rune
	for {
		match, found := e.lastMatch(string(needle))
		e.renderSearch(string(needle), match, found)
		r, _, err := e.rd.ReadRune()
		if err != nil {
			return
		}
		switch r {
		case keyCR, keyLF, keyEsc:
			if found {
				e.buf = []rune(match)
				e.pos = len(e.buf)
			}
			return
		case keyCtrlC:
			return
		case keyCtrlH, keyDel:
			if len(needle) > 0 {
				needle = needle[:len(needle)-1]
			}
		case keyCtrlR:
			// Repeat search is not offered; one hit is the useful case and a
			// second index would need its own state to stay honest.
		default:
			if unicode.IsPrint(r) {
				needle = append(needle, r)
			}
		}
	}
}

func (e *editor) lastMatch(needle string) (string, bool) {
	if needle == "" {
		return "", false
	}
	for i := e.hist.len() - 1; i >= 0; i-- {
		if strings.Contains(e.hist.at(i), needle) {
			return e.hist.at(i), true
		}
	}
	return "", false
}

func (e *editor) renderSearch(needle, match string, found bool) {
	line := "(reverse-search)`" + needle + "': "
	if found {
		line += oneLine(match)
	} else if needle != "" {
		line += "(no match)"
	}
	e.out.WriteString("\r\x1b[K" + line)
}

// render redraws the prompt and the line, scrolling the window horizontally
// when the line is longer than the terminal is wide.
func (e *editor) render(prompt string) {
	width := terminalWidth(e.out.Fd())
	pw := visibleWidth(prompt)
	avail := width - pw - 1
	if avail < 8 {
		avail = 8
	}

	start := 0
	if e.pos > avail {
		start = e.pos - avail
	}
	end := len(e.buf)
	if end > start+avail {
		end = start + avail
	}

	var b strings.Builder
	b.WriteString("\r\x1b[K")
	b.WriteString(prompt)
	for _, r := range e.buf[start:end] {
		if r == '\n' {
			// A recalled multi-line snippet is one entry. Showing the break
			// keeps the display honest about what pressing return will send.
			b.WriteRune('↵')
			continue
		}
		b.WriteRune(r)
	}
	b.WriteString("\r")
	if col := pw + (e.pos - start); col > 0 {
		b.WriteString("\x1b[")
		b.WriteString(itoa(col))
		b.WriteString("C")
	}
	e.out.WriteString(b.String())
}

func oneLine(s string) string {
	return strings.ReplaceAll(s, "\n", "↵")
}

// sgr matches the escape sequences a coloured prompt carries, which take no
// columns on screen and must not be counted as though they did.
var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func visibleWidth(s string) int {
	return len([]rune(sgr.ReplaceAllString(s, "")))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
