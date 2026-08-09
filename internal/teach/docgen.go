package teach

import (
	"cmp"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"perl2golang/internal/report"
)

// The document generator. It turns one DocInput into the Markdown bundle that
// ships beside the generated program: the readme the developer lands on, the
// honest account of the conversion, the line-by-line walkthrough of their own
// file, the lessons their code triggered, and exercises against the code that
// was just produced for them.
//
// Everything here is deterministic: the same DocInput produces byte-identical
// output, so a generated project can be committed and its documentation diffed
// like any other file.

//go:embed guide/*.md
var guideFS embed.FS

// guideSource is the long-form orientation document, copied into every bundle.
// It is optional: when it is absent, orientationFallback composes a shorter one
// from the knowledge base.
const guideSource = "guide/go-for-perl-developers.md"

// The bundle keys, relative to the generated project root.
const (
	fileReadme     = "README.md"
	fileStartHere  = "docs/start-here.md"
	fileReport     = "docs/conversion-report.md"
	fileWalk       = "docs/walkthrough.md"
	fileNotTrans   = "docs/not-translated.md"
	fileExercises  = "docs/exercises.md"
	fileGuide      = "docs/go-for-perl-developers.md"
	fileConceptIdx = "docs/concepts/index.md"
	conceptDir     = "docs/concepts"
)

// conceptFile returns the bundle key of one lesson.
func conceptFile(id string) string { return path.Join(conceptDir, id+".md") }

// Docs renders the complete teaching bundle for one conversion.
// Keys are paths relative to the generated project root; values are Markdown.
func Docs(in DocInput) (map[string]string, error) {
	b, err := newBundle(in)
	if err != nil {
		return nil, err
	}

	out := map[string]string{
		fileReadme:     b.readme(),
		fileStartHere:  b.startHere(),
		fileReport:     b.conversionReport(),
		fileWalk:       b.walkthrough(),
		fileNotTrans:   b.notTranslated(),
		fileExercises:  b.exercises(),
		fileGuide:      b.orientation(),
		fileConceptIdx: b.conceptIndex(),
	}
	for _, c := range b.concepts {
		out[conceptFile(c.ID)] = b.conceptPage(c)
	}
	return out, nil
}

// bundle is one conversion's documentation in progress: the input, the lessons
// it resolved to, and the cross-references between them.
type bundle struct {
	in  DocInput
	kb  *KB
	rep *report.Report

	// entries is a copy of the report's entries in source order. The caller's
	// report is never modified.
	entries []report.Entry

	// concepts are the triggered lessons plus their prerequisites, ordered so
	// that a lesson never precedes one it depends on.
	concepts []*Concept
	inBundle map[string]*Concept

	// why records, per lesson, the things that pulled it in. fromCode marks
	// the ones a fact about the developer's own code pulled in, as opposed to
	// the ones an exercise or another lesson needs. next records which lessons
	// in the bundle build on it.
	why      map[string][]string
	fromCode map[string]bool
	next     map[string][]string

	// guide is the long-form orientation text, empty when it is not embedded.
	guide string

	// funcs are the functions declared in the generated program, in source
	// order, so exercises can name the developer's own code.
	funcs []string

	// tasks are the exercises this bundle ships, either the caller's or the
	// ones derived from the generated code. taskSeeds is the subset whose
	// lessons are pulled into the bundle.
	tasks     []Exercise
	taskSeeds []Exercise

	script  string // the input's display name, empty when there is no file name
	program string // the generated program's name
}

func newBundle(in DocInput) (*bundle, error) {
	b := &bundle{
		in:       in,
		kb:       Load(),
		rep:      in.Report,
		inBundle: make(map[string]*Concept),
		why:      make(map[string][]string),
		fromCode: make(map[string]bool),
		next:     make(map[string][]string),
	}
	if b.rep == nil {
		b.rep = &report.Report{}
	}

	// Entries are read in the order the developer reads their own file. The
	// caller's report keeps its own order: it is rendered to the terminal and
	// to JSON as well, and this one is not allowed to disturb those.
	b.entries = slices.Clone(b.rep.Entries)
	slices.SortStableFunc(b.entries, func(a, c report.Entry) int {
		if n := cmp.Compare(a.Line, c.Line); n != 0 {
			return n
		}
		if n := cmp.Compare(a.Col, c.Col); n != 0 {
			return n
		}
		return cmp.Compare(a.Code, c.Code)
	})

	b.script = displayName(in, b.rep)
	b.program = programName(in, b.script)
	b.funcs = goFuncNames(in.GoSource)

	// Two passes, because the tasks are chosen from the lessons and then
	// contribute lessons of their own. The second pass only ever adds.
	b.resolveConcepts()
	b.tasks = in.Exercises
	if len(b.tasks) == 0 {
		b.tasks = b.defaultExercises()
	}
	if strings.TrimSpace(in.GoSource) != "" {
		// The tasks name lessons of their own, and those belong in the bundle
		// so the links resolve. With no generated code there is nothing to set
		// a task against, and a bundle of lessons nothing triggered would be
		// the generic tutorial dump this tool exists to avoid.
		b.taskSeeds = b.tasks
	}
	b.resolveConcepts()

	guide, err := guideText()
	if err != nil {
		return nil, err
	}
	b.guide = guide

	return b, nil
}

// resolveConcepts collects every triggered lesson id, expands it with its
// prerequisites, and records why each one is in the bundle.
func (b *bundle) resolveConcepts() {
	var ids []string
	ids = append(ids, b.in.Concepts...)
	ids = append(ids, b.rep.Concepts...)
	for _, e := range b.entries {
		ids = append(ids, e.Concepts...)
	}
	for _, s := range b.in.Walkthrough {
		ids = append(ids, s.Concepts...)
	}
	for _, x := range b.taskSeeds {
		ids = append(ids, x.Concepts...)
	}
	stdlib := stdlibConcepts(b.in.GoSource)
	for _, u := range stdlib {
		ids = append(ids, u.concept)
	}

	// Unknown ids are dropped rather than reported: a lesson that does not
	// exist is nothing the developer can act on.
	concepts, _ := b.kb.Resolve(dedupe(ids))
	b.concepts = concepts
	b.inBundle = make(map[string]*Concept, len(concepts))
	b.why = make(map[string][]string, len(concepts))
	b.fromCode = make(map[string]bool, len(concepts))
	b.next = make(map[string][]string, len(concepts))
	for _, c := range concepts {
		b.inBundle[c.ID] = c
	}

	for _, e := range b.entries {
		for _, id := range e.Concepts {
			if _, ok := b.inBundle[id]; !ok {
				continue
			}
			b.why[id] = append(b.why[id], fmt.Sprintf("%s%s", e.Construct, positionSuffix(e.Line, entryScript(e, b.script))))
			b.fromCode[id] = true
		}
	}
	for _, s := range b.in.Walkthrough {
		for _, id := range s.Concepts {
			if _, ok := b.inBundle[id]; !ok {
				continue
			}
			b.why[id] = append(b.why[id], fmt.Sprintf("the region %q%s", s.Title, lineRangeSuffix(s.PerlFrom, s.PerlTo)))
			b.fromCode[id] = true
		}
	}

	for _, u := range stdlib {
		if _, ok := b.inBundle[u.concept]; ok {
			b.why[u.concept] = append(b.why[u.concept], "the generated code calls `"+u.call+"`")
			b.fromCode[u.concept] = true
		}
	}
	for _, x := range b.taskSeeds {
		for _, id := range x.Concepts {
			if _, ok := b.inBundle[id]; ok {
				b.why[id] = append(b.why[id], fmt.Sprintf("the exercise %q", x.Title))
			}
		}
	}

	// A prerequisite is in the bundle because something else needs it, which is
	// worth saying on its page.
	for _, c := range b.concepts {
		for _, p := range c.Prerequisites {
			if _, ok := b.inBundle[p]; ok {
				b.next[p] = append(b.next[p], c.ID)
			}
		}
	}
}

// stdlibUse is one standard-library call found in the generated program, with
// the lesson that explains the package it came from.
type stdlibUse struct {
	call    string
	concept string
}

// stdlibCalls maps a call the emitter can produce to the lesson a developer
// reading it will want. The conversion's own triggers come from the Perl side
// and stop at the language; these come from the Go side, so a program that
// ends up formatting with fmt or parsing with strconv gets the lesson about
// the package it now depends on, whatever Perl it started as.
var stdlibCalls = []stdlibUse{
	{"fmt.Printf(", "fmt-and-verbs"},
	{"fmt.Sprintf(", "fmt-and-verbs"},
	{"fmt.Fprintf(", "fmt-and-verbs"},
	{"strings.", "strings-package"},
	{"strconv.", "strconv-parsing"},
	{"regexp.", "regexp-is-re2"},
	{"bufio.NewScanner(", "bufio-scanner-limit"},
	{"slices.Sort", "sort-slice"},
	{"sort.Slice(", "sort-slice"},
	{"time.", "time-layouts"},
	{"filepath.", "filepath-and-paths"},
	{"flag.", "flag-package"},
	{"json.", "encoding-json"},
	{"exec.Command", "os-exec"},
}

// stdlibConcepts returns the lessons the generated program's own calls earn,
// each with the call that earned it, in the order the table lists them.
func stdlibConcepts(goSrc string) []stdlibUse {
	if strings.TrimSpace(goSrc) == "" {
		return nil
	}
	var out []stdlibUse
	seen := make(map[string]bool)
	for _, u := range stdlibCalls {
		if seen[u.concept] || !strings.Contains(goSrc, u.call) {
			continue
		}
		seen[u.concept] = true
		out = append(out, stdlibUse{call: strings.TrimSuffix(u.call, "("), concept: u.concept})
	}
	return out
}

// readme is the generated project's own front page.
func (b *bundle) readme() string {
	m := &md{}
	if b.program != "" {
		m.h(1, "%s", b.program)
	} else {
		m.h(1, "The converted program")
	}

	if b.script != "" {
		m.p("This directory holds a Go program converted from `%s`, and the material that explains the conversion. Everything here was written for you to read, not just to run.", b.script)
	} else {
		m.p("This directory holds a converted Go program and the material that explains the conversion. Everything here was written for you to read, not just to run.")
	}

	m.h(2, "Build and run")
	if b.buildFailed() {
		m.p("Before anything else: **this program does not compile as generated.** The converter built it with a real Go toolchain and the build failed, which is a defect in the converter rather than in your original file. The output is still here to be read and fixed, and %s quotes the errors the toolchain reported.",
			link("the conversion report", rel(fileReadme, fileReport)))
	}
	m.p("There are two copies of the program. The clean one is at the root of this directory: ordinary Go, formatted by `gofmt`, with the kind of comments a Go developer would write and nothing about where it came from.")
	m.fence("", "go run .")
	m.p("The annotated one is the same program with the reasoning left in. Every non-obvious construct carries a comment saying what it is, why it was chosen, and which piece of the original it came from. It is a separate program in its own directory, so it compiles and runs on its own:")
	m.fence("", "go run ./annotated")
	m.p("Both programs behave the same way. Read the annotated one, ship the clean one. To build a binary instead of running from source:")
	if b.program != "" {
		m.fence("", fmt.Sprintf("go build -o %s .", b.program))
	} else {
		m.fence("", "go build .")
	}
	m.p("There is nothing to install first. Arguments and standard input work the same as they did before. Everything after the package on a `go run` line goes to the program, flags included, so `go run . --verbose input.txt` passes both of those through. There is no `--` separator: `go run` would hand that to the program as an argument of its own.")
	if b.rep.Stats.Refused > 0 {
		m.p("Run it even though parts of it were refused, because that is what the refusals were shaped to allow. Where a construct could not be converted the code calls `notImplemented`, which hands back the zero value of the type that position wanted and carries on, so everything around the hole still runs. The first time the program reaches one it prints a line beginning `TODO` on standard error and keeps going: that is a report, not a crash. %s says what each one is and what to write in its place.",
			link("What did not translate", rel(fileReadme, fileNotTrans)))
	}

	m.h(2, "What is in here")
	m.bullet("`main.go` and any other `.go` files at the root: the clean program.")
	m.bullet("`annotated/`: the same program, commented as a lesson.")
	if b.in.Module != "" {
		m.bullet("`go.mod`: the module definition. The module path is `%s`; change it when you move this code somewhere permanent.", b.in.Module)
	} else {
		m.bullet("`go.mod`: the module definition. Change the module path when you move this code somewhere permanent.")
	}
	m.bullet("`docs/`: the teaching material for this conversion, described below.")

	m.h(2, "What to read first")
	m.p("%s. It says what was produced, how completely it converted, and what to read in what order.", link("Start with docs/start-here.md", rel(fileReadme, fileStartHere)))
	m.p("After that, the rest of `docs/`:")
	m.bullet("%s: your original file and the Go it became, region by region, with the reasoning.", link("walkthrough.md", rel(fileReadme, fileWalk)))
	m.bullet("%s: what converted, what was approximated, what was refused, and the counts behind those words.", link("conversion-report.md", rel(fileReadme, fileReport)))
	m.bullet("%s: the parts you have to finish by hand, and how.", link("not-translated.md", rel(fileReadme, fileNotTrans)))
	m.bullet("%s: the general orientation, independent of this program.", link("go-for-perl-developers.md", rel(fileReadme, fileGuide)))
	m.bullet("%s: the lessons this particular script triggered.", link("concepts/index.md", rel(fileReadme, fileConceptIdx)))
	m.bullet("%s: small checkable tasks against the code in this directory.", link("exercises.md", rel(fileReadme, fileExercises)))

	m.raw(b.credit())
	return m.String()
}

// startHere orients the developer who has just run the tool.
func (b *bundle) startHere() string {
	m := &md{}
	m.h(1, "Start here")

	m.p("You asked for a conversion and got a directory. This page says what is in it, how far the conversion actually got, and the order to read things in. Ten minutes here saves an hour of guessing.")

	m.h(2, "What was produced")
	if b.buildFailed() {
		m.bullet("The clean program, at the root of this directory. It does not build as generated: see below and %s.", link("the conversion report", rel(fileStartHere, fileReport)))
		m.bullet("The annotated program, in `annotated/`. The same code with the reasoning left in, and the same build errors, since the two are one program rendered twice.")
	} else {
		m.bullet("The clean program, at the root of this directory. Run it with `go run .` from the parent directory of this `docs/` folder.")
		m.bullet("The annotated program, in `annotated/`. Same behaviour, heavy commentary. Run it with `go run ./annotated`.")
	}
	m.bullet("This documentation. It is specific to your file: the lessons in `concepts/` were chosen by what your code actually does, not from a fixed curriculum.")
	m.bullet("%s, which repeats the build instructions.", link("The project readme", rel(fileStartHere, fileReadme)))

	m.h(2, "How completely it converted")
	m.p("%s", b.honesty())

	m.h(2, "What to read, in what order")
	m.numbered(1, "%s. Your file and its translation side by side. This is the one to read while looking at the generated code in another window.", link("The walkthrough", rel(fileStartHere, fileWalk)))
	m.numbered(2, "%s. The counts, and every construct the converter had something to say about.", link("The conversion report", rel(fileStartHere, fileReport)))
	m.numbered(3, "%s. The list of things you have to write yourself, with the reasoning for each.", link("What did not translate", rel(fileStartHere, fileNotTrans)))
	m.numbered(4, "%s. The lessons your code triggered, ordered so that nothing depends on something you have not read yet.", link("The concept lessons", rel(fileStartHere, fileConceptIdx)))
	m.numbered(5, "%s. The general tour, worth reading once whether or not it relates to this program.", link("Go for Perl developers", rel(fileStartHere, fileGuide)))
	m.numbered(6, "%s. Small tasks against this code. Reading Go is not learning Go; changing it is.", link("The exercises", rel(fileStartHere, fileExercises)))

	m.h(2, "A word on how to use the annotated program")
	m.p("The annotated copy is not a text file with code in it. It is the same program, so you can edit it, break it, and see what the compiler says. That feedback loop is the fastest way into a new language, and the comments in it are placed where the surprises are rather than where prose is easy to write. Each explanation appears once, at the first place it applies, so the file stays readable top to bottom.")

	m.raw(b.credit())
	return m.String()
}

// honesty is the one-paragraph statement of how well this conversion went. It
// is written to be read by someone deciding whether to trust the output.
func (b *bundle) honesty() string {
	s := b.rep.Stats
	var parts []string

	subject := "The input"
	if b.script != "" {
		subject = "`" + b.script + "`"
	}
	if n := countLines(b.in.PerlSource); n > 0 {
		parts = append(parts, fmt.Sprintf("%s is %d line%s long.", subject, n, plural(n)))
	}

	switch {
	case s.Statements > 0 && s.Converted >= s.Statements:
		parts = append(parts, fmt.Sprintf("The converter recognised all %d statement%s in it and produced Go for every one.", s.Statements, plural(s.Statements)))
	case s.Statements > 0:
		parts = append(parts, fmt.Sprintf("Of the %d statement%s in it, %d converted directly.", s.Statements, plural(s.Statements), s.Converted))
	}

	switch {
	case s.Approximated > 0 && s.Refused > 0:
		parts = append(parts, fmt.Sprintf("%s approximated, meaning the Go runs but does not do exactly what the original did, and %s refused outright, meaning no Go was produced for %s at all.",
			constructsWere(s.Approximated), constructsWere(s.Refused), itOrThem(s.Refused)))
	case s.Approximated > 0:
		parts = append(parts, fmt.Sprintf("%s approximated: the Go runs, but it does not do exactly what the original did.", constructsWere(s.Approximated)))
	case s.Refused > 0:
		parts = append(parts, fmt.Sprintf("%s refused outright: no Go was produced for %s, and the program is incomplete until you write %s yourself.",
			constructsWere(s.Refused), itOrThem(s.Refused), itOrThem(s.Refused)))
	default:
		parts = append(parts, "Nothing was approximated and nothing was refused.")
	}

	if s.Todos > 0 {
		parts = append(parts, fmt.Sprintf("There %s %d TODO marker%s in the generated code, each naming the specific problem at the place it occurs.", verbIs(s.Todos), s.Todos, plural(s.Todos)))
	}
	if s.Refused > 0 {
		parts = append(parts, "No refusal stops the program: each one stands in for the value its position wanted, so this builds and runs, does the parts it could, and says on standard error which gap it walked past.")
	}

	if s.Symbols > 0 {
		dynamic := s.Symbols - s.SymbolsTyped
		if dynamic > 0 {
			parts = append(parts, fmt.Sprintf("Type inference gave a concrete Go type to %d of the %d variables it tracked; the other %d fall back to a dynamic value, which works but is not the Go you would write by hand.", s.SymbolsTyped, s.Symbols, dynamic))
		} else if s.Symbols == 1 {
			parts = append(parts, "The one variable it tracked was given a concrete Go type, so there is no dynamic fallback anywhere in the output.")
		} else {
			parts = append(parts, fmt.Sprintf("Every one of the %d variables it tracked was given a concrete Go type, so there is no dynamic fallback anywhere in the output.", s.Symbols))
		}
	}
	if s.ParseErrors > 0 {
		region := "that region is"
		if s.ParseErrors > 1 {
			region = "those regions are"
		}
		parts = append(parts, fmt.Sprintf("%d part%s of the file could not be parsed at all, and %s marked in the generated code and listed in the report.", s.ParseErrors, plural(s.ParseErrors), region))
	}

	switch {
	case b.buildFailed():
		// Nothing else on this page matters as much as this, so it goes last,
		// where the paragraph ends.
		parts = append(parts, fmt.Sprintf("**The generated program does not compile.** That is a defect in the converter, not in your file: the Go was written out anyway so you can see it, and %s quotes what the toolchain said. Expect to fix those lines before `go run .` works.",
			link("the conversion report", rel(fileStartHere, fileReport))))
	case s.Refused > 0 || s.ParseErrors > 0:
		parts = append(parts, "Treat this as a starting point that still needs work, not a finished port.")
	case s.Approximated > 0:
		parts = append(parts, "Treat it as a good starting point that needs review at the places listed in the report, not as a finished port.")
	case s.Statements == 0:
		parts = append(parts, "The converter recorded no per-statement counts for this run, so judge the result by reading the walkthrough and the generated code.")
	default:
		parts = append(parts, "Nothing in this file defeated the converter, which is a statement about the converter's confidence, not a proof of correctness: the tests are still yours to write.")
	}

	return strings.Join(parts, " ")
}

// conversionReport renders the report as Markdown.
func (b *bundle) conversionReport() string {
	m := &md{}
	m.h(1, "Conversion report")

	if b.script != "" {
		m.p("This is the account of what the converter did to `%s`, including the parts it did badly. Everything the tool was unsure about is here, so that nothing about the output has to be taken on trust.", b.script)
	} else {
		m.p("This is the account of what the converter did, including the parts it did badly. Everything the tool was unsure about is here, so that nothing about the output has to be taken on trust.")
	}

	m.h(2, "Summary")
	s := b.rep.Stats
	m.table([]string{"Measure", "Count"}, [][]string{
		{"Statements found", fmt.Sprint(s.Statements)},
		{"Converted directly", fmt.Sprint(s.Converted)},
		{"Approximated", fmt.Sprint(s.Approximated)},
		{"Refused", fmt.Sprint(s.Refused)},
		{"TODO markers in the generated code", fmt.Sprint(s.Todos)},
		{"Variables tracked", fmt.Sprint(s.Symbols)},
		{"Variables given a concrete type", fmt.Sprint(s.SymbolsTyped)},
		{"Variables left dynamic", fmt.Sprint(s.Symbols - s.SymbolsTyped)},
		{"Parse errors", fmt.Sprint(s.ParseErrors)},
	})
	if s.Symbols > 0 {
		m.p("Dynamic fallback rate: %.0f%%. That is the share of variables the tool could not give a static Go type, and it is the number that best predicts how much of this code you will want to rewrite by hand.", b.rep.DynamicRate()*100)
	}
	if s.Statements == 0 && len(b.entries) == 0 {
		m.p("The counters are all zero because the converter recorded no statement-level statistics for this run. That is a gap in the record, not a claim that the file was empty.")
	}

	m.h(2, "Checks run on the generated code")
	for _, line := range b.verificationLines() {
		m.bullet("%s", line)
	}
	if v := b.rep.Verified; v.Error != "" {
		// The toolchain's own words, laid out as it printed them. Folded onto
		// one line, a build failure is unreadable, and this is the line the
		// developer most needs to read.
		m.p("What the toolchain said:")
		m.fence("", strings.TrimRight(v.Error, "\n"))
	}

	if len(b.entries) == 0 {
		m.h(2, "Entries")
		m.p("The converter had nothing to flag: no note, no approximation, no refusal. Read %s anyway; a clean report means the tool understood the constructs it saw, not that the resulting Go is the Go you would have written.", link("the walkthrough", rel(fileReport, fileWalk)))
	} else {
		// Refusals and approximations are work, and the work list is
		// not-translated.md. Repeating their full entries here made the two
		// documents near-identical copies; the report keeps the one-line
		// account and the reader follows one link for the reasoning.
		refused := b.entrySummarySection(m, report.Refuse, "Refused", "Nothing was generated for these. The program does not do what the original did until you write them yourself, though it does still run: each one stands in for the value its position wanted and names itself on standard error when it is reached.")
		approximated := b.entrySummarySection(m, report.Warn, "Approximated", "Go was generated for these, but it differs from the original in a way you need to know about.")
		if refused || approximated {
			m.p("Each of these is in %s with the full reasoning and what to do about it by hand.",
				link("What did not translate", rel(fileReport, fileNotTrans)))
		}
		b.entrySection(m, fileReport, report.Note, "Notes", "These converted cleanly. They are here because the difference between the two languages is worth pointing out at this spot.")
	}

	b.symbolSection(m)

	m.rule()
	if b.rep.Stats.Approximated+b.rep.Stats.Refused > 0 {
		m.p("%s turns the approximations and refusals above into a work list, with what to do about each. %s explains the language differences behind them.",
			link("What did not translate", rel(fileReport, fileNotTrans)),
			link("The lesson index", rel(fileReport, fileConceptIdx)))
	} else {
		m.p("%s says the same thing from the other direction, and %s covers the language differences worth knowing about regardless.",
			link("What did not translate", rel(fileReport, fileNotTrans)),
			link("the lesson index", rel(fileReport, fileConceptIdx)))
	}
	m.raw(b.credit())
	return m.String()
}

// buildFailed reports whether a real toolchain rejected the generated code.
// It is the single most important fact about a conversion, so the pages the
// developer lands on say it rather than leaving it in the report.
func (b *bundle) buildFailed() bool {
	v := b.rep.Verified
	return v.Toolchain && !v.Built
}

// verificationLines describes how thoroughly the tool checked its own output.
func (b *bundle) verificationLines() []string {
	v := b.rep.Verified
	var out []string

	if v == (report.Verification{}) {
		return []string{"No verification was recorded for this conversion, so treat the output as unchecked. Run `go build ./...` and `go vet ./...` before trusting it."}
	}

	if v.Parsed {
		out = append(out, "Every generated file was parsed with Go's own parser, so the output is syntactically valid Go.")
	} else {
		out = append(out, "The generated code did not pass a parse check. That is a defect in the converter rather than in your original file; the output should be treated as a draft.")
	}

	switch {
	case v.Built:
		out = append(out, "It was compiled with a real Go toolchain, and the build succeeded.")
	case v.Toolchain:
		out = append(out, "A Go toolchain was available and the build was attempted, but it failed.")
	default:
		out = append(out, "No Go toolchain was found, so the code was parsed but not compiled. Run `go build ./...` and `go vet ./...` yourself before trusting it.")
	}

	return out
}

// entrySummarySection renders one severity group as a compact list: the code,
// the construct, where, and the first sentence of what happened. The full
// reasoning and the what-to-do live in not-translated.md, which the section
// ends by pointing at; writing both documents in full made them copies of
// each other, read twice by anyone following the suggested order. It reports
// whether it wrote anything, so the caller adds the pointer once.
func (b *bundle) entrySummarySection(m *md, sev report.Severity, title, intro string) bool {
	var group []report.Entry
	for _, e := range b.entries {
		if e.Severity == sev {
			group = append(group, e)
		}
	}
	if len(group) == 0 {
		return false
	}

	grouped := groupEntries(group)
	m.h(2, "%s (%d)", title, len(grouped))
	m.p("%s", intro)
	for _, g := range grouped {
		construct := g.Construct
		if construct == "" {
			construct = "unnamed construct"
		}
		head := construct + linesSuffix(g.lines, "")
		if g.Code != "" {
			head = "**" + g.Code + "**: " + head
		}
		if summary := sentence(firstParagraph(g.Message)); summary != "" {
			head += ". " + capitalise(summary)
		}
		m.bullet("%s", head)
	}
	return true
}

// entrySection renders one severity group of report entries.
func (b *bundle) entrySection(m *md, from string, sev report.Severity, title, intro string) {
	var group []report.Entry
	for _, e := range b.entries {
		if e.Severity == sev {
			group = append(group, e)
		}
	}
	if len(group) == 0 {
		return
	}

	grouped := groupEntries(group)
	m.h(2, "%s (%d)", title, len(grouped))
	m.p("%s", intro)
	for _, g := range grouped {
		b.entryBody(m, from, g, 3)
	}
}

// groupEntries folds entries that say the same thing into one, keeping every
// place they came from.
//
// The same unconvertible construct usually appears many times in a file, at
// several different expressions. Printing the whole explanation for each of
// them buries the entries that differ: a file with thirty method calls in it
// produced thirty copies of one paragraph about how Go declares methods. The
// explanation is now given once and the places are listed under it.
func groupEntries(in []report.Entry) []groupedEntry {
	var out []groupedEntry
	index := map[string]int{}
	for _, e := range in {
		key := e.Code + "\x00" + e.Construct + "\x00" + e.Message + "\x00" + e.Advice
		i, ok := index[key]
		if !ok {
			i = len(out)
			index[key] = i
			out = append(out, groupedEntry{Entry: e})
		}
		out[i].places++
		out[i].addLine(e.Line)
		out[i].addSite(e.Perl, e.Line)
	}
	return out
}

// groupedEntry is one report entry with every place it came from.
type groupedEntry struct {
	report.Entry
	// lines is every distinct line the entry was raised on, in order.
	lines []int
	// places is how many times it was raised, which is what the terminal
	// summary counts.
	places int
	// sites are the distinct pieces of source that raised it, in the order
	// they first appeared.
	sites []site
}

// site is one piece of the original that raised an entry, and the lines it
// appeared on.
type site struct {
	perl  string
	lines []int
}

// addLine records a line once, keeping the list sorted. One statement can
// raise the same entry several times, and a heading that says "at lines 69,
// 69, 69" is noise.
func (g *groupedEntry) addLine(line int) {
	at, found := slices.BinarySearch(g.lines, line)
	if found {
		return
	}
	g.lines = slices.Insert(g.lines, at, line)
}

// addSite records one occurrence under the piece of source that caused it.
func (g *groupedEntry) addSite(perl string, line int) {
	for i := range g.sites {
		if g.sites[i].perl == perl {
			if at, found := slices.BinarySearch(g.sites[i].lines, line); !found {
				g.sites[i].lines = slices.Insert(g.sites[i].lines, at, line)
			}
			return
		}
	}
	g.sites = append(g.sites, site{perl: perl, lines: []int{line}})
}

// maxSites caps how many distinct expressions one entry lists. Past a handful
// the list stops being a work list and becomes a concordance; the line numbers
// in the heading still cover every one of them.
const maxSites = 8

// entryBody renders one entry: what it was, where it was, what happened, and
// what to do.
func (b *bundle) entryBody(m *md, from string, g groupedEntry, level int) {
	e := g.Entry
	construct := e.Construct
	if construct == "" {
		construct = "unnamed construct"
	}
	heading := construct + linesSuffix(g.lines, b.script)
	if e.Code != "" {
		heading = e.Code + ": " + heading
	}
	m.h(level, "%s", heading)

	b.entrySites(m, g)
	if msg := strings.TrimSpace(e.Message); msg != "" {
		// A message that spans lines is quoted output, usually the compiler's.
		// Run together as a paragraph it is unreadable, so only the first line
		// is prose and the rest keeps its shape.
		head, rest, multiline := strings.Cut(msg, "\n")
		m.p("%s", head)
		if multiline && strings.TrimSpace(rest) != "" {
			m.fence("", strings.TrimRight(rest, "\n"))
		}
	}
	if strings.TrimSpace(e.Advice) != "" {
		m.p("What to do: %s", strings.TrimSpace(e.Advice))
	}
	if links := b.conceptLinks(e.Concepts, from); links != "" {
		m.p("Lessons: %s", links)
	}
}

// entrySites shows the source that raised an entry: the expression itself when
// there is only one, and a list of them when the same thing was said about
// several.
func (b *bundle) entrySites(m *md, g groupedEntry) {
	var sites []site
	for _, s := range g.sites {
		if strings.TrimSpace(s.perl) != "" {
			sites = append(sites, s)
		}
	}
	switch {
	case len(sites) == 0:
		return
	case len(sites) == 1:
		m.p("The original:")
		m.fence("perl", clipSource(dedent(sites[0].perl)))
		return
	}

	m.p("Where it happens:")
	shown := sites
	if len(shown) > maxSites {
		shown = shown[:maxSites]
	}
	for _, s := range shown {
		text := dedent(strings.TrimSpace(s.perl))
		where := strings.TrimPrefix(linesSuffix(s.lines, ""), " ")
		if strings.Contains(text, "\n") {
			m.p("%s:", capitalise(where))
			m.fence("perl", clipSource(text))
			continue
		}
		m.bullet("`%s`%s", text, linesSuffix(s.lines, ""))
	}
	if len(sites) > len(shown) {
		m.bullet("and %d more like these", len(sites)-len(shown))
	}
}

// clipSource caps a quoted region of the original. A diagnostic about a
// construct sometimes carries the whole block the construct heads, and a
// thirty-line quote buries the entries around it; the head names the
// construct and the reader has the file.
func clipSource(text string) string {
	const limit, keep = 16, 12
	lines := strings.Split(text, "\n")
	if len(lines) <= limit {
		return text
	}
	return strings.Join(lines[:keep], "\n") + fmt.Sprintf("\n# ... %d more lines", len(lines)-keep)
}

// symbolSection lists the variables type inference could not resolve, which is
// the most useful measure of how well the tool understood the program.
func (b *bundle) symbolSection(m *md) {
	var dynamic []report.Symbol
	for _, s := range b.rep.Symbols {
		if !s.Inferred {
			dynamic = append(dynamic, s)
		}
	}
	if len(dynamic) == 0 {
		return
	}

	m.h(2, "Variables left dynamic (%d)", len(dynamic))
	m.p("These carry a dynamic value in the generated code instead of a Go type. The program works, but the compiler cannot help you with them and the code reads like a translation. They are the best places to start rewriting.")
	rows := make([][]string, 0, len(dynamic))
	for _, s := range dynamic {
		where := ""
		if s.Line > 0 {
			where = fmt.Sprint(s.Line)
		}
		reason := s.Reason
		if reason == "" {
			reason = "no reason recorded"
		}
		rows = append(rows, []string{"`" + s.Name + "`", where, reason})
	}
	m.table([]string{"Variable", "Line", "Why the type is not known"}, rows)
}

// walkthrough is the document that makes the tool feel like a review rather
// than a compiler: the developer's own file, region by region.
func (b *bundle) walkthrough() string {
	m := &md{}
	if b.script != "" {
		m.h(1, "%s, line by line", b.script)
	} else {
		m.h(1, "The conversion, line by line")
	}

	if len(b.in.Walkthrough) == 0 {
		m.h(2, "No regions were recorded")
		m.p("This page is normally a region-by-region tour: a piece of the original, the Go it became, and the reasoning. The converter did not record one for this file. That happens when the input is small enough that the generated code reads as its own explanation, and it also happens when the converter got far enough to emit code but not far enough to explain it.")
		m.p("Two things fill the gap. The annotated program in `annotated/` carries the same kind of explanation inline, at the code it belongs to, and %s lists every construct the converter had an opinion about. %s cover the language differences you are most likely to hit.",
			link("the conversion report", rel(fileWalk, fileReport)),
			link("The lessons", rel(fileWalk, fileConceptIdx)))
		m.raw(b.credit())
		return m.String()
	}

	m.p("Each section below takes one region of the original, shows the Go it became, and explains the choice. The line numbers refer to the original file, so you can keep it open beside this page. Read the sections in order: later ones assume the earlier ones, and an explanation is given once, at the first region it applies to. Where a region raises several remarks, each one starts with the line of the original it is about, so a specific line can be looked up without reading the rest.")
	m.p("Where a region raises something structural about Go rather than something local to your code, it links to a lesson in %s. Those lessons stand alone, so follow them when you want to and skip them when you do not.", link("concepts/", rel(fileWalk, fileConceptIdx)))

	said := map[string]bool{}
	linked := map[string]bool{}
	for i, seg := range b.in.Walkthrough {
		if i > 0 {
			m.rule()
		}
		title := strings.TrimSpace(seg.Title)
		if title == "" {
			title = "Region " + fmt.Sprint(i+1)
		}
		m.h(2, "%d. %s", i+1, title)

		if strings.TrimSpace(seg.Perl) != "" {
			m.p("%s", capitalise(sourceIntro(b.script, seg.PerlFrom, seg.PerlTo)))
			m.fence("perl", dedent(seg.Perl))
		}
		if strings.TrimSpace(seg.Go) != "" {
			m.p("The Go it became:")
			m.fence("go", dedent(seg.Go))
		}
		explained := renderRegionNotes(m, seg, said)
		if !explained && (len(seg.Notes) > 0 || strings.TrimSpace(seg.Explain) != "") {
			// Every note here was made earlier. Saying so is better than an
			// empty section, and better than repeating the paragraphs.
			m.p("No new ground in this region: every construct in it is one the sections above already explain.")
		}
		// The converter's own concepts come from the Perl side. The packages
		// the region ended up calling are a second source, and the one that
		// covers a region whose Perl was unremarkable and whose Go is not.
		ids := slices.Clone(seg.Concepts)
		for _, u := range stdlibConcepts(seg.Go) {
			ids = append(ids, u.concept)
		}
		if links := b.conceptLinks(firstMentions(ids, linked, maxRegionLessons), fileWalk); links != "" {
			m.p("Lessons this region introduces: %s", links)
		}
	}

	m.rule()
	m.h(2, "Next")
	m.p("Reading a translation is the easy half. The other half is changing it and finding out what the compiler thinks, which is what %s are for: each one is small, names code that is actually in this directory, and tells you how to check that you got it right.", link("the exercises", rel(fileWalk, fileExercises)))
	m.raw(b.credit())
	return m.String()
}

// maxRegionLessons caps the lesson links under one region. A region can touch
// a dozen concepts, and a dozen links is a list nobody follows; the ones left
// out are still in the index.
const maxRegionLessons = 4

// renderRegionNotes writes a region's explanation and reports whether it said
// anything. Remarks that appeared in an earlier region are dropped: the same
// construct recurs all through a script, and repeating its explanation at
// every occurrence trains the reader to skip the prose.
//
// When the fresh remarks span more than one line of the original, each is
// anchored to its line. A region's code block can be long, and a wall of
// unanchored paragraphs leaves the reader matching prose to code by guesswork;
// the anchor is what lets them look up one line and skip the rest.
func renderRegionNotes(m *md, seg Segment, said map[string]bool) bool {
	notes := seg.Notes
	if len(notes) == 0 {
		for _, para := range strings.Split(seg.Explain, "\n\n") {
			if para = strings.TrimSpace(para); para != "" {
				notes = append(notes, SegmentNote{Text: para})
			}
		}
	}

	var fresh []SegmentNote
	lines := map[int]bool{}
	for _, n := range notes {
		if n.Text == "" || said[n.Text] {
			continue
		}
		said[n.Text] = true
		fresh = append(fresh, n)
		if n.Line > 0 {
			lines[n.Line] = true
		}
	}
	if len(fresh) == 0 {
		return false
	}

	if len(lines) < 2 {
		for _, n := range fresh {
			m.p("%s", n.Text)
		}
		return true
	}

	introduced := map[int]bool{}
	for _, n := range fresh {
		switch {
		case n.Line <= 0:
			m.bullet("%s", n.Text)
		case introduced[n.Line]:
			m.bullet("**Line %d**: %s", n.Line, n.Text)
		default:
			introduced[n.Line] = true
			if frag := anchorFragment(n.Perl); frag != "" {
				m.bullet("**Line %d**, `%s`: %s", n.Line, frag, n.Text)
			} else {
				m.bullet("**Line %d**: %s", n.Line, n.Text)
			}
		}
	}
	return true
}

// anchorFragment shortens a remark's source line to something a reader can
// recognise beside the line number. An empty result means the anchor carries
// the number alone.
func anchorFragment(perl string) string {
	frag := strings.Join(strings.Fields(perl), " ")
	if strings.Contains(frag, "`") {
		// A backtick would end the code span and swallow the remark.
		return ""
	}
	const limit = 60
	if len(frag) <= limit {
		return frag
	}
	cut := strings.LastIndexByte(frag[:limit], ' ')
	if cut < limit/2 {
		cut = limit
	}
	return strings.TrimRight(frag[:cut], " ,({") + " ..."
}

// firstMentions returns up to max ids that have not been shown yet, recording
// only the ones it returns. An id displaced by the cap stays unseen, so it can
// still be introduced by a later region that also raises it.
func firstMentions(ids []string, seen map[string]bool, max int) []string {
	var out []string
	for _, id := range ids {
		if seen[id] || len(out) == max {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// workItems phrases one severity's count as jobs and, when they differ, the
// number of places those jobs cover.
func workItems(kind string, groups []groupedEntry) string {
	places := countPlaces(groups)
	head := fmt.Sprintf("%d %s%s", len(groups), kind, plural(len(groups)))
	if places == len(groups) {
		return head
	}
	return fmt.Sprintf("%s covering %d places", head, places)
}

// countPlaces totals how many times a set of grouped entries was raised.
func countPlaces(groups []groupedEntry) int {
	places := 0
	for _, g := range groups {
		places += g.places
	}
	return places
}

// notTranslated is the work list: everything the developer has to finish.
func (b *bundle) notTranslated() string {
	m := &md{}
	m.h(1, "What did not translate")

	var refusedEntries, approximatedEntries []report.Entry
	for _, e := range b.entries {
		switch e.Severity {
		case report.Refuse:
			refusedEntries = append(refusedEntries, e)
		case report.Warn:
			approximatedEntries = append(approximatedEntries, e)
		}
	}
	// Counted after grouping, so that one construct reported at three lines
	// is one item of work rather than three.
	refused := groupEntries(refusedEntries)
	approximated := groupEntries(approximatedEntries)

	if len(refused) == 0 && len(approximated) == 0 {
		if b.script != "" {
			m.p("Nothing. Every construct in `%s` was converted without approximation and without refusal.", b.script)
		} else {
			m.p("Nothing. Every construct in the input was converted without approximation and without refusal.")
		}
		m.p("That is a claim about what the converter recognised, not a proof that the two programs behave identically. %s has the counts behind it, and running both programs against the same input remains the only real check.", link("The conversion report", rel(fileNotTrans, fileReport)))
		m.raw(b.credit())
		return m.String()
	}

	m.p("There are two kinds of entry here. A refusal means no Go was produced for a construct, so the program is missing that behaviour until you add it. An approximation means Go was produced, it runs, and it differs from the original in a way that will eventually bite you if you do not know about it. Both are listed with the reasoning and with what to do by hand.")
	grouping := ""
	if countPlaces(refused)+countPlaces(approximated) > len(refused)+len(approximated) {
		grouping = " It groups by what has to be done rather than by where, so the " +
			"terminal summary counts the places and this page counts the jobs."
	}
	m.p("This file is a work list: %s and %s.%s",
		workItems("refusal", refused), workItems("approximation", approximated), grouping)

	if len(refused) > 0 {
		m.h(2, "Refused: you have to write these (%d)", len(refused))
		m.p("The generated code marks each of these at the place it belongs, so the compiler and your editor will keep reminding you.")
		standInSection(m, 3)
		for _, g := range refused {
			b.entryBody(m, fileNotTrans, g, 3)
		}
	}
	if len(approximated) > 0 {
		m.h(2, "Approximated: check these (%d)", len(approximated))
		m.p("These compile and run. Read each one and decide whether the difference matters for your data; sometimes it does not, and then the honest thing is to delete the TODO and move on.")
		for _, g := range approximated {
			b.entryBody(m, fileNotTrans, g, 3)
		}
	}

	m.rule()
	m.p("When you have worked through this list, the fastest way to prove it is to write a test for each item you fixed. %s cover the mechanics.", link("The exercises", rel(fileNotTrans, fileExercises)))
	m.raw(b.credit())
	return m.String()
}

// standInSection explains what a refusal leaves behind in the code, what the
// program does when it reaches one, and how to close it.
//
// It earns its place because the first thing anyone does with a partly
// converted program is run it, and the first thing they see is a line they did
// not write appearing on standard error. Left unexplained, that line reads like
// a crash. It is the opposite: it is the program telling you it walked past a
// hole and kept going.
func standInSection(m *md, level int) {
	m.h(level, "What a refusal looks like in the code")
	m.p("A refusal is not a gap in the file. Each one leaves a call behind, carrying the same diagnostic code this page uses:")
	m.fence("go", `// TODO: object method calls are not implemented
sku := notImplemented[any]("P2G7001", "object method calls are not implemented")`)
	m.p("A refused step that produced no value, rather than standing in for one, leaves the statement form instead, `notImplementedHere(code, what)`. The explanatory comment is written above the first place each distinct refusal appears; the call is at every one of them, so searching the code for `notImplemented` finds them all and searching for a code finds one.")
	m.p("Three things follow from that shape.")
	m.bullet("**The program builds and runs.** It runs past every one of these until something else stops it, which is the point: the parts that did convert are ordinary Go, and you can watch them work. A refusal that ended the program would make every converted line below it unreachable, and a partial conversion whose converted half cannot run is worth very little.")
	m.bullet("**Each gap says so, once, on standard error.** The first time the program reaches one it prints `TODO` and the code, then carries on. Standard output is left alone, so a diff of the program's real output against the original's is still a diff of the two programs.")
	m.bullet("**The value it hands back is not the answer.** It is the zero value of whatever type that position wanted: `0`, `\"\"`, `nil`, an empty slice. Everything downstream of a gap that ran is unproven, however plausible it looks. That is the trade for being able to run the thing at all, and it is why the marks are loud.")
	m.p("To close one: find the call, read the entry below that carries its code, write the Go the entry describes, and delete the call and the comment above it. When the last of them is gone the program has no `notImplemented` left in it and nothing to print on standard error, which is a check you can run rather than a claim you have to trust.")
}

// exercises renders the tasks against this specific generated code.
func (b *bundle) exercises() string {
	m := &md{}
	m.h(1, "Exercises")

	list := b.tasks
	generated := len(b.in.Exercises) == 0

	m.p("These are tasks against the code in this directory, not toy examples. Each one is small enough for one sitting and ends with a check, so you can tell whether you got it right without asking anyone.")
	if generated {
		m.p("Work them in order. They are arranged so that each one leaves the code in a better state than it found it, and the later ones assume the earlier ones are done.")
	}
	m.p("Run everything from the directory above this one, where `go.mod` lives.")

	for i, x := range list {
		title := strings.TrimSpace(x.Title)
		if title == "" {
			title = "Exercise " + fmt.Sprint(i+1)
		}
		m.h(2, "%d. %s", i+1, title)
		if task := strings.TrimSpace(x.Task); task != "" {
			m.p("%s", task)
		}
		if success := strings.TrimSpace(x.Success); success != "" {
			m.p("Done when: %s", success)
		}
		if links := b.conceptLinks(x.Concepts, fileExercises); links != "" {
			m.p("Lessons: %s", links)
		}
	}

	m.rule()
	m.p("When these stop being interesting, the next exercise is the real one: pick the part of the generated code you like least, delete it, and write it the way you would write it now. That is the point at which the translation stops being someone else's code.")
	m.raw(b.credit())
	return m.String()
}

// defaultExercises builds a set of tasks from the report and the generated
// code when the converter supplied none. Every task names something that is
// actually in this project.
func (b *bundle) defaultExercises() []Exercise {
	var out []Exercise

	// A program the toolchain rejected has one task ahead of every other.
	if b.buildFailed() {
		out = append(out, Exercise{
			Title:    "Make it build",
			Task:     "The converter's own build of this code failed, and the errors are quoted in " + link("the conversion report", rel(fileExercises, fileReport)) + ". Run `go build ./...` yourself, read the first error rather than the whole list, and fix it. Go reports errors in source order and later ones are often consequences of the first, so rebuild after each fix. The usual causes here are a value the converter typed as `any` being used as a number or a slice, which is fixed by giving the variable a concrete type where it is declared.",
			Success:  "`go build ./...` and `go vet ./...` are both silent, and `go run .` produces the output the original produced for the same input.",
			Concepts: []string{"static-types-and-zero-values", "type-assertions-and-switches", "compile-time-mindset"},
		})
	}

	target, hasFunc := b.exerciseTarget()
	if hasFunc {
		out = append(out, Exercise{
			Title: "Write a table-driven test for `" + target + "`",
			Task: fmt.Sprintf("Create `main_test.go` next to `main.go`, in `package main`, and write `Test%s`. Use the table-driven form that Go code uses everywhere: a slice of anonymous structs holding a case name, the inputs, and the expected output, then one `t.Run(tc.name, ...)` per row. Cover an ordinary input and at least one edge case, such as empty input or a value the original handled by accident.",
				exportedish(target)),
			Success: fmt.Sprintf("`go test ./...` passes, and `go test -run Test%s -v` lists one subtest per row of your table. Then change `%s` to be wrong on purpose and confirm the failure message names the case that broke.",
				exportedish(target), target),
			Concepts: []string{"table-driven-tests", "multiple-return-values"},
		})
	} else {
		out = append(out, Exercise{
			Title:    "Give the program something to test",
			Task:     "Everything currently lives in `main`, which is hard to test because it reads the world and writes to it. Pick the innermost piece of work in `main.go` (the part that turns input values into output values, with no printing and no file access), move it into a named function that takes its inputs as parameters and returns its result, and call that function from `main`. Then write `main_test.go` with a table-driven test for it.",
			Success:  "`go run .` still produces exactly the output it produced before, `go test ./...` passes, and `main` is short enough to read in one screen.",
			Concepts: []string{"table-driven-tests", "multiple-return-values"},
		})
	}

	if trap := b.firstTrap(); trap != nil {
		out = append(out, Exercise{
			Title: fmt.Sprintf("Make the %q trap happen on purpose", trap.Title),
			Task: fmt.Sprintf("Read %s, then write the smallest possible program in a scratch directory that triggers the failure it describes, and run it. Once you have seen the failure with your own eyes, go back to `main.go` and find the place where the same shape appears in your converted code. Decide whether the generated code is actually safe there, and write a comment saying why.",
				link("the lesson", rel(fileExercises, conceptFile(trap.ID)))),
			Success:  "You have seen the real error message rather than read about it, and you can point at the line in `main.go` that would produce it if the guard were removed.",
			Concepts: []string{trap.ID},
		})
	}

	// Both of the next two name a construct rather than a line, so they are
	// only worth setting when the construct is actually in the output.
	if hits := exitCalls(b.in.GoSource); hits != "" {
		task := fmt.Sprintf("The generated code stops the program by calling %s.", hits)
		if hasFunc {
			task += " Pick a call site that is not in `main`, and change the function it sits in so that it returns an `error` as its last result."
		} else {
			task += " Everything is in `main` today, so start by moving the work that can fail into a function of its own, and have that function return an `error` as its last result."
		}
		out = append(out, Exercise{
			Title:    "Return an error instead of exiting",
			Task:     task + " Propagate the error up with `fmt.Errorf(\"doing the thing: %w\", err)` at each level, and let `main` be the only function that decides to stop the program. `defer` statements do not run when `os.Exit` is called, so this change also fixes cleanup you may not have noticed was being skipped.",
			Success:  "Only `main` calls `os.Exit` (or nothing does, and `main` simply returns). `go vet ./...` is silent, the program still exits with a non-zero status on the failure path (check with `go run . ; echo $?`), and the error message now says what was being attempted, not just what went wrong.",
			Concepts: []string{"errors-are-values", "if-err-nil-rhythm", "error-wrapping", "defer-timing"},
		})
	}

	if hasLoop(b.in.GoSource) {
		out = append(out, Exercise{
			Title:    "Replace one loop with a slices call",
			Task:     "Find a loop in the generated code that searches for an element, tests membership, or removes elements, and replace it with `slices.Contains`, `slices.IndexFunc`, `slices.ContainsFunc`, or `slices.DeleteFunc` from the standard library. Leave the loops that transform or accumulate alone: Go has no `map` or `grep` over slices, and the explicit loop is the idiom there, not a failure of taste. If no loop in this program is doing one of those three jobs, say so in a comment and move on; recognising that is the exercise.",
			Success:  "The program compiles, `go vet ./...` is silent, and running it against the same input produces byte-identical output (`go run . > after.txt` and `diff` it against the output you saved first). The file is shorter than it was.",
			Concepts: []string{"slices-not-arrays", "range-is-not-foreach", "sort-slice"},
		})
	}

	if e, ok := b.firstUnfinished(); ok {
		construct := e.Construct
		if construct == "" {
			construct = "the construct the converter refused"
		}
		task := fmt.Sprintf("The converter %s %s%s.", verbFor(e.Severity), construct, positionSuffix(e.Line, entryScript(e, b.script)))
		if strings.TrimSpace(e.Advice) != "" {
			task += " " + strings.TrimSpace(e.Advice)
		} else if strings.TrimSpace(e.Message) != "" {
			task += " " + strings.TrimSpace(e.Message)
		}
		task += " Implement it by hand in the generated code and delete the TODO that marks it."
		out = append(out, Exercise{
			Title:    unfinishedTitle(e.Severity, construct),
			Task:     task,
			Success:  "The TODO is gone, the program compiles, and you have a test that fails against the version without your fix. If you conclude the original behaviour was not worth reproducing, write that decision down in a comment; that is a valid answer, and an undocumented one is not.",
			Concepts: e.Concepts,
		})
	}

	out = append(out, Exercise{
		Title:    "Make the project idiomatic on its own terms",
		Task:     "Run `gofmt -l .` and `go vet ./...` and fix anything they report. Then add a package comment above `package main` in `main.go` saying in one sentence what the program does, name any single-letter variable that survives more than five lines, and delete anything the compiler tells you is unused. Finally run `go doc .` and read what your own package now says about itself.",
		Success:  "`gofmt -l .` prints nothing, `go vet ./...` prints nothing, and `go doc .` shows a sentence that would be useful to someone who has never seen this program.",
		Concepts: []string{"toolchain-gofmt-godoc", "packages-and-exported-names"},
	})

	return out
}

// unfinishedTitle names the task for a construct the converter did not
// finish. A refusal left a hole to fill; an approximation left something that
// runs and is wrong in a known way, which is a different verb.
func unfinishedTitle(sev report.Severity, construct string) string {
	if sev == report.Refuse {
		return "Finish " + construct
	}
	return "Fix " + construct
}

// exerciseTarget picks the function an exercise should name: the first
// declared function that is not main or init.
func (b *bundle) exerciseTarget() (string, bool) {
	for _, name := range b.funcs {
		if name != "main" && name != "init" {
			return name, true
		}
	}
	return "", false
}

// firstTrap returns the trap-severity lesson an exercise should send the
// reader at: one the developer's own code raised if there is one, since the
// task ends by finding the same shape in their file, and otherwise the first
// trap in reading order.
func (b *bundle) firstTrap() *Concept {
	var fallback *Concept
	for _, c := range b.concepts {
		if c.Severity != SeverityTrap {
			continue
		}
		if b.fromCode[c.ID] {
			return c
		}
		if fallback == nil {
			fallback = c
		}
	}
	return fallback
}

// firstUnfinished returns the first refusal, or the first approximation when
// nothing was refused.
func (b *bundle) firstUnfinished() (report.Entry, bool) {
	// An entry with no position and no source is about the conversion as a
	// whole rather than about a construct, so there is nothing in the file for
	// the reader to go and finish.
	located := func(e report.Entry) bool { return e.Line > 0 || strings.TrimSpace(e.Perl) != "" }
	for _, e := range b.entries {
		if e.Severity == report.Refuse && located(e) {
			return e, true
		}
	}
	for _, e := range b.entries {
		if e.Severity == report.Warn && located(e) {
			return e, true
		}
	}
	return report.Entry{}, false
}

// conceptIndex lists the lessons this conversion pulled in.
func (b *bundle) conceptIndex() string {
	m := &md{}
	m.h(1, "Concept lessons")

	if len(b.concepts) == 0 {
		m.p("This conversion triggered no lessons. That happens with small or very plain scripts: nothing in the file touches a place where the two languages disagree sharply enough to be worth a page of its own.")
		m.p("%s is the general tour and does not depend on what your script contained. %s is the specific one.",
			link("Go for Perl developers", rel(fileConceptIdx, fileGuide)),
			link("The walkthrough", rel(fileConceptIdx, fileWalk)))
		m.raw(b.credit())
		return m.String()
	}

	var direct, background []*Concept
	for _, c := range b.concepts {
		if b.fromCode[c.ID] {
			direct = append(direct, c)
			continue
		}
		background = append(background, c)
	}

	m.p("These %d lesson%s were selected by what is in your code, not from a fixed syllabus. They are ordered so that a lesson never depends on one further down the list, which makes top to bottom a sensible way to read them.", len(b.concepts), plural(len(b.concepts)))
	m.p("A lesson marked as a trap describes something that produces a crash or wrong data from code that looks correct. Read those first if you are short on time.")

	entry := func(i int, c *Concept) {
		label := ""
		switch c.Severity {
		case SeverityTrap:
			label = " (trap)"
		case SeverityWarning:
			label = " (easy to get wrong)"
		}
		m.numbered(i, "%s%s. %s", link(c.Title, rel(fileConceptIdx, conceptFile(c.ID))), label, sentence(firstParagraph(c.Body)))
	}

	if len(background) == 0 {
		for i, c := range b.concepts {
			entry(i+1, c)
		}
	} else {
		m.h(2, "Raised by your code")
		if name := b.scriptOrProgram(); name != "" {
			m.p("Something in `%s` triggered each of these directly. Each lesson says at the top which part of your file pulled it in.", name)
		} else {
			m.p("Something in your file triggered each of these directly. Each lesson says at the top which part of it pulled the lesson in.")
		}
		for i, c := range direct {
			entry(i+1, c)
		}

		m.h(2, "Background the rest builds on")
		m.p("Nothing in your file triggered %s directly. %s here because the lessons above, or the exercises, rely on %s, and reading %s first makes the others shorter.",
			itOrThem(len(background)), theyAre(len(background)), itOrThem(len(background)), itOrThem(len(background)))
		for i, c := range background {
			entry(i+1, c)
		}
	}

	m.rule()
	m.p("To read any of these outside a conversion, or to look up something that is not here, run `perl2golang explain <topic>`; `perl2golang explain --list` prints every topic the tool knows.")
	m.raw(b.credit())
	return m.String()
}

// conceptPage renders one lesson with the cross-references that tie it to this
// particular conversion.
func (b *bundle) conceptPage(c *Concept) string {
	rendered := strings.TrimRight(c.Render(), "\n")
	head, body, found := strings.Cut(rendered, "\n\n")
	if !found {
		head, body = rendered, ""
	}

	m := &md{}
	m.raw(head)
	if why := b.whyLine(c); why != "" {
		m.p("%s", why)
	}
	m.raw(b.linkMentions(body, conceptFile(c.ID), c.ID))

	m.rule()
	if links := b.conceptLinks(c.Prerequisites, conceptFile(c.ID)); links != "" {
		m.p("Read first: %s.", links)
	}
	if links := b.conceptLinks(b.next[c.ID], conceptFile(c.ID)); links != "" {
		m.p("Builds towards: %s.", links)
	}
	m.p("Back to %s, or to %s.",
		link("the lesson index", rel(conceptFile(c.ID), fileConceptIdx)),
		link("the walkthrough of your file", rel(conceptFile(c.ID), fileWalk)))
	m.raw(b.credit())
	return m.String()
}

// whyLine says what in the developer's code pulled this lesson in.
func (b *bundle) whyLine(c *Concept) string {
	reasons := dedupe(b.why[c.ID])
	if len(reasons) > 3 {
		rest := len(reasons) - 3
		reasons = append(reasons[:3:3], fmt.Sprintf("%d other place%s", rest, plural(rest)))
	}
	if len(reasons) > 0 {
		return "Why this came up in your code: " + joinList(reasons) + "."
	}

	var needs []string
	for _, id := range b.next[c.ID] {
		if dep, ok := b.inBundle[id]; ok {
			needs = append(needs, link(dep.Title, rel(conceptFile(c.ID), conceptFile(id))))
		}
	}
	if len(needs) > 0 {
		builds, lesson := "builds", "that lesson is also in this bundle"
		if len(needs) > 1 {
			builds, lesson = "build", "those lessons are also in this bundle"
		}
		return fmt.Sprintf("Why this came up in your code: nothing in your file triggered this lesson directly. It is here because %s %s on it, and %s.", joinList(needs), builds, lesson)
	}
	return "Why this came up in your code: the converter flagged it while translating this file."
}

// linkMentions turns the lesson-id references inside a page into links, from
// the file at from. The knowledge base and the orientation guide both name
// related lessons in backticks (`nil-vs-undef`), which is dead text on a page;
// here it becomes navigation. Only lessons this bundle carries are linked, and
// never inside a fenced code block.
func (b *bundle) linkMentions(body, from, exclude string) string {
	if body == "" {
		return ""
	}
	ids := make([]string, 0, len(b.inBundle))
	for id := range b.inBundle {
		if id != exclude {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)

	lines := strings.Split(body, "\n")
	fence := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if n := leadingBackticks(trimmed); n >= 3 {
			fence = trimmed[:n]
			continue
		}
		for _, id := range ids {
			needle := "`" + id + "`"
			if !strings.Contains(line, needle) {
				continue
			}
			line = strings.ReplaceAll(line, needle, link(needle, rel(from, conceptFile(id))))
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// orientation returns the general guide: the embedded long-form document when
// it is present, and a knowledge-base summary when it is not.
func (b *bundle) orientation() string {
	if strings.TrimSpace(b.guide) == "" {
		return b.orientationFallback()
	}

	text := strings.TrimRight(b.guide, "\n")
	if !strings.HasPrefix(strings.TrimSpace(text), "#") {
		text = "# Go for Perl developers\n\n" + text
	}

	m := &md{}
	m.raw(b.linkMentions(b.fixGuideLinks(text), fileGuide, ""))
	m.rule()
	m.p("This document is the same in every conversion. The parts that are specific to your file are %s and %s.",
		link("the walkthrough", rel(fileGuide, fileWalk)),
		link("the lessons your code triggered", rel(fileGuide, fileConceptIdx)))
	m.raw(b.credit())
	return m.String()
}

// fixGuideLinks makes the copied guide's relative links resolve inside this
// bundle. A link to a lesson the bundle carries is pointed at it; a link to one
// this conversion did not trigger becomes plain text with a pointer to the
// explain command, because the bundle only ships the lessons the code earned.
func (b *bundle) fixGuideLinks(text string) string {
	links := findLinks(text)
	if len(links) == 0 {
		return text
	}

	var out strings.Builder
	last := 0
	for _, l := range links {
		target := l.target
		if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		clean, _, _ := strings.Cut(target, "#")
		key := path.Join(path.Dir(fileGuide), clean)
		replacement := ""
		switch id := strings.TrimSuffix(path.Base(clean), ".md"); {
		case key == fileGuide:
			continue
		case b.hasKey(key):
			replacement = link(l.text, rel(fileGuide, key))
		case b.inBundle[id] != nil:
			replacement = link(l.text, rel(fileGuide, conceptFile(id)))
		default:
			replacement = l.text
			if _, known := b.kb.Get(id); known {
				replacement += " (not included in this conversion; run `perl2golang explain " + id + "`)"
			}
		}
		out.WriteString(text[last:l.start])
		out.WriteString(replacement)
		last = l.end
	}
	out.WriteString(text[last:])
	return out.String()
}

// hasKey reports whether the bundle contains the given document.
func (b *bundle) hasKey(key string) bool {
	switch key {
	case fileReadme, fileStartHere, fileReport, fileWalk, fileNotTrans, fileExercises, fileGuide, fileConceptIdx:
		return true
	}
	id := strings.TrimSuffix(path.Base(key), ".md")
	return path.Dir(key) == conceptDir && b.inBundle[id] != nil
}

// orientationFallback composes a short orientation from the knowledge base, so
// that a bundle is never missing its general guide.
func (b *bundle) orientationFallback() string {
	m := &md{}
	m.h(1, "Go for Perl developers")
	m.p("A short orientation, written for someone who already knows how to program and is meeting Go's decisions for the first time. It is not a language tutorial: the reference at https://go.dev/ref/spec and the tour at https://go.dev/tour are better at that than any generated document. What follows is the set of places where an expert's instincts transfer badly, which is a different list from the one a beginner needs.")

	m.h(2, "The ten differences that matter first")
	m.bullet("Types are static and everything has a zero value. There is no `undef` for an `int`; an uninitialised `int` is `0` and an uninitialised `string` is `\"\"`. Absence has to be modelled deliberately, usually with a pointer or a second boolean result.")
	m.bullet("`nil` is not `undef`. Only pointers, maps, slices, channels, functions, and interfaces can be nil, dereferencing a nil pointer panics rather than warns, and nothing autovivifies: writing through a nil inner map is a crash.")
	m.bullet("Slices are views onto arrays, and `append` may or may not copy. Two slices can share memory, so writing through one changes the other until a reallocation silently separates them.")
	m.bullet("Map iteration order is deliberately randomised on every run. Code that depends on hash order fails intermittently rather than consistently; sort the keys when order matters.")
	m.bullet("Errors are values, not exceptions. A function that can fail returns an `error` alongside its result, callers check it with `if err != nil`, and nothing unwinds the stack on your behalf.")
	m.bullet("`defer` runs at function exit, not at the end of the enclosing block, and it does not run at all if the program calls `os.Exit`.")
	m.bullet("Interfaces are satisfied implicitly. A type never declares that it implements one; if it has the methods, it fits, which makes small interfaces defined next to the consumer the normal design.")
	m.bullet("Concurrency is goroutines and channels rather than processes. Shared memory is real, so the race detector (`go test -race`) is part of the working routine, not an advanced tool.")
	m.bullet("Capitalisation is visibility. An identifier starting with an upper-case letter is exported from its package and a lower-case one is not; there is no separate export list.")
	m.bullet("The toolchain is not optional culture. `gofmt` settles formatting arguments, `go vet` catches a specific set of real bugs, `go test` runs table-driven tests, and `go doc` renders the comments you already wrote.")

	m.h(2, "The orientation lessons")
	m.p("These knowledge-base entries cover the ground above in more detail, each with runnable Go:")
	for _, c := range b.kb.ByTag("orientation") {
		if _, ok := b.inBundle[c.ID]; ok {
			m.bullet("%s: %s", link(c.Title, rel(fileGuide, conceptFile(c.ID))), sentence(firstParagraph(c.Body)))
			continue
		}
		m.bullet("%s: %s Run `perl2golang explain %s` to read it.", c.Title, sentence(firstParagraph(c.Body)), c.ID)
	}

	m.rule()
	m.p("The parts of this bundle that are specific to your file are %s and %s.",
		link("the walkthrough", rel(fileGuide, fileWalk)),
		link("the lessons your code triggered", rel(fileGuide, fileConceptIdx)))
	m.raw(b.credit())
	return m.String()
}

// conceptLinks renders the lessons among ids that this bundle carries, as a
// comma-separated list of links relative to from.
func (b *bundle) conceptLinks(ids []string, from string) string {
	var out []string
	for _, id := range dedupe(ids) {
		c, ok := b.inBundle[id]
		if !ok {
			continue
		}
		out = append(out, link(c.Title, rel(from, conceptFile(id))))
	}
	return joinList(out)
}

// credit is the provenance line at the foot of every document.
func (b *bundle) credit() string {
	if b.in.Version == "" {
		return "Written by perl2golang, from your source."
	}
	return "Written by perl2golang " + b.in.Version + ", from your source."
}

// guideText reads the embedded long-form guide, returning an empty string when
// it has not been written yet.
func guideText() (string, error) {
	data, err := fs.ReadFile(guideFS, guideSource)
	switch {
	case err == nil:
		return string(data), nil
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	default:
		return "", fmt.Errorf("reading the embedded guide: %w", err)
	}
}

// displayName returns the input's name for use in prose, or an empty string
// when the input had no file name.
func displayName(in DocInput, rep *report.Report) string {
	name := strings.TrimSpace(in.ScriptName)
	if name == "" && rep != nil {
		name = strings.TrimSpace(rep.Source)
	}
	switch name {
	case "", "-", "-e":
		return ""
	}
	return path.Base(name)
}

// programName returns the name of the generated program, falling back to the
// input's base name and then to a neutral word.
func programName(in DocInput, script string) string {
	if name := strings.TrimSpace(in.ProgramName); name != "" {
		return name
	}
	if script != "" {
		if base := strings.TrimSuffix(script, path.Ext(script)); base != "" {
			return base
		}
	}
	return ""
}

// goFuncNames returns the functions declared in the generated source, in the
// order they appear. It reads the text rather than parsing it, because the
// source may be a fragment.
func goFuncNames(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		rest, ok := strings.CutPrefix(line, "func ")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, "(") { // a method: skip the receiver
			close := strings.IndexByte(rest, ')')
			if close < 0 {
				continue
			}
			rest = strings.TrimSpace(rest[close+1:])
		}
		name := rest
		if i := strings.IndexAny(name, "([ "); i >= 0 {
			name = name[:i]
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// exitCalls names the ways the generated code stops the program, for the
// exercise that replaces them with errors.
func exitCalls(src string) string {
	var found []string
	for _, call := range []string{"os.Exit", "log.Fatal", "panic("} {
		if strings.Contains(src, call) {
			found = append(found, "`"+strings.TrimSuffix(call, "(")+"`")
		}
	}
	return joinList(found)
}

// hasLoop reports whether the generated code contains a loop, so that a task
// about loops is only set when there is one to work on.
func hasLoop(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "for ") || strings.HasPrefix(t, "for{") || t == "for {" {
			return true
		}
	}
	return false
}

// exportedish returns a name usable in a test function name.
func exportedish(name string) string {
	if name == "" {
		return "Function"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// verbFor describes what the converter did to a construct.
func verbFor(s report.Severity) string {
	if s == report.Refuse {
		return "refused"
	}
	return "approximated"
}

func verbIs(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

// constructsWere renders "1 construct was" or "3 constructs were".
func constructsWere(n int) string {
	if n == 1 {
		return "1 construct was"
	}
	return fmt.Sprintf("%d constructs were", n)
}

// itOrThem agrees a pronoun with a count.
func itOrThem(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}

// theyAre agrees the subject of a sentence with a count.
func theyAre(n int) string {
	if n == 1 {
		return "It is"
	}
	return "They are"
}

// scriptOrProgram names the input in prose, falling back to the generated
// program when the input had no file name.
func (b *bundle) scriptOrProgram() string {
	if b.script != "" {
		return b.script
	}
	return b.program
}

// positionSuffix renders " at line 42 of report.pl", or as much of it as is
// known.
// entryScript names the file an entry came from, which is the file being
// converted unless the entry came out of a module beside it.
func entryScript(e report.Entry, script string) string {
	if e.File != "" {
		return e.File
	}
	return script
}

func positionSuffix(line int, script string) string {
	switch {
	case line > 0 && script != "":
		return fmt.Sprintf(" at line %d of `%s`", line, script)
	case line > 0:
		return fmt.Sprintf(" at line %d", line)
	default:
		return ""
	}
}

// linesSuffix renders " at line 42 of report.pl", or " at lines 34 and 47 of
// report.pl" when one construct was reported more than once.
func linesSuffix(lines []int, script string) string {
	var known []string
	for _, l := range lines {
		if l > 0 {
			known = append(known, fmt.Sprint(l))
		}
	}
	// A heading that names two dozen line numbers is a heading nobody reads
	// to the end of. Past a handful, the span says as much and the list of
	// expressions under it says the rest.
	const spell = 6
	where := joinList(known)
	if len(known) > spell {
		where = fmt.Sprintf("%s to %s", known[0], known[len(known)-1])
	}
	switch {
	case len(known) == 0:
		return ""
	case len(known) == 1 && script != "":
		return fmt.Sprintf(" at line %s of `%s`", known[0], script)
	case len(known) == 1:
		return fmt.Sprintf(" at line %s", known[0])
	case len(known) > spell && script != "":
		return fmt.Sprintf(" at %d places in `%s`, lines %s", len(known), script, where)
	case len(known) > spell:
		return fmt.Sprintf(" at %d places, lines %s", len(known), where)
	case script != "":
		return fmt.Sprintf(" at lines %s of `%s`", where, script)
	default:
		return fmt.Sprintf(" at lines %s", where)
	}
}

// lineRangeSuffix renders " (lines 12 to 20)" when the range is known.
func lineRangeSuffix(from, to int) string {
	switch {
	case from > 0 && to > from:
		return fmt.Sprintf(" (lines %d to %d)", from, to)
	case from > 0:
		return fmt.Sprintf(" (line %d)", from)
	default:
		return ""
	}
}

// sourceIntro introduces a quoted region of the original.
func sourceIntro(script string, from, to int) string {
	name := "the original"
	if script != "" {
		name = "`" + script + "`"
	}
	switch {
	case from > 0 && to > from:
		return fmt.Sprintf("%s, lines %d to %d:", name, from, to)
	case from > 0:
		return fmt.Sprintf("%s, line %d:", name, from)
	default:
		return fmt.Sprintf("From %s:", name)
	}
}

// joinList renders items as "a", "a and b", or "a, b, and c".
func joinList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

// capitalise upper-cases the first letter of a sentence that does not start
// with markup.
func capitalise(s string) string {
	if s == "" || s[0] == '`' || s[0] == '[' {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// firstParagraph returns the first prose paragraph of a concept body, which is
// the "why you care" opening every lesson starts with.
func firstParagraph(body string) string {
	var para []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" && len(para) > 0:
			return strings.Join(para, " ")
		case trimmed == "", strings.HasPrefix(trimmed, "#"), strings.HasPrefix(trimmed, "```"):
			continue
		default:
			para = append(para, trimmed)
		}
	}
	return strings.Join(para, " ")
}

// dedent removes the common leading whitespace from a quoted source region, so
// that a fragment taken from the middle of a file does not render indented.
func dedent(src string) string {
	src = strings.ReplaceAll(src, "\t", "    ")
	lines := strings.Split(strings.TrimRight(src, "\n \t"), "\n")

	indent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " "))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	if indent <= 0 {
		return strings.Join(lines, "\n")
	}
	for i, line := range lines {
		if len(line) >= indent {
			lines[i] = line[indent:]
		} else {
			lines[i] = strings.TrimLeft(line, " ")
		}
	}
	return strings.Join(lines, "\n")
}
