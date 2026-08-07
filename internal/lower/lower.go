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

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
	"perl2golang/internal/perl/parser"
	perlrt "perl2golang/internal/runtime"
	"perl2golang/internal/perl/token"
	"perl2golang/internal/report"
)

// Options configure a lowering run.
type Options struct {
	// File is the source name used in diagnostics.
	File string
	// Program is the generated program's identifier-safe name.
	Program string
	// Module is the generated Go module path.
	Module string
	// Modules are the Perl modules the script pulls in from beside itself,
	// in load order. Their packages become types and functions in the one Go
	// package the conversion produces, because Go has no way to reach a
	// second file except through a second directory.
	Modules []SourceFile
}

// SourceFile is one parsed Perl file taking part in a conversion.
type SourceFile struct {
	// Path is what a diagnostic names, for example "Shape.pm".
	Path string
	Src  []byte
	Prog *ast.Program

	lines []string
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

	// classes are the packages the file declares, keyed by their Perl name,
	// and classOrd keeps them in declaration order so two runs agree.
	classes  map[string]*Class
	classOrd []string
	// byGoType finds a class from the generated type name, which is how an
	// expression's type answers "what class is this".
	byGoType map[string]*Class
	// units are the statements of every file taking part, each tagged with
	// the package it was written in and the file it came from. Perl lets one
	// file hold several packages and one program hold several files.
	units []pkgStmt
	// curFile is the file whose statements are being lowered, which decides
	// what a diagnostic quotes and which path it names.
	curFile *SourceFile
	// mainFile is the script itself, as opposed to a module beside it.
	mainFile *SourceFile
	// files are every source taking part, modules first and the script last,
	// which is the order perl loads them in.
	files []*SourceFile
	// loaded names the modules whose own file is part of this conversion.
	loaded map[string]bool
	// curPkg is the package whose code is being lowered, which decides how
	// an unqualified call resolves.
	curPkg string
	// hoisted marks a file-scope `my` whose variable a sub also reads. Perl
	// closes over it; Go needs one package-level variable for both to be
	// talking about the same thing.
	hoisted map[*ast.Var]bool
	// classVars marks the bindings that hold a class name rather than an
	// object: the $class of a constructor and the $proto it came from.
	classVars map[*Binding]*Class
	// optionHash maps a hash an option block fills in to the struct type it
	// became, keyed by the hash's Perl name.
	optionHash map[string]*Class
	// optionDests holds the type an option specification gave a destination
	// variable, which is a declaration and beats anything inference saw.
	optionDests map[*Binding]*ir.Type
	// optionSites records where each option destination was registered, so
	// that `defined $opt` after parsing can ask whether the option was given
	// rather than whether its value is the zero one.
	optionSites map[any]optionSite
	// bundling and passThrough record what Getopt::Long::Configure asked for,
	// which changes how the arguments are prepared before the flag set sees
	// them. They are read from the whole program before anything is lowered,
	// because a Configure call inside a sub still governs every block.
	bundling    bool
	passThrough bool
	// interfaces are the types a collection of several classes needed, keyed
	// by the ancestor and method set they were built from.
	interfaces   map[string]*Class
	interfaceOrd []*Class
	// isaFuncs and isaDecls hold the predicate declared for each class the
	// file asks isa about, which lists the concrete types that inherit from
	// it because Go embedding is not subtyping.
	isaFuncs map[*Class]string
	isaDecls []ir.Decl
	// classNameFunc names the generated function that maps a value's type
	// back to the class name the Perl knew it by.
	classNameFunc string
	// throwsObject records that some die in the file throws a blessed
	// object, which is what decides whether $@ holds text or a value.
	throwsObject bool
	// promoteWanted names the accessors a call reached through a value whose
	// class did not resolve, gathered fresh on every discovery round.
	promoteWanted map[string]bool
	// records holds the struct synthesised for each set of literal hash keys
	// the file uses as a record, so two literals of one shape share a type.
	records map[string]*Class
	// recordStack is the chain of record types whose literals are being
	// built, outermost first. A literal nested inside one whose keys are a
	// subset of its fields is the same kind of thing with fields left off,
	// a leaf node of a tree being the usual case, and takes that type.
	recordStack []*Class
	// recordEscaped marks record types whose values were also stored where
	// only `any` fits, so readers on the far side expect a map. Their
	// literals are built as maps from the sweep after the escape is seen.
	recordEscaped map[*Class]bool
	// recordLookups and recordDecls hold the by-name field reader declared
	// for a record whose field name is worked out while the program runs.
	recordLookups map[*Class]string
	recordDecls   []ir.Decl
	// recordUsed marks the record types a literal was actually built for, so
	// that a shape considered and rejected leaves no dead type behind.
	recordUsed map[*Class]bool
	// namedRecords maps a `my %h` declaration to the record type its
	// initialiser earns, decided before the first pass from the whole file.
	namedRecords map[*ast.Var]*Class
	// hints is the stack of names in scope for a synthesised type, innermost
	// last: the variable a literal is being stored in, mostly.
	hints []string
	// fieldAt remembers which struct field an access resolved to, so that a
	// write through it can say what the field holds. A field is not a binding
	// and has nowhere else to record the evidence.
	fieldAt map[ast.Node]*ClassField
	// curClass is the class whose method is being lowered.
	curClass *Class
	// arrowCalls and qualCalls record how the file calls its subs, which is
	// what decides whether a sub in a package is a method or a function.
	arrowCalls map[string]bool
	qualCalls  map[string]bool

	// anonSubs records the synthetic Sub behind each `sub { ... }`, keyed by
	// the AST node so both passes agree about which one they are looking at.
	anonSubs map[*ast.AnonSub]*Sub
	// blockSubs holds the function literal a bare block argument stands for,
	// keyed by its call so that every pass sees the same node.
	blockSubs map[*ast.Call]*ast.AnonSub
	anonOrd   []*Sub

	// decls maps a declaration site to its binding so the second pass
	// reuses exactly the records the first pass built.
	decls map[ast.Node]*Binding

	// aliases rewrites a binding to an arbitrary expression. It exists for
	// the loops where Perl's aliasing was load-bearing and the Go form has
	// to index back into the slice.
	aliases map[*Binding]ir.Expr

	// patternTodo is the refusal from the pattern most recently turned down,
	// waiting for whichever caller invents the stand-in expression so the
	// stand-in can carry the marker. A match that quietly reads as false is
	// the one shape a reader cannot tell from working code.
	patternTodo *ir.Todo

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
	// destroyPlans records, per declaring statement, where an object's
	// destructor call goes; destroyBound finds the plan again from the
	// binding when an undef names the instant itself.
	destroyPlans map[ast.Stmt]*destroyPlan
	destroyBound map[*Binding]*destroyPlan
	// spliced marks a lowered list part that stands for several values, so
	// the list builder knows to splice it in rather than store it as one
	// element. The type alone cannot say: an array flattens into a list and
	// a reference to one does not, and both are slices in Go.
	spliced map[ir.Expr]bool
	patterns   map[string]*patternVar
	patternOrd []string

	rep      *report.Report
	tmpSeq   int
	curSub   *Sub
	loopDeep int
	// findWalk is the directory walk being lowered, when the code being
	// lowered is the block a tree walk runs for every entry.
	findWalk *findWalk
	// topicStack tracks what $_ currently refers to.
	topicStack []ir.Expr
	// captureStack tracks the identifier holding the current regex
	// submatch slice, so $1 and friends resolve.
	captureStack []*captureFrame
	// integerPragma records that `use integer` is in force here. It changes
	// what / and % mean for the rest of the enclosing block, and it is
	// lexically scoped, so the block lowering saves and restores it.
	integerPragma bool
	// usedExit records that the program calls os.Exit somewhere.
	usedExit bool
	// errVar names the error variable in scope, so $! resolves to the error
	// the failing call actually returned.
	errVar string
	// seps holds what Perl's separator variables were last set to. Go has no
	// global of that kind, so their effect is folded into the calls they
	// govern while the file is being converted rather than while it runs.
	seps separators
	// curStmt is the statement being lowered, used to place a diagnostic
	// whose own node has no usable position.
	curStmt ast.Node
	// seenEntry keeps one report entry per situation per place.
	seenEntry map[string]bool
	// tmpNames hands out temporary identifiers, seeded on the second pass
	// with every name a variable already claimed.
	tmpNames *nameSet
	// redoLabel is the label of the loop a redo-wrapped body belongs to, or
	// the empty string when the body was wrapped and needs no label. It is
	// nil when the body being lowered is not inside such a wrapper.
	redoLabel *string
	// uniformFn forces the function literals being lowered to one signature,
	// which is what lets a collection of callbacks have an element type.
	uniformFn bool
	// transValue holds the result of a transliteration written with the r
	// modifier, which yields a new string instead of changing one.
	transValue ir.Expr
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
	// mixedReads records that the program reads single lines outside of
	// loops, which means every read on a handle has to share one buffered
	// reader or they would steal each other's read-ahead. Discovered on the
	// first pass and acted on by the second.
	mixedReads bool
}

// label returns a loop label only when something branches to it. Go rejects a
// label nothing uses, where Perl is happy to let one sit there.
// outerLabel is the label a next or last should carry: the one written in the
// source, or the one the redo wrapper gave the outer loop.
func (l *Lowerer) outerLabel(name string) string {
	if written := l.label(name); written != "" {
		return written
	}
	if name == "" && l.redoLabel != nil {
		return *l.redoLabel
	}
	return ""
}

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
		opts:        opts,
		src:         string(src),
		lines:       strings.Split(string(src), "\n"),
		names:       newNameSet(),
		subs:        map[string]*Sub{},
		classes:     map[string]*Class{},
		byGoType:    map[string]*Class{},
		hoisted:     map[*ast.Var]bool{},
		classVars:   map[*Binding]*Class{},
		optionHash:  map[string]*Class{},
		optionSites: map[any]optionSite{},
		loaded:      map[string]bool{},
		aliases:     map[*Binding]ir.Expr{},
		optionDests: map[*Binding]*ir.Type{},
		fieldAt:     map[ast.Node]*ClassField{},
		arrowCalls:  map[string]bool{},
		qualCalls:   map[string]bool{},
		curPkg:      "main",
		seps:        defaultSeparators(),
		decls:       map[ast.Node]*Binding{},
		globalSeen:  map[string]*Binding{},
		helpers:     map[string]bool{},
		spliced:     map[ir.Expr]bool{},
		patterns:    map[string]*patternVar{},
		rep: &report.Report{
			Source: opts.File,
			Module: opts.Module,
		},
	}
	// main and the package name are taken; nothing else may claim them.
	l.names.reserve("main")
	l.names.reserve("err")
	// So is every package the generated files may import. Imports are
	// file-scoped, but a package-level identifier conflicts with any file's
	// import of the same name, and the helper file's imports are not known
	// until emission. A user sub called fmt or sort takes a suffixed name
	// instead of breaking the build.
	for _, pkg := range []string{
		"bufio", "base64", "binary", "bytes", "cmp", "errors", "exec", "flag",
		"filepath", "fmt", "fs", "hash", "hex", "io", "json", "maps", "math",
		"md5", "os", "reflect", "regexp", "runtime", "slices", "sort",
		"strconv", "strings", "sync", "syscall", "time", "unicode", "utf8",
	} {
		l.names.reserve(pkg)
	}
	// And every helper the runtime library can emit beside the program: a
	// user sub that happens to share a name with one would otherwise
	// redeclare it the moment both are needed.
	for _, h := range perlrt.Names() {
		l.names.reserve(h)
	}

	for _, d := range res.Diags {
		l.rep.Stats.ParseErrors++
		_ = d
	}

	l.mainFile = &SourceFile{Path: opts.File, Src: src, Prog: res.Program, lines: l.lines}
	for i := range l.opts.Modules {
		m := &l.opts.Modules[i]
		m.lines = strings.Split(string(m.Src), "\n")
		l.files = append(l.files, m)
		for _, pkg := range declaredPackages(m.Prog.Stmts) {
			l.loaded[pkg] = true
		}
	}
	l.files = append(l.files, l.mainFile)
	l.curFile = l.mainFile

	l.collectClasses()
	l.collectOptions()
	l.collectRecordHashes()
	l.markShared()
	l.hoistSubs()

	// Pass 1 discovers types. Its IR is thrown away.
	//
	// It runs until the answers stop moving, because some of what it learns
	// depends on what it has already learned. A variable holding an object
	// takes its type from the constructor call; the method called on it takes
	// its parameter types from that call site; the struct field filled from
	// that parameter takes its type from there in turn. Each sweep resolves
	// one more link in such a chain, and the loop stops as soon as a sweep
	// changes nothing, which on an ordinary script is after two or three.
	prev := ""
	for round := 0; round < maxDiscoveryRounds; round++ {
		l.pass = 1
		l.scope = newScope(nil)
		l.curPkg = "main"
		l.seps = defaultSeparators()
		// A round sees every return the file has, so what the previous round
		// concluded is stale rather than extra evidence. Keeping it would
		// leave a signature promising the type a variable had before the
		// round that pinned it down.
		l.forgetResults()
		l.promoteWanted = nil
		l.run()
		l.resolveTypes()
		l.settleFields()
		state := l.typeState()
		if state == prev {
			break
		}
		prev = state
	}

	// An accessor reached through a value of unsettled class has to become a
	// real method, and the rename happens here, once, on what the settled
	// types asked for rather than on an early guess.
	l.applyPromotions()

	// Pass 2 builds the real tree.
	l.collectDestroys()
	l.pass = 2
	l.scope = newScope(nil)
	l.seps = defaultSeparators()
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
	// The predicates are declarations, so the pass that builds the real tree
	// writes them afresh rather than inheriting the discovery pass's copies.
	l.isaFuncs = nil
	l.isaDecls = nil
	l.classNameFunc = ""
	l.recordLookups = nil
	l.recordDecls = nil
	l.recordUsed = nil
	out := l.run()

	l.finishReport()
	return out
}

// maxDiscoveryRounds caps the type-discovery loop. Each round can only make a
// type more specific, so the loop terminates on its own; the cap is there so
// that a program the converter models badly costs bounded time rather than
// spinning.
const maxDiscoveryRounds = 6

// forgetResults drops the return shapes gathered by the previous round.
func (l *Lowerer) forgetResults() {
	for _, name := range l.subOrd {
		l.subs[name].ResultEvidence = nil
		l.subs[name].TailSpill = false
	}
	for _, s := range l.anonOrd {
		s.ResultEvidence = nil
		s.TailSpill = false
	}
}

// typeState renders every type the discovery pass settled, so that two rounds
// can be compared for whether anything moved.
func (l *Lowerer) typeState() string {
	var sb strings.Builder
	names := make([]string, 0, len(l.decls)+len(l.globals))
	for _, b := range l.decls {
		names = append(names, b.Go+"\x00"+b.Type.String())
	}
	for _, b := range l.globals {
		names = append(names, b.Go+"\x00"+b.Type.String())
	}
	for _, name := range l.classOrd {
		for _, f := range l.classes[name].Fields {
			names = append(names, name+"."+f.Go+"\x00"+f.Type.String())
		}
	}
	for _, name := range l.subOrd {
		s := l.subs[name]
		for _, r := range s.Results {
			names = append(names, name+"\x00"+r.String())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		sb.WriteString(n)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// setFile points the source-quoting machinery at one of the files taking part,
// so that a diagnostic raised while lowering it names that file and quotes
// from it rather than from the script.
func (l *Lowerer) setFile(f *SourceFile) {
	if f == nil || f == l.curFile {
		return
	}
	l.curFile = f
	l.src = string(f.Src)
	l.lines = f.lines
}

// run performs one whole pass over the program.
func (l *Lowerer) run() *Result {
	file := &ir.File{Name: "main.go", Package: "main"}

	var top []ir.Stmt
	for _, u := range l.units {
		l.curPkg = u.pkg
		l.setFile(u.file)
		if sd, ok := u.st.(*ast.SubDecl); ok {
			l.lowerSubDecl(sd)
			continue
		}
		// `our @ISA = ...` says what a class inherits from, which the type
		// declaration has already recorded. There is nothing left to run.
		if es, ok := u.st.(*ast.ExprStmt); ok {
			if _, isISA := parentFromISA(es.X); isISA && u.pkg != "main" {
				l.reportMultipleInheritance(l.classes[u.pkg], es)
				continue
			}
		}
		top = append(top, l.stmts([]ast.Stmt{u.st})...)
	}
	l.curPkg = "main"
	l.setFile(l.mainFile)

	// A type and the methods on it belong together, the way a Go file is
	// laid out, and both come before the code that uses them.
	emitted := map[*Sub]bool{}
	for _, c := range l.interfaceOrd {
		file.Decls = append(file.Decls, l.interfaceDecl(c))
	}
	file.Decls = append(file.Decls, l.isaDecls...)
	file.Decls = append(file.Decls, l.recordDecls...)
	for _, name := range l.classOrd {
		c := l.classes[name]
		if !c.IsType {
			continue
		}
		if c.Record && !l.recordUsed[c] {
			// The shape was considered and something else was chosen for it,
			// so there is no value of this type and nothing to declare.
			continue
		}
		file.Decls = append(file.Decls, l.classDecl(c))
		file.Decls = append(file.Decls, l.virtualDecls(c)...)
		for _, s := range c.Subs {
			emitted[s] = true
			if s.Inherited != nil {
				file.Decls = append(file.Decls, l.inheritedCtorDecl(s))
				continue
			}
			if s.irDecl != nil {
				file.Decls = append(file.Decls, s.irDecl)
			}
		}
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
		s := l.subs[name]
		if emitted[s] {
			continue
		}
		if fn := s.irDecl; fn != nil {
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
	if l.curFile != nil && l.curFile != l.mainFile {
		e.File = l.curFile.Path
	}
	key := e.File + ":" + e.Code + ":" + itoa(e.Line) + ":" + itoa(e.Col) + ":" + e.Construct
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
	if n.Pos().Offset < l.curStmt.Pos().Offset || n.Pos().Line < l.curStmt.Pos().Line {
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
	// The blank identifier is not a name and can be reused freely; numbering
	// it would produce a variable nothing reads.
	if base == "_" || l.tmpNames == nil {
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
		// The verdict comes from the type the binding finished with, not from
		// the flag inference set while it was working. A binding can be marked
		// dynamic early and be pinned down later, by a use further down the
		// file or by the shape of what it is finally assigned, and reporting
		// the flag would call a variable the generated code declares as
		// []string a variable with no type.
		dynamic := isDynamic(scalarPart(b.Type))
		reason := b.Reason
		if !dynamic {
			reason = ""
		}
		l.rep.Stats.Symbols++
		if !dynamic {
			l.rep.Stats.SymbolsTyped++
		}
		l.rep.Symbols = append(l.rep.Symbols, report.Symbol{
			Name:     b.Perl,
			Type:     b.Type.String(),
			Inferred: !dynamic,
			Reason:   reason,
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
		// A hash an option block fills in is the struct the block declared,
		// whatever else the file did with it.
		if b.Sigil == '%' {
			if c, ok := l.optionHash[b.Perl]; ok {
				b.Type = c.Value
				return
			}
		}
		// An option specification says what its destination holds, which is
		// a declaration rather than an observation.
		if t, ok := l.optionDests[b]; ok {
			b.Type = t
			return
		}
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
			// A hash whose keys are all written out and whose values differ
			// in kind is a record, and the struct synthesised for it is what
			// the variable holds. Everything else is a map.
			if t.Kind != ir.Map && l.classOf(t) == nil {
				t = ir.MapOf(t)
			}
		}
		// A container the file put undef into needs room in its element type
		// for "nothing here", and a Go scalar has none: nil in a
		// map[string]*int is a state that 0 in a map[string]int cannot
		// express. Only the settled element type can be wrapped, because the
		// wrapping has to happen once, after every observation is in.
		if b.NilElems && (t.Kind == ir.Slice || t.Kind == ir.Map) && isScalarKind(t.Elem) {
			if t.Kind == ir.Slice {
				t = ir.SliceOf(nullable(t.Elem))
			} else {
				t = ir.MapOf(nullable(t.Elem))
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
	l.unifyOverrides()
	for _, b := range l.decls {
		once(b)
	}
	for _, b := range l.globals {
		once(b)
	}
	for _, name := range l.subOrd {
		fillResults(l.subs[name])
	}
	// A record stored where the settled type has room only for `any` has
	// readers the struct rewrite cannot reach: they will assert the value
	// back to a map, find a struct, and quietly get nothing. A rewrite that
	// changes a value's representation must reach every reader or not fire,
	// so the class is marked and the next sweep builds those literals as the
	// maps their readers expect.
	for _, b := range l.decls {
		l.escapeLostRecords(b)
	}
	for _, b := range l.globals {
		l.escapeLostRecords(b)
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
