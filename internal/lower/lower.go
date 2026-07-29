// Package lower turns a parsed Perl program into the typed IR the Go emitter
// prints.
//
// This is where the interesting decisions live. A Perl scalar has no type and
// Go insists on one; Perl evaluates in list or scalar context and Go has no
// such notion; Perl reports failure by dying and Go returns errors. Every one
// of those gaps is closed here, deliberately and visibly: each IR node carries
// the Perl it came from and a note explaining the Go, so the annotated program
// and the walkthrough document can be generated from the same tree that
// produced the clean program.
//
// The pass runs twice. The first pass discovers what types the variables want
// to be; the second builds the IR for real with those types known. Running the
// same code twice is simpler and far less error-prone than maintaining a
// separate inference walker that has to agree with the lowering walker about
// what every expression means.
package lower

import (
	"sort"
	"strings"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
	"perl2go/internal/perl/parser"
	"perl2go/internal/perl/token"
	"perl2go/internal/report"
)

// Options configure a lowering run.
type Options struct {
	// File is the source name used in diagnostics.
	File string
	// Program is the generated program's identifier-safe name.
	Program string
	// Module is the generated Go module path.
	Module string
}

// Result is everything one lowering run produced.
type Result struct {
	Program *ir.Program
	Report  *report.Report
	// Helpers names the runtime support functions the generated code calls.
	Helpers []string
	// TopLevel are the IR statements that make up main, in source order,
	// paired with the Perl that produced them. The walkthrough is built
	// from these.
	TopLevel []ir.Stmt
}

// Lowerer holds the state of one conversion.
type Lowerer struct {
	opts  Options
	src   string
	lines []string

	// pass is 1 while types are being discovered and 2 while the real IR is
	// being built. Diagnostics and notes are only recorded on pass 2, so
	// nothing is reported twice.
	pass int

	scope  *scope
	names  *nameSet
	subs   map[string]*Sub
	subOrd []string

	// anonSubs records the synthetic Sub behind each `sub { ... }`, keyed by
	// the AST node so both passes agree about which one they are looking at.
	anonSubs map[*ast.AnonSub]*Sub
	anonOrd  []*Sub

	// decls maps a declaration site to its binding so the second pass
	// reuses exactly the records the first pass built.
	decls map[ast.Node]*Binding

	// aliases rewrites a binding to an arbitrary expression. It exists for
	// the loops where Perl's aliasing was load-bearing and the Go form has
	// to index back into the slice.
	aliases map[*Binding]ir.Expr

	// globals are package-level variables, in declaration order.
	globals    []*Binding
	globalSeen map[string]*Binding
	// constants are the declarations `use constant` produced.
	constants []*ir.VarDecl
	// constNames maps a `use constant` name to its binding, so a bareword
	// use of it resolves to the constant rather than looking like a call.
	constNames map[string]*Binding

	// pre holds statements that must be emitted before the statement being
	// lowered. Perl expressions do work that Go can only do in statement
	// position, so a lowering step can push a setup statement here and the
	// statement loop flushes it.
	pre []ir.Stmt

	// helpers records which runtime support functions were used.
	helpers    map[string]bool
	helperOrd  []string
	patterns   map[string]*patternVar
	patternOrd []string

	rep      *report.Report
	tmpSeq   int
	curSub   *Sub
	loopDeep int
	// topicStack tracks what $_ currently refers to.
	topicStack []ir.Expr
	// captureStack tracks the identifier holding the current regex
	// submatch slice, so $1 and friends resolve.
	captureStack []*captureFrame
	// usedExit records that the program calls os.Exit somewhere.
	usedExit bool
	// errVar names the error variable in scope, so $! resolves to the error
	// the failing call actually returned.
	errVar string
	// curStmt is the statement being lowered, used to place a diagnostic
	// whose own node has no usable position.
	curStmt ast.Node
	// seenEntry keeps one report entry per situation per place.
	seenEntry map[string]bool
	// tmpNames hands out temporary identifiers, seeded on the second pass
	// with every name a variable already claimed.
	tmpNames *nameSet
	// usedLabels records which loop labels something actually branches to.
	usedLabels map[string]bool
	// lastStat is the path the most recent file test asked about, so that
	// `-f _` has something to reuse.
	lastStat ast.Expr
	// countsLines records that the program read $., which the line-reading
	// loops have to keep up to date. It is discovered on the first pass and
	// acted on by the second.
	countsLines bool
	// readLoops counts the line-reading loops in the file, discovered on the
	// first pass. More than one means they share the line counter and each
	// has to start it again.
	readLoops int
	// traps records that the program catches a failure with eval, which
	// changes what die has to compile into. It is discovered on the first
	// pass and acted on by the second.
	traps bool
}

// label returns a loop label only when something branches to it. Go rejects a
// label nothing uses, where Perl is happy to let one sit there.
func (l *Lowerer) label(name string) string {
	if name == "" || !l.usedLabels[name] {
		return ""
	}
	return name
}

// captureFrame is one active regex match whose groups are in scope.
type captureFrame struct {
	// Name is the Go identifier holding the []string of submatches.
	Name string
	// Count is how many capture groups the pattern has.
	Count int
	// Named maps a named capture to its group index.
	Named map[string]int
}

// patternVar is a regular expression hoisted to package level.
type patternVar struct {
	Name    string
	GoRegex string
	Perl    string
	Line    int
}

// Lower converts a parsed program.
func Lower(res parser.Result, src []byte, opts Options) *Result {
	l := &Lowerer{
		opts:       opts,
		src:        string(src),
		lines:      strings.Split(string(src), "\n"),
		names:      newNameSet(),
		subs:       map[string]*Sub{},
		decls:      map[ast.Node]*Binding{},
		globalSeen: map[string]*Binding{},
		helpers:    map[string]bool{},
		patterns:   map[string]*patternVar{},
		rep: &report.Report{
			Source: opts.File,
			Module: opts.Module,
		},
	}
	// main and the package name are taken; nothing else may claim them.
	l.names.reserve("main")
	l.names.reserve("err")

	for _, d := range res.Diags {
		l.rep.Stats.ParseErrors++
		_ = d
	}

	l.hoistSubs(res.Program.Stmts)

	// Pass 1 discovers types. Its IR is thrown away.
	l.pass = 1
	l.scope = newScope(nil)
	l.run(res.Program)
	l.resolveTypes()

	// Pass 2 builds the real tree.
	l.pass = 2
	l.scope = newScope(nil)
	l.tmpSeq = 0
	l.tmpNames = newNameSet()
	for _, b := range l.decls {
		l.tmpNames.reserve(b.Go)
	}
	for _, b := range l.globals {
		l.tmpNames.reserve(b.Go)
	}
	for _, name := range l.subOrd {
		l.tmpNames.reserve(l.subs[name].Go)
	}
	l.tmpNames.reserve("main")
	l.constants = nil
	l.patterns = map[string]*patternVar{}
	l.patternOrd = nil
	l.helpers = map[string]bool{}
	l.helperOrd = nil
	out := l.run(res.Program)

	l.finishReport()
	return out
}

// run performs one whole pass over the program.
func (l *Lowerer) run(prog *ast.Program) *Result {
	file := &ir.File{Name: "main.go", Package: "main"}

	var top []ir.Stmt
	for _, st := range prog.Stmts {
		if sd, ok := st.(*ast.SubDecl); ok {
			l.lowerSubDecl(sd)
			continue
		}
		top = append(top, l.stmts([]ast.Stmt{st})...)
	}

	// Package-level regular expressions come first: they are compiled once
	// at start-up, which is the Go idiom and a lesson in itself.
	for _, name := range l.patternOrd {
		p := l.patterns[name]
		file.Decls = append(file.Decls, l.patternDecl(p))
	}
	for _, d := range l.constants {
		file.Decls = append(file.Decls, d)
	}
	for _, g := range l.globals {
		file.Decls = append(file.Decls, l.globalDecl(g))
	}

	if l.traps {
		// A failure nothing traps has to end the program the way it did
		// before, with its message on standard error rather than with a
		// stack trace.
		top = append([]ir.Stmt{l.mainRecover()}, top...)
	}
	mainFn := &ir.FuncDecl{Name: "main", Body: l.markUnused(&ir.Block{Stmts: top})}
	l.annotateMain(mainFn)
	file.Decls = append(file.Decls, mainFn)

	for _, name := range l.subOrd {
		if fn := l.subs[name].irDecl; fn != nil {
			file.Decls = append(file.Decls, fn)
		}
	}

	prog2 := &ir.Program{
		Module:   l.opts.Module,
		Files:    []*ir.File{file},
		Helpers:  append([]string(nil), l.helperOrd...),
		Concepts: append([]string(nil), l.rep.Concepts...),
	}
	return &Result{Program: prog2, Report: l.rep, Helpers: l.helperOrd, TopLevel: top}
}

// annotateMain gives main its doc comment and its opening lesson.
func (l *Lowerer) annotateMain(fn *ir.FuncDecl) {
	fn.Doc = []string{"main is the program's entry point."}
	if l.pass != 2 {
		return
	}
	ir.Annotate(fn,
		"Every Go program starts at func main in package main. A Perl script's "+
			"top-level statements have no enclosing function, so they all live here, "+
			"in the order they were written.")
	ir.Annotate(fn,
		"Go has no implicit exit status from the last statement: falling off the "+
			"end of main exits 0. Anything else has to call os.Exit explicitly.",
		"static-types-and-zero-values")
}

// ---------------------------------------------------------------------------
// Source access

// posOf returns the 1-based line and column of a node.
func posOf(n ast.Node) (int, int) {
	if n == nil {
		return 0, 0
	}
	p := n.Pos()
	return p.Line, p.Col
}

// snippet returns the original source text of a node, trimmed, with interior
// runs of blank lines collapsed. It is what the annotated output quotes.
func (l *Lowerer) snippet(n ast.Node) string {
	if n == nil {
		return ""
	}
	from, to := n.Pos().Offset, n.End().Offset
	if from < 0 || to > len(l.src) || to <= from {
		// Fall back to the whole line, which is always useful even when the
		// end position is missing.
		if ln := n.Pos().Line; ln >= 1 && ln <= len(l.lines) {
			return strings.TrimSpace(l.lines[ln-1])
		}
		return ""
	}
	return strings.TrimRight(strings.TrimLeft(l.src[from:to], " \t"), " \t\n")
}

// prov builds the provenance record for a node.
func (l *Lowerer) prov(n ast.Node) ir.Provenance {
	line, col := posOf(n)
	return ir.Provenance{Line: line, Col: col, Text: l.snippet(n)}
}

// setProv stamps a node's origin onto an IR node.
func (l *Lowerer) setProv(target ir.Annotated, n ast.Node) {
	if target == nil || n == nil {
		return
	}
	m := ir.MetaOf(target)
	if m == nil || m.Prov.Valid() {
		return
	}
	m.Prov = l.prov(n)
}

// ---------------------------------------------------------------------------
// Notes and diagnostics

// note attaches an explanation to an IR node and records the concepts it
// touches. Notes are only rendered into the annotated program, so this is the
// main channel through which the tool teaches.
func (l *Lowerer) note(target ir.Annotated, text string, concepts ...string) {
	if l.pass != 2 || target == nil {
		return
	}
	ir.Annotate(target, text, concepts...)
	for _, c := range concepts {
		l.rep.AddConcept(c)
	}
}

// concept records a triggered teaching concept without attaching a note.
func (l *Lowerer) concept(ids ...string) {
	if l.pass != 2 {
		return
	}
	for _, id := range ids {
		l.rep.AddConcept(id)
	}
}

// entry records a report entry. Everything the converter approximated or
// refused goes through here, so the report, the terminal summary and the
// generated documents all see the same facts.
//
// Two entries with the same code at the same place are one fact stated twice,
// which is noise in a report whose whole value is that it is read.
func (l *Lowerer) entry(e report.Entry, n ast.Node) report.Entry {
	if l.pass != 2 {
		return e
	}
	n = l.locate(n)
	e.Line, e.Col = posOf(n)
	if e.Perl == "" {
		e.Perl = l.snippet(n)
	}
	key := e.Code + ":" + itoa(e.Line) + ":" + itoa(e.Col) + ":" + e.Construct
	if l.seenEntry[key] {
		return e
	}
	if l.seenEntry == nil {
		l.seenEntry = map[string]bool{}
	}
	l.seenEntry[key] = true
	l.rep.Add(e)
	return e
}

// locate falls back to the enclosing statement when a node's position is not
// usable.
//
// An expression embedded in a double-quoted string is parsed from that string
// rather than from the file, so its offsets belong to the fragment. A child
// node can never begin before the statement that contains it, and when one
// appears to, the statement is the honest answer.
func (l *Lowerer) locate(n ast.Node) ast.Node {
	if n == nil {
		return l.curStmt
	}
	if l.curStmt == nil {
		return n
	}
	if n.Pos().Offset < l.curStmt.Pos().Offset || n.Pos().Line < 1 {
		return l.curStmt
	}
	return n
}

// refuse records a construct the converter will not translate and returns the
// Todo to bury in the generated code. A refusal is a finished answer, not a
// placeholder: it says exactly what was refused, why Go cannot express it the
// same way, and what to do instead.
func (l *Lowerer) refuse(n ast.Node, code, construct, short, message, advice string, concepts ...string) ir.Todo {
	e := l.entry(report.Entry{
		Code:      code,
		Severity:  report.Refuse,
		Construct: construct,
		Short:     short,
		Message:   message,
		Advice:    advice,
		Concepts:  concepts,
	}, n)
	line, col := posOf(n)
	return ir.Todo{
		Code:    code,
		Short:   short,
		Message: message + " " + advice,
		Perl:    e.Perl,
		Prov:    ir.Provenance{Line: line, Col: col, Text: e.Perl},
	}
}

// approximate records a construct that was converted, but not exactly. The
// generated code runs; it just does not behave identically in every case, and
// the developer has to know which cases.
func (l *Lowerer) approximate(n ast.Node, code, construct, short, message, advice string, concepts ...string) {
	l.entry(report.Entry{
		Code:      code,
		Severity:  report.Warn,
		Construct: construct,
		Short:     short,
		Message:   message,
		Advice:    advice,
		Concepts:  concepts,
	}, n)
}

// inform records something worth knowing that is not a defect.
func (l *Lowerer) inform(n ast.Node, code, construct, message string, concepts ...string) {
	l.entry(report.Entry{
		Code:      code,
		Severity:  report.Note,
		Construct: construct,
		Short:     construct,
		Message:   message,
		Concepts:  concepts,
	}, n)
}

// ---------------------------------------------------------------------------
// Helpers, temporaries and imports

// use registers a runtime support helper and returns the identifier the
// generated code calls it by.
func (l *Lowerer) use(name string) string {
	if !l.helpers[name] {
		l.helpers[name] = true
		l.helperOrd = append(l.helperOrd, name)
	}
	return name
}

// tmp hands out a fresh local identifier that no variable in the file is
// already using. Temporaries are unique across the whole file rather than per
// scope, which costs nothing and removes a whole class of shadowing bug from
// the generated code.
func (l *Lowerer) tmp(base string) string {
	if l.tmpNames == nil {
		return base
	}
	return l.tmpNames.take(base)
}

// emit queues a statement to appear before the statement being lowered.
func (l *Lowerer) emit(s ir.Stmt) {
	if s != nil {
		l.pre = append(l.pre, s)
	}
}

// takePre returns and clears the queued statements.
func (l *Lowerer) takePre() []ir.Stmt {
	out := l.pre
	l.pre = nil
	return out
}

// ---------------------------------------------------------------------------
// Report assembly

// finishReport fills in the counters that can only be known at the end.
func (l *Lowerer) finishReport() {
	l.rep.SortEntries()
	seen := map[*Binding]bool{}
	record := func(b *Binding) {
		if seen[b] || !b.declared() {
			return
		}
		seen[b] = true
		l.rep.Stats.Symbols++
		if !b.Dynamic {
			l.rep.Stats.SymbolsTyped++
		}
		l.rep.Symbols = append(l.rep.Symbols, report.Symbol{
			Name:     b.Perl,
			Type:     b.Type.String(),
			Inferred: !b.Dynamic,
			Reason:   b.Reason,
			Line:     b.Line,
		})
	}
	for _, b := range l.decls {
		record(b)
	}
	for _, b := range l.globals {
		record(b)
	}
	// The bindings live in a map, so they come out in whatever order the
	// runtime feels like. Sorting them makes two runs over the same input
	// produce the same report, which is what lets a report be diffed.
	sort.SliceStable(l.rep.Symbols, func(i, j int) bool {
		a, b := l.rep.Symbols[i], l.rep.Symbols[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	for _, e := range l.rep.Entries {
		if e.Severity == report.Refuse {
			l.rep.Stats.Todos++
		}
	}
	l.rep.Stats.Converted = l.rep.Stats.Statements - l.rep.Stats.Refused
	if l.rep.Stats.Converted < 0 {
		l.rep.Stats.Converted = 0
	}
}

// resolveTypes turns the evidence gathered on pass 1 into a decision per
// binding. A binding that saw one consistent type keeps it; one that saw
// several incompatible types becomes dynamic and is reported.
func (l *Lowerer) resolveTypes() {
	settle := func(b *Binding) {
		b.Used = b.Reads
		b.Reads = 0
		t := joinAll(b.Evidence)
		if t == nil {
			t = defaultFor(b.Sigil)
		}
		switch b.Sigil {
		case '@':
			if t.Kind != ir.Slice {
				t = ir.SliceOf(t)
			}
		case '%':
			if t.Kind != ir.Map {
				t = ir.MapOf(t)
			}
		}
		b.Type = t
		if isDynamic(scalarPart(t)) {
			b.Dynamic = true
			if b.Reason == "" {
				b.Reason = l.dynamicReason(b)
			}
		}
	}
	// Parameters settle first, because a function literal's type is built out
	// of them, and whatever holds that literal (a variable, a map of
	// callbacks) is inferred from the literal's type in turn. Settling the
	// containers before the signatures they hold is settled would infer them
	// from a signature that was still empty.
	settled := map[*Binding]bool{}
	once := func(b *Binding) {
		if settled[b] {
			return
		}
		settled[b] = true
		settle(b)
	}
	for _, b := range l.decls {
		if b.Kind == KindParam {
			once(b)
		}
	}
	l.settleSubs()
	for _, b := range l.decls {
		once(b)
	}
	for _, b := range l.globals {
		once(b)
	}
	for _, name := range l.subOrd {
		fillResults(l.subs[name])
	}
	for _, s := range l.anonOrd {
		fillResults(s)
	}
}

// fillResults replaces a result type nothing pinned down with the dynamic
// fallback, since a signature has to be written down in full.
func fillResults(s *Sub) {
	for i, r := range s.Results {
		if r == nil {
			s.Results[i] = ir.TAny
		}
	}
}

// pushCaptures records how deep the capture stack was, so a statement can drop
// whatever its condition pushed once the statement is finished.
func (l *Lowerer) captureDepth() int { return len(l.captureStack) }

func (l *Lowerer) restoreCaptures(depth int) {
	if depth <= len(l.captureStack) {
		l.captureStack = l.captureStack[:depth]
	}
}

// scalarPart reaches through a container to the value type inside it, which is
// what decides whether inference actually succeeded.
func scalarPart(t *ir.Type) *ir.Type {
	if t == nil {
		return nil
	}
	if t.Kind == ir.Slice || t.Kind == ir.Map {
		return t.Elem
	}
	return t
}

// dynamicReason explains, for the report, why a binding did not get a concrete
// type. The wording is aimed at someone who wants to fix it by hand.
func (l *Lowerer) dynamicReason(b *Binding) string {
	kinds := map[ir.TypeKind]bool{}
	for _, t := range b.Evidence {
		if s := scalarPart(t); s != nil {
			kinds[s.Kind] = true
		}
	}
	var names []string
	for k := range kinds {
		switch k {
		case ir.Int:
			names = append(names, "whole numbers")
		case ir.Float:
			names = append(names, "floating-point numbers")
		case ir.String:
			names = append(names, "text")
		case ir.Bool:
			names = append(names, "true/false values")
		}
	}
	switch len(names) {
	case 0:
		return "nothing in the file pins down what this holds"
	case 1:
		return "the values reaching it were not all " + names[0]
	default:
		return "it holds " + strings.Join(names, " and ") + " at different points, which no single Go type covers"
	}
}

// posLine is a small convenience for diagnostics that only need a line.
func posLine(n ast.Node) int {
	if n == nil {
		return 0
	}
	return n.Pos().Line
}

var _ = token.EOF
