// Package repl is the interactive session: type Perl, see the Go it becomes,
// and see the Go concepts behind it.
//
// It is the fastest teaching surface in the tool because the loop is seconds
// long rather than a whole file long. Everything it shows comes from the same
// pipeline a file conversion runs, so anything learned here is true of the
// converter, and anything the converter cannot do is refused here in the same
// words.
package repl

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"perl2go/internal/convert"
	"perl2go/internal/report"
	"perl2go/internal/teach"
)

// Options configure one session.
type Options struct {
	// Stdin is where snippets come from. A file or a pipe works exactly like
	// a terminal, minus the line editing.
	Stdin io.Reader
	// Stdout carries the whole transcript: prompts, echoed input, generated
	// Go, diagnostics and teaching notes.
	Stdout io.Writer
	// Stderr carries only failures that end the session.
	Stderr io.Writer

	// Color turns the escape sequences on. The caller decides, from whether
	// Stdout is a terminal and what NO_COLOR says.
	Color bool
	// Interactive asks for the raw-mode line editor. It is only honoured when
	// the streams really are a terminal.
	Interactive bool

	// Mode is "clean" or "annotated": which of the two generated programs the
	// session shows.
	Mode string
	// Notes prints the concept line under each snippet.
	Notes bool

	// NoHistory suppresses reading and writing the history file.
	NoHistory bool
	// HistoryFile overrides where history is kept.
	HistoryFile string

	// Load names a file whose lines are typed into the session before the
	// prompt is handed over.
	Load string
}

// Exit statuses. A session that ran is a success whatever the Perl in it did,
// because a refusal printed and understood is the tool working.
const (
	exitOK     = 0
	exitFailed = 2
)

// repl is one running session.
type repl struct {
	opt  Options
	p    *printer
	s    *session
	kb   *teach.KB
	src  lineSource
	hist *history

	mode  string
	notes bool
	done  bool
}

// Run holds one session open until it is asked to leave, and returns the
// process exit status.
//
// Nothing inside the loop can end it early. A snippet that does not parse, a
// meta command that does not exist and a construct with no Go equivalent are
// all ordinary events that print something and return to the prompt.
func Run(o Options) int {
	if o.Mode == "" {
		o.Mode = "clean"
	}
	r := &repl{
		opt:   o,
		p:     &printer{out: o.Stdout, color: o.Color},
		s:     newSession(),
		kb:    teach.Load(),
		mode:  o.Mode,
		notes: o.Notes,
	}

	r.hist = openHistory(o)
	defer r.hist.close()
	r.src = newLineSource(o, r.hist)
	defer r.src.close()

	r.banner()
	if o.Load != "" {
		if code := r.loadFile(o.Load); code != exitOK {
			return code
		}
	}
	r.loop()
	return exitOK
}

// banner is the one line a session opens with. It names the version, because
// the Go the tool produces changes between them, and it names the two ways out.
func (r *repl) banner() {
	r.p.line(fmt.Sprintf("perl2go %s  type Perl, see the Go. :help for commands, :quit to leave.", convert.Version))
	if fb := r.src.fallback(); fb != "" {
		r.p.note("%s", fb)
	}
	r.p.blank()
}

// loop reads snippets until the session ends.
func (r *repl) loop() {
	for !r.done {
		text, meta, err := r.read()
		switch {
		case errors.Is(err, io.EOF):
			r.p.blank()
			return
		case err != nil:
			r.p.note("input stopped: %v", err)
			return
		}
		if meta != "" {
			r.hist.add(meta)
			r.command(meta)
			if !r.done {
				r.p.blank()
			}
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		r.hist.add(text)
		r.run(text)
		r.p.blank()
	}
}

// errInterrupt is what the line editor returns for Ctrl-C.
var errInterrupt = errors.New("interrupted")

// read gathers one logical unit of input: either a meta command, or a Perl
// snippet complete enough to convert.
//
// Continuation is decided by the text, not by the user. Nothing has to be
// typed to say "there is more coming", which matters because a Perl developer
// already knows when a statement has ended and should not have to say so a
// second time.
func (r *repl) read() (text, meta string, err error) {
	var buf []string
	blanks := 0
	reminded := false

	for {
		line, err := r.src.readLine(r.p.prompt(len(buf) > 0))
		switch {
		case errors.Is(err, errInterrupt):
			if len(buf) == 0 {
				r.p.note("(nothing to discard; :quit leaves)")
				continue
			}
			r.p.note("discarded %s", plural(len(buf), "pending line"))
			buf, blanks, reminded = nil, 0, false
			continue
		case errors.Is(err, io.EOF):
			if len(buf) > 0 {
				r.p.note("input ended with %s unfinished; discarded", plural(len(buf), "line"))
			}
			return "", "", io.EOF
		case err != nil:
			return "", "", err
		}

		if len(buf) == 0 && strings.HasPrefix(strings.TrimSpace(line), ":") {
			return "", strings.TrimSpace(line), nil
		}

		if strings.TrimSpace(line) == "" {
			// A blank line does not submit: Perl statements end in a
			// semicolon, and a blank line inside a sub is ordinary
			// formatting. Two in a row mean the typist has given up on
			// whatever is open.
			if len(buf) == 0 {
				continue
			}
			blanks++
			if blanks >= 2 {
				r.p.note("blank line twice: discarded %s", plural(len(buf), "pending line"))
				buf, blanks, reminded = nil, 0, false
				continue
			}
			buf = append(buf, "")
			continue
		}
		blanks = 0
		buf = append(buf, line)

		open, incomplete := continuation(strings.Join(buf, "\n"))
		if !incomplete {
			return strings.Join(buf, "\n"), "", nil
		}
		if len(buf) >= 3 && !reminded {
			reminded = true
			r.p.note("(%s opened at line %d; a blank line twice discards it)", open.what, open.line)
		}
	}
}

// run converts one snippet in the context of everything typed before it, and
// prints what came out.
func (r *repl) run(text string) {
	t, rej := r.s.submit(text)
	if rej != nil {
		r.reject(text, rej)
		return
	}
	r.show(t)
}

// reject reports a snippet that never entered the session program.
//
// The session is left exactly as it was. One mistyped line cannot cost
// somebody the twenty they typed before it, and it cannot silently change the
// meaning of the ones that follow.
func (r *repl) reject(text string, rej *rejection) {
	if rej.internal != "" {
		// A converter panic is a defect in this tool. It is caught per
		// snippet, said plainly, and the session carries on.
		r.p.compact(internalEntry())
		r.p.note("%s", firstLine(rej.internal))
		r.p.note("the session is unchanged; please report the snippet above")
		return
	}
	lines := strings.Split(text, "\n")
	for i, d := range rej.diags {
		if i == 3 {
			r.p.note("and %d more", len(rej.diags)-3)
			break
		}
		r.p.line("  " + r.p.style(sgrRefuse, fmt.Sprintf("!  %d:%d", d.Pos.Line, d.Pos.Col)) + "  " + d.Msg)
		if src, ok := nth(lines, d.Pos.Line); ok {
			r.p.line("  " + r.p.style(sgrDim, "| ") + src)
			r.p.line("  " + r.p.style(sgrDim, "| "+caret(src, d.Pos.Col)))
		}
	}
	r.p.note("not added to the session; the %s already typed %s untouched",
		plural(r.s.lineCount(), "line"), verb(r.s.lineCount()))
}

// show prints one accepted snippet's result: what the converter had to say,
// the Go it produced, and the concepts behind it.
func (r *repl) show(t *turn) {
	// Diagnostics first, so that a refusal is read before the code it
	// explains rather than after it.
	shown := 0
	for _, e := range t.entries {
		if e.Severity < report.Warn {
			continue
		}
		if shown == 3 {
			r.p.note("and %d more; :diag shows them all", len(t.entries)-shown)
			break
		}
		r.p.compact(e)
		shown++
	}

	code := t.added
	if r.mode == "annotated" {
		code = t.annotated
	}
	if len(code) == 0 {
		r.p.note("(no Go for that snippet)")
	} else {
		r.p.code(code)
	}

	if t.removed > 0 {
		// Re-inference can change a decision printed several snippets ago.
		// Saying so is the difference between a teaching tool and one that
		// quietly contradicts itself.
		r.p.note("(that replaced %s shown earlier; :go full shows the session as it stands)",
			plural(t.removed, "line"))
	}
	if len(t.helpers) > 0 {
		r.p.note("(support code added: %s; :go full includes it)", strings.Join(t.helpers, ", "))
	}
	if r.notes {
		r.conceptLine(t)
	}
}

// conceptLine is the whole teaching surface in its resting state: one line,
// naming what this snippet touched that the session has not touched before.
//
// One line, because the second and third time a session names the same concept
// the reader has stopped seeing it, and the noise costs more than the reminder
// is worth. Everything behind the line is one `:explain` away.
func (r *repl) conceptLine(t *turn) {
	if len(t.concepts) == 0 {
		return
	}
	const shown = 2
	names := t.concepts
	extra := ""
	if len(names) > shown {
		extra = fmt.Sprintf("+%d more; ", len(names)-shown)
		names = names[:shown]
	}
	hint := extra + ":explain to expand"
	if len(t.entries) > 0 {
		hint += ", :diag for the full note"
	}
	r.p.line("  " + r.p.style(sgrDim, "concepts: ") + strings.Join(names, ", ") +
		"  " + r.p.style(sgrDim, "("+hint+")"))
}

// loadFile types a file's lines into the session, which is how a demo is
// reproduced and how a bug report is replayed.
func (r *repl) loadFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(r.opt.Stderr, "perl2go repl: %v\n", err)
		return exitFailed
	}
	r.replay(string(data))
	return exitOK
}

// replay feeds text through the same reader the keyboard uses, so a loaded
// file and a typed session take exactly one code path.
func (r *repl) replay(text string) {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	var buf []string
	for _, line := range lines {
		if len(buf) == 0 && strings.HasPrefix(strings.TrimSpace(line), ":") {
			r.p.line(r.p.prompt(false) + line)
			r.command(strings.TrimSpace(line))
			continue
		}
		r.p.line(r.p.prompt(len(buf) > 0) + line)
		if strings.TrimSpace(line) == "" && len(buf) == 0 {
			continue
		}
		buf = append(buf, line)
		if _, open := continuation(strings.Join(buf, "\n")); open {
			continue
		}
		r.run(strings.Join(buf, "\n"))
		r.p.blank()
		buf = nil
	}
	if len(buf) > 0 {
		r.p.note("the file ended with %s unfinished; discarded", plural(len(buf), "line"))
	}
}

// defaultHistoryPath is where a session keeps its history when nothing says
// otherwise. State rather than config, because it is written by the program
// and never edited by hand.
func defaultHistoryPath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "perl2go", "history")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "perl2go", "history")
}

// verb agrees with a count, so no message ever says "1 line are".
func verb(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

// plural renders a count with its noun, so no message ever says "1 lines".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// firstLine trims a multi-line error down to something a prompt can show.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// nth returns the 1-based line n of lines.
func nth(lines []string, n int) (string, bool) {
	if n < 1 || n > len(lines) {
		return "", false
	}
	return lines[n-1], true
}

// caret builds the marker line that points at a column, keeping tabs so the
// marker lines up under a tab-indented source line.
func caret(src string, col int) string {
	if col < 1 {
		col = 1
	}
	var b strings.Builder
	for i := 0; i < col-1; i++ {
		if i < len(src) && src[i] == '\t' {
			b.WriteByte('\t')
			continue
		}
		b.WriteByte(' ')
	}
	b.WriteByte('^')
	return b.String()
}
