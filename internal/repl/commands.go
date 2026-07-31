package repl

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"

	"perl2go/internal/diag"
	"perl2go/internal/report"
	"perl2go/internal/teach"
)

// meta is one command available at the prompt.
//
// Every one of them does a job nothing else does. Three that were considered
// and left out: `:type EXPR`, because `:vars` answers the same question
// without a second inference path to keep honest; `:run`, because running the
// Go needs a toolchain and a temporary module, and the point here is reading
// the Go rather than executing it; and a shell escape, because a tool whose
// central promise is that it never runs your Perl should not offer to run
// anything else either.
type meta struct {
	name string
	arg  string
	help string
	run  func(r *repl, arg string)
}

// commands is the closed set, in the order :help prints them. It is a
// function rather than a variable because :help is itself one of the entries.
func commands() []meta {
	return []meta{
		{name: ":help", help: "this list", run: (*repl).cmdHelp},
		{name: ":go", arg: "[full]", help: "reprint the last Go, or the whole session ready to paste", run: (*repl).cmdGo},
		{name: ":explain", arg: "[WHAT]", help: "expand a concept, a P2G code, or the last snippet's concepts", run: (*repl).cmdExplain},
		{name: ":concepts", help: "every concept this session has touched, with its title", run: (*repl).cmdConcepts},
		{name: ":why", help: "the converter's reasoning for the last snippet", run: (*repl).cmdWhy},
		{name: ":diag", help: "the last snippet's diagnostics in full, with source and carets", run: (*repl).cmdDiag},
		{name: ":vars", help: "the variables in scope and the Go type inferred for each", run: (*repl).cmdVars},
		{name: ":perl", help: "the Perl the session holds so far", run: (*repl).cmdPerl},
		{name: ":mode", arg: "[clean|annotated]", help: "switch between plain Go and Go with the reasoning in comments", run: (*repl).cmdMode},
		{name: ":notes", arg: "on|off", help: "show or hide the concept line", run: (*repl).cmdNotes},
		{name: ":save", arg: "FILE", help: "write the session's Perl to a file", run: (*repl).cmdSave},
		{name: ":load", arg: "FILE", help: "type the lines of a file into the session", run: (*repl).cmdLoad},
		{name: ":reset", help: "forget the session program and start again", run: (*repl).cmdReset},
		{name: ":clear", help: "clear the screen, keeping the session", run: (*repl).cmdClear},
		{name: ":quit", help: "leave the session (:q and Ctrl-D do the same)", run: (*repl).cmdQuit},
	}
}

// aliases are the short spellings.
var aliases = map[string]string{":q": ":quit", ":h": ":help", ":?": ":help", ":exit": ":quit"}

// command runs one meta command.
func (r *repl) command(line string) {
	name, arg, _ := strings.Cut(line, " ")
	name = strings.ToLower(strings.TrimSpace(name))
	arg = strings.TrimSpace(arg)
	if full, ok := aliases[name]; ok {
		name = full
	}
	for _, c := range commands() {
		if c.name == name {
			c.run(r, arg)
			return
		}
	}
	if near := closest(name); near != "" {
		r.p.note("there is no %s command; the closest is %s. :help lists them all", name, near)
		return
	}
	r.p.note("there is no %s command. :help lists them all", name)
}

// closest finds the command a mistyped name most likely meant.
func closest(name string) string {
	best, bestScore := "", 4
	for _, c := range commands() {
		if d := distance(name, c.name); d < bestScore {
			best, bestScore = c.name, d
		}
	}
	return best
}

// distance is Levenshtein, used only to suggest a command name.
func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func (r *repl) cmdHelp(string) {
	r.p.title("commands")
	for _, c := range commands() {
		left := c.name
		if c.arg != "" {
			left += " " + c.arg
		}
		r.p.line(fmt.Sprintf("  %-24s %s", left, c.help))
	}
	r.p.blank()
	r.p.body("Anything that does not start with a colon is Perl. It is converted the\n" +
		"moment it parses, and declarations stay in scope for the rest of the\n" +
		"session, so a program can be built a line at a time. A snippet that spans\n" +
		"lines needs no continuation marker: the prompt keeps reading until the\n" +
		"snippet is complete.\n\n" +
		"Your Perl is never executed. It is read, converted and explained.")
	r.p.blank()
}

func (r *repl) cmdGo(arg string) {
	if r.s.last == nil {
		r.p.note("nothing has been converted yet")
		return
	}
	switch arg {
	case "", "last":
		code := r.s.last.added
		if r.mode == "annotated" {
			code = r.s.last.annotated
		}
		if len(code) == 0 {
			r.p.note("(the last snippet produced no Go)")
			return
		}
		r.p.code(code)
	case "full", "all":
		r.p.code(strings.Split(strings.TrimRight(r.s.full(r.mode == "annotated"), "\n"), "\n"))
		if r.unusedNote() {
			r.p.note("(a `_ = name` line is there because Go rejects a variable it never reads;")
			r.p.note(" using the variable anywhere removes it)")
		}
	default:
		r.p.note(":go takes nothing or `full`")
	}
}

// unusedNote reports whether the session's Go contains a blank assignment,
// which is the first thing Go rejects that Perl allows and the cheapest
// possible place to learn it.
func (r *repl) unusedNote() bool {
	return strings.Contains(r.s.full(false), "\t_ = ")
}

// codePattern is how :explain tells a diagnostic code from a concept id.
var codePattern = regexp.MustCompile(`^[Pp]2[Gg][0-9]{4}$`)

func (r *repl) cmdExplain(arg string) {
	if arg == "" {
		r.explainLast()
		return
	}
	if codePattern.MatchString(arg) {
		r.explainCode(strings.ToUpper(arg))
		return
	}
	if c, ok := r.kb.Get(arg); ok {
		r.p.body(c.Terminal())
		return
	}
	hits := r.kb.Search(arg)
	if len(hits) == 1 {
		r.p.body(hits[0].Terminal())
		return
	}
	if len(hits) > 1 {
		r.p.note("%q matches %d concepts:", arg, len(hits))
		for _, c := range hits {
			r.p.line(fmt.Sprintf("  %-34s %s", c.ID, c.Title))
		}
		return
	}
	r.p.note("no concept or diagnostic code matches %q; :concepts lists what this session touched", arg)
}

// explainLast is the expansion the concept line advertises: every concept the
// last snippet raised, each with its title and its opening paragraph.
//
// A digest rather than the full lessons, because expanding six lessons at once
// buries the code they were about. One `:explain <id>` gets the whole lesson.
func (r *repl) explainLast() {
	if r.s.last == nil || len(r.s.last.concepts) == 0 {
		r.p.note("the last snippet raised no new concepts; :concepts lists the session's")
		return
	}
	concepts, unknown := r.lookup(r.s.last.concepts)
	for i, c := range concepts {
		if i > 0 {
			r.p.blank()
		}
		r.p.title(c.Title)
		r.p.body(c.Opening())
		r.p.note("(:explain %s for the whole lesson)", c.ID)
	}
	for _, id := range unknown {
		r.p.note("no lesson is written for %s yet", id)
	}
}

// lookup fetches exactly the concepts named, in the order they were named.
//
// Deliberately not kb.Resolve, which also pulls in each concept's
// prerequisites: the concept line promised two ids and expanding it into six
// lessons, four of which this snippet never touched, is not what was offered.
func (r *repl) lookup(ids []string) (found []*teach.Concept, unknown []string) {
	for _, id := range ids {
		if c, ok := r.kb.Get(id); ok {
			found = append(found, c)
			continue
		}
		unknown = append(unknown, id)
	}
	return found, unknown
}

func (r *repl) explainCode(code string) {
	entry, ok := diag.Lookup(diag.Code(code))
	if !ok {
		r.p.note("%s is not a diagnostic code perl2go knows", code)
		return
	}
	built := report.Entry{
		Code:         code,
		Severity:     entry.Severity,
		SeverityName: entry.Severity.String(),
		Short:        entry.Short,
		Message:      placeholders(entry.Message),
		Advice:       entry.Advice,
		Concepts:     entry.Concepts,
	}
	var b strings.Builder
	_ = diag.Render(&b, built, "perl2go "+code, nil, diag.Options{Color: r.p.color})
	r.p.body(b.String())
}

// placeholders blanks out the format verbs in a registry message, which is
// what turns a template into something readable on its own.
func placeholders(msg string) string {
	msg = strings.ReplaceAll(msg, "%s", "...")
	msg = strings.ReplaceAll(msg, "%d", "...")
	return strings.ReplaceAll(msg, "%%", "%")
}

func (r *repl) cmdConcepts(string) {
	if len(r.s.named) == 0 {
		r.p.note("no concepts yet; they arrive as the Go does")
		return
	}
	ids := make([]string, 0, len(r.s.named))
	for id := range r.s.named {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	concepts, unknown := r.lookup(ids)
	r.p.title(fmt.Sprintf("%s in this session", plural(len(ids), "concept")))
	for _, c := range concepts {
		r.p.line(fmt.Sprintf("  %-34s %s", c.ID, c.Title))
	}
	for _, id := range unknown {
		r.p.line(fmt.Sprintf("  %-34s (no lesson written yet)", id))
	}
	r.p.note("read one with :explain <id>")
}

func (r *repl) cmdWhy(string) {
	segs := r.s.segments()
	if len(segs) == 0 {
		r.p.note("nothing to explain yet; convert a snippet first")
		return
	}
	for i, seg := range segs {
		if i > 0 {
			r.p.blank()
		}
		r.p.title(seg.Title)
		if seg.Explain != "" {
			r.p.body(teach.WrapAt(seg.Explain, 76))
		}
		if len(seg.Concepts) > 0 {
			r.p.note("concepts: %s", strings.Join(seg.Concepts, ", "))
		}
	}
	r.p.note("(:mode annotated puts this reasoning in the code itself)")
}

func (r *repl) cmdDiag(string) {
	if r.s.last == nil || len(r.s.last.entries) == 0 {
		r.p.note("the last snippet raised no diagnostics")
		return
	}
	lines := strings.Split(r.s.last.perl, "\n")
	for i, e := range r.s.last.entries {
		if i > 0 {
			r.p.blank()
		}
		// The entry's line is in session coordinates; the excerpt the caret
		// sits under is the snippet, so the source is offset to match.
		shown := e
		shown.Line = e.Line - r.s.last.startLine + 1
		var b strings.Builder
		_ = diag.Render(&b, shown, "<repl>", lines, diag.Options{Color: r.p.color})
		r.p.body(b.String())
	}
}

func (r *repl) cmdVars(string) {
	syms := r.s.symbols()
	if len(syms) == 0 {
		r.p.note("no variables yet")
		return
	}
	sorted := slices.Clone(syms)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	width := 0
	for _, s := range sorted {
		width = max(width, len(s.Name))
	}
	for _, s := range sorted {
		typ := s.Type
		if typ == "" {
			typ = "any"
		}
		line := fmt.Sprintf("  %-*s  %s", width, s.Name, typ)
		if s.Reason != "" {
			line += "  " + r.p.style(sgrDim, s.Reason)
		}
		r.p.line(line)
	}
	if dynamic(sorted) > 0 {
		r.p.note("a type of `any` means inference had nothing to work from; give the")
		r.p.note("variable a use with a definite type and it will sharpen")
	}
}

// dynamic counts the symbols type inference could not pin down.
func dynamic(syms []report.Symbol) int {
	n := 0
	for _, s := range syms {
		if s.Type == "any" || s.Type == "" {
			n++
		}
	}
	return n
}

func (r *repl) cmdPerl(string) {
	if len(r.s.snippets) == 0 {
		r.p.note("the session is empty")
		return
	}
	for i, line := range strings.Split(strings.TrimRight(r.s.program(), "\n"), "\n") {
		r.p.line(fmt.Sprintf("  %s %s", r.p.style(sgrDim, fmt.Sprintf("%3d", i+1)), line))
	}
}

func (r *repl) cmdMode(arg string) {
	switch arg {
	case "":
		r.p.note("mode is %s; :mode clean or :mode annotated switches it", r.mode)
	case "clean", "annotated":
		r.mode = arg
		r.p.note("mode is now %s", arg)
	default:
		r.p.note(":mode takes clean or annotated, not %q", arg)
	}
}

func (r *repl) cmdNotes(arg string) {
	switch arg {
	case "on":
		r.notes = true
		r.p.note("concept line on")
	case "off":
		r.notes = false
		r.p.note("concept line off")
	case "":
		state := "off"
		if r.notes {
			state = "on"
		}
		r.p.note("concept line is %s; :notes on or :notes off switches it", state)
	default:
		r.p.note(":notes takes on or off, not %q", arg)
	}
}

func (r *repl) cmdSave(arg string) {
	if arg == "" {
		r.p.note(":save needs a file name")
		return
	}
	if len(r.s.snippets) == 0 {
		r.p.note("the session is empty; nothing to save")
		return
	}
	text := r.s.program()
	if err := os.WriteFile(arg, []byte(text), 0o644); err != nil {
		r.p.note("could not write %s: %v", arg, err)
		return
	}
	r.p.note("wrote %s to %s", plural(strings.Count(text, "\n"), "line"), arg)
	r.p.note("perl2go convert %s   for the full project, docs and report", arg)
}

func (r *repl) cmdLoad(arg string) {
	if arg == "" {
		r.p.note(":load needs a file name")
		return
	}
	data, err := os.ReadFile(arg)
	if err != nil {
		r.p.note("could not read %s: %v", arg, err)
		return
	}
	r.replay(string(data))
}

func (r *repl) cmdReset(string) {
	lines := r.s.lineCount()
	r.s.reset()
	r.p.note("forgot %s; the session is empty", plural(lines, "line"))
}

func (r *repl) cmdClear(string) {
	if !r.opt.Interactive || !r.p.color {
		// Escape codes never go into a pipe. Saying what did not happen is
		// more use than a control character in a transcript.
		r.p.note("(the screen is only cleared on a terminal; :reset forgets the session)")
		return
	}
	r.p.raw("\x1b[H\x1b[2J")
}

func (r *repl) cmdQuit(string) {
	r.done = true
}

// internalEntry is the diagnostic for a converter defect. It names the tool
// rather than the input, because that is whose fault it is.
func internalEntry() report.Entry {
	return report.Entry{
		Code:         "P2G0010",
		Severity:     report.Refuse,
		SeverityName: report.Refuse.String(),
		Short:        "perl2go failed while converting that snippet",
	}
}
