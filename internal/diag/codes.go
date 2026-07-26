// Package diag is the registry of everything perl2go can say about one place in
// a Perl file, and the renderer that says it.
//
// A limitation is a product feature with a name, a code, and an address. Every
// construct the tool cannot convert faithfully has a stable code here, a message
// that states what is true about the code, advice that names a real Go API, and
// a teaching concept to read afterwards. "Unsupported" on its own is a bug in
// this tool.
//
// The registry in this file is the only place a diagnostic code is written down.
// Nothing else in the tool builds a [Code] from a string literal. Codes are
// never reused and never renumbered: a code identifies a situation, not a
// sentence, so the wording may improve at any time while the code, the meaning,
// and the severity stay put.
//
// Code ranges, by the subsystem the fact comes from:
//
//	P2G0001-P2G0099  tool level: usage, filesystem, internal errors
//	P2G1000-P2G1499  lexing: quotes, heredocs, sigils, slash disambiguation
//	P2G1500-P2G1999  parsing: grammar, prototypes, statement recovery
//	P2G2000-P2G2499  scope, symbols, context, local
//	P2G2500-P2G2999  references, aliasing, autovivification, mutation
//	P2G3000-P2G3499  type inference, including every dynamic fallback
//	P2G3500-P2G3999  IR lowering and control-flow restructuring
//	P2G4000-P2G4499  regex features RE2 lacks
//	P2G4500-P2G4999  regex and string-splitting semantics: /g, pos, split, tr
//	P2G5000-P2G5499  strings, sprintf, encoding, character versus byte
//	P2G5500-P2G5999  numbers, sort, comparison, builtin list functions
//	P2G6000-P2G6499  file I/O, filehandles, buffering, file tests
//	P2G6500-P2G6999  processes, signals, environment, exit status
//	P2G7000-P2G7499  OO: bless, @ISA, SUPER::, method resolution
//	P2G7500-P2G7999  modules and the CPAN mapping table
//	P2G8000-P2G8499  dynamic Perl: eval STRING, globs, tie, AUTOLOAD, format
//	P2G8500-P2G8999  verification of the tool's own output
//	P2G9000-P2G9499  AI mode
//	P2G9500-P2G9999  REPL and interactive sessions
//
// The last two digits carry no meaning; gaps are left on purpose.
package diag

import "perl2go/internal/report"

// Code is a stable diagnostic code. It always matches /^P2G[0-9]{4}$/.
type Code string

// Entry is one row of the registry: everything the tool knows about a situation
// before it meets a particular source file.
//
// Message is a template for [fmt.Sprintf], so a literal percent sign in it is
// written `%%`. The other strings are never formatted, so a percent sign in them
// is written once.
type Entry struct {
	// Severity decides the sigil, the colour, and whether --strict fails.
	Severity report.Severity

	// Message is the full one-liner: what is true about the code, before why it
	// is true. It names the construct the way Perl spells it, in backticks, and
	// ends without a period.
	Message string

	// Short is the summary-column form, at most 44 bytes.
	Short string

	// Advice is the `try:` footer. It is imperative, and it names a real API
	// with its package or describes the shape of the rewrite in one clause.
	Advice string

	// Cost is the `cost:` footer: what the approximation gives up. Empty when
	// nothing was given up.
	Cost string

	// Converted is the `converted:` footer, or the `not converted:` footer when
	// Severity is [report.Refuse]. It states what the tool did, which is a
	// different fact from what the message states about the code.
	Converted string

	// Concepts are teaching concept ids, each of which is a file in the
	// knowledge base. They are the `learn:` footer.
	Concepts []string
}

// Tool level: usage, filesystem, and the tool's own bugs.
const (
	// FlagConflict: two command-line flags contradict each other.
	FlagConflict Code = "P2G0001"
	// InputUnreadable: the input file cannot be opened.
	InputUnreadable Code = "P2G0002"
	// OutputDirNotEmpty: the output directory exists and holds files.
	OutputDirNotEmpty Code = "P2G0003"
	// InputNotUTF8: the input is not valid UTF-8.
	InputNotUTF8 Code = "P2G0004"
	// InternalPanic: perl2go panicked while converting.
	InternalPanic Code = "P2G0010"
	// UnknownCode: a diagnostic was raised with a code that is not registered.
	UnknownCode Code = "P2G0011"
	// PerlAssistRunsPerl: --perl-assist executes the input under perl.
	PerlAssistRunsPerl Code = "P2G0020"
	// VerifyRunsBoth: --verify-behaviour runs the Perl and the Go.
	VerifyRunsBoth Code = "P2G0021"
	// StrictFailed: --strict was given and the run produced warnings.
	StrictFailed Code = "P2G0030"
)

// Lexing.
const (
	// UnterminatedString: a quoted string has no closing delimiter.
	UnterminatedString Code = "P2G1002"
	// UnterminatedHeredoc: a heredoc has no terminator line.
	UnterminatedHeredoc Code = "P2G1007"
	// UnbalancedDelimiters: a quote-like block has unbalanced delimiters.
	UnbalancedDelimiters Code = "P2G1009"
	// SlashReadAsRegex: a slash after a closing brace was read as a pattern.
	SlashReadAsRegex Code = "P2G1013"
	// DataSection: __DATA__ was read as embedded data.
	DataSection Code = "P2G1019"
	// SourceFilter: a source filter rewrites the file before perl sees it.
	SourceFilter Code = "P2G1021"
)

// Parsing.
const (
	// UnexpectedToken: the parser found a token no grammar rule accepts.
	UnexpectedToken Code = "P2G1502"
	// UnexpectedFatComma: `=>` appeared where an expression was expected.
	UnexpectedFatComma Code = "P2G1505"
	// Prototype: a sub prototype changes how calls to it parse.
	Prototype Code = "P2G1509"
	// StatementNotParsed: one statement did not parse and became a stub.
	StatementNotParsed Code = "P2G1514"
	// SubNotParsed: one sub did not parse and was skipped.
	SubNotParsed Code = "P2G1520"
	// FeatureSignatures: `use feature 'signatures'` changes sub header parsing.
	FeatureSignatures Code = "P2G1530"
)

// Scope, context, and dynamic scoping.
const (
	// LocalDynamicScope: `local` retargets a package variable dynamically.
	LocalDynamicScope Code = "P2G2001"
	// LocalRecordSeparator: `local $/` changes reading for the whole call tree.
	LocalRecordSeparator Code = "P2G2004"
	// ArgAliasing: writing to `@_` writes through to the caller.
	ArgAliasing Code = "P2G2020"
	// Wantarray: a sub inspects its calling context.
	Wantarray Code = "P2G2031"
	// ForeachAliasing: the foreach variable aliases the element.
	ForeachAliasing Code = "P2G2040"
)

// References, aliasing, and mutation.
const (
	// Autovivification: a nested access creates the intermediate containers.
	Autovivification Code = "P2G2510"
	// MutateWhileIterating: a collection is modified inside its own loop.
	MutateWhileIterating Code = "P2G2544"
)

// Type inference, including every dynamic fallback.
const (
	// DynamicScalar: a scalar's type could not be inferred.
	DynamicScalar Code = "P2G3001"
	// MixedHash: a hash holds values of more than one type.
	MixedHash Code = "P2G3010"
	// NumericAndString: a scalar is used as both a number and a string.
	NumericAndString Code = "P2G3016"
	// IntegerOverflow: integers past 2^63 behave differently.
	IntegerOverflow Code = "P2G3020"
	// UndefBecameZeroValue: undef became a zero value plus an ok boolean.
	UndefBecameZeroValue Code = "P2G3025"
	// NilDereference: a pointer that can be nil is dereferenced.
	NilDereference Code = "P2G3030"
	// InferredInt: a scalar was inferred as int from its arithmetic use.
	InferredInt Code = "P2G3040"
	// ContextDependentReturn: a sub returns a different shape per path.
	ContextDependentReturn Code = "P2G3050"
	// MixedArray: an array holds elements of more than one type.
	MixedArray Code = "P2G3060"
)

// IR lowering and control-flow restructuring.
const (
	// Redo: `redo` restarts a loop body without the loop's increment.
	Redo Code = "P2G3510"
	// GotoLabel: `goto LABEL` moves control in a way Go's goto cannot.
	GotoLabel Code = "P2G3520"
)

// Regex features RE2 lacks.
const (
	// RegexBackreference: a pattern refers back to an earlier capture.
	RegexBackreference Code = "P2G4001"
	// RegexLookahead: a pattern uses a lookahead assertion.
	RegexLookahead Code = "P2G4004"
	// RegexLookbehind: a pattern uses a lookbehind assertion.
	RegexLookbehind Code = "P2G4005"
	// RegexRecursion: a pattern calls itself.
	RegexRecursion Code = "P2G4010"
	// RegexEmbeddedCode: a pattern runs Perl while matching.
	RegexEmbeddedCode Code = "P2G4011"
	// RegexKeepOut: a pattern uses `\K`.
	RegexKeepOut Code = "P2G4014"
	// RegexFreeSpacing: `/x` was expanded at conversion time.
	RegexFreeSpacing Code = "P2G4030"
	// SubstEval: `s///e` evaluates its replacement as Perl.
	SubstEval Code = "P2G4040"
	// SubstDoubleEval: `s///ee` evaluates the replacement's result again.
	SubstDoubleEval Code = "P2G4041"
	// RuntimePattern: the pattern text is only known at run time.
	RuntimePattern Code = "P2G4080"
)

// Regex and string-splitting semantics.
const (
	// SplitSingleSpace: `split ' '` is not a split on one space.
	SplitSingleSpace Code = "P2G4510"
	// TrModifiers: `tr///` carries modifiers that change the rule.
	TrModifiers Code = "P2G4520"
	// MatchPosition: `/g` and `pos` keep match state on the scalar.
	MatchPosition Code = "P2G4530"
)

// Strings, sprintf, and encoding.
const (
	// LengthCountsRunes: `length` counts characters and Go counts bytes.
	LengthCountsRunes Code = "P2G5010"
	// SubstrReplacement: four-argument `substr` writes in place.
	SubstrReplacement Code = "P2G5012"
	// Chomp: `chomp` removes the current record separator.
	Chomp Code = "P2G5020"
	// MagicIncrement: `++` on a string steps through the Perl sequence.
	MagicIncrement Code = "P2G5030"
	// SprintfFormat: a sprintf format has no fmt verb with the same meaning.
	SprintfFormat Code = "P2G5040"
)

// Numbers, sort, and comparison.
const (
	// FloatFormatting: Perl and Go print floats to different precision.
	FloatFormatting Code = "P2G5501"
	// ModuloSign: `%` takes its sign from a different operand in each language.
	ModuloSign Code = "P2G5520"
	// DefaultSortIsStringwise: `sort` with no comparator sorts as strings.
	DefaultSortIsStringwise Code = "P2G5540"
	// SortStability: the comparator ties, so stability is observable.
	SortStability Code = "P2G5545"
	// HashOrder: hash iteration order is randomised, differently in each.
	HashOrder Code = "P2G5550"
)

// File I/O.
const (
	// TwoArgOpen: `open` takes its mode from the front of the filename.
	TwoArgOpen Code = "P2G6001"
	// ReadLineLoop: `while (<FH>)` became a bufio.Scanner.
	ReadLineLoop Code = "P2G6010"
	// SlurpFile: `$/ = undef` reads the whole file at once.
	SlurpFile Code = "P2G6012"
	// AutoflushNoOp: `$| = 1` has nothing to disable in the emitted code.
	AutoflushNoOp Code = "P2G6020"
	// FileTest: a `-f` style file test became os.Stat.
	FileTest Code = "P2G6030"
	// WriteErrorChecked: print errors are silent in Perl and checked in Go.
	WriteErrorChecked Code = "P2G6040"
	// EncodingLayer: an `:encoding` layer has no Go counterpart.
	EncodingLayer Code = "P2G6060"
)

// Processes, signals, and the environment.
const (
	// Backticks: backticks capture the output of a shell command.
	Backticks Code = "P2G6501"
	// SystemCall: `system` runs a command through a shell.
	SystemCall Code = "P2G6505"
	// ExitStatusShift: `$? >> 8` decodes a wait status.
	ExitStatusShift Code = "P2G6510"
	// Fork: `fork` copies the process.
	Fork Code = "P2G6520"
	// SignalHandler: a %SIG handler became a channel.
	SignalHandler Code = "P2G6530"
	// EnvForChildren: %ENV writes reach children in Perl only.
	EnvForChildren Code = "P2G6540"
)

// Object orientation.
const (
	// Bless: `bless` became a struct with methods.
	Bless Code = "P2G7001"
	// MultipleInheritance: @ISA lists more than one parent.
	MultipleInheritance Code = "P2G7005"
	// SuperCall: `SUPER::` resolves to the embedded parent.
	SuperCall Code = "P2G7010"
	// DynamicMethodName: the method name is computed at run time.
	DynamicMethodName Code = "P2G7015"
	// CanCheck: `->can` asks at run time what Go answers at compile time.
	CanCheck Code = "P2G7020"
	// Destroy: `DESTROY` runs at scope exit and Go collects later.
	Destroy Code = "P2G7030"
)

// Modules and the CPAN mapping table.
const (
	// GetoptPermutation: Getopt::Long permutes and flag does not.
	GetoptPermutation Code = "P2G7505"
	// JSONNumbers: JSON numbers decode as float64 through any.
	JSONNumbers Code = "P2G7510"
	// DumperEval: Data::Dumper output is read back with eval.
	DumperEval Code = "P2G7515"
	// HTTPTimeout: the HTTP client default timeout differs.
	HTTPTimeout Code = "P2G7525"
	// SQLPlaceholders: placeholder syntax is decided by the driver.
	SQLPlaceholders Code = "P2G7530"
	// ModuleUnmapped: a module has no entry in the mapping table.
	ModuleUnmapped Code = "P2G7550"
	// StrictWarnings: `use strict` and `use warnings` have no counterpart.
	StrictWarnings Code = "P2G7555"
	// PosixFloor: POSIX::floor maps to math.Floor.
	PosixFloor Code = "P2G7560"
	// Base64Wrapping: base64 line wrapping differs.
	Base64Wrapping Code = "P2G7570"
	// YAMLDependency: YAML needs a third-party package.
	YAMLDependency Code = "P2G7580"
	// StorableFormat: Storable files have no Go reader.
	StorableFormat Code = "P2G7585"
	// WalkOrder: directory walk order differs.
	WalkOrder Code = "P2G7590"
)

// Dynamic Perl.
const (
	// EvalString: `eval STRING` compiles Perl at run time.
	EvalString Code = "P2G8001"
	// SymbolicRefMapped: a symbolic reference became a dispatch map lookup.
	SymbolicRefMapped Code = "P2G8010"
	// SymbolicRefOpen: a symbolic reference names something unenumerable.
	SymbolicRefOpen Code = "P2G8011"
	// GlobAssignment: a typeglob assignment redirects a handle.
	GlobAssignment Code = "P2G8020"
	// GlobComputed: the typeglob target is built at run time.
	GlobComputed Code = "P2G8021"
	// AutoloadExpanded: AUTOLOAD was expanded into the methods this file calls.
	AutoloadExpanded Code = "P2G8030"
	// AutoloadOpen: AUTOLOAD answers names that are not visible here.
	AutoloadOpen Code = "P2G8031"
	// TieConverted: a tied variable became explicit method calls.
	TieConverted Code = "P2G8040"
	// TieEscapes: a tied variable is passed where the tie is invisible.
	TieEscapes Code = "P2G8041"
	// OperatorOverload: an overloaded operator was resolved per call site.
	OperatorOverload Code = "P2G8050"
	// MonkeyPatch: a sub is replaced at run time.
	MonkeyPatch Code = "P2G8060"
	// FormatWrite: `format` and `write` became a tabwriter function.
	FormatWrite Code = "P2G8070"
	// GotoSub: `goto &$sub` replaces the current frame.
	GotoSub Code = "P2G8080"
	// GotoOverDeclaration: a goto jumps over a declaration.
	GotoOverDeclaration Code = "P2G8085"
	// BeginDecidesParse: a BEGIN block decides how the rest of the file parses.
	BeginDecidesParse Code = "P2G8090"
	// BeginEndBlocks: BEGIN and END became init and defer.
	BeginEndBlocks Code = "P2G8095"
	// InlineC: Inline::C embeds C source.
	InlineC Code = "P2G8100"
)

// Verification of the tool's own output.
const (
	// OutputDoesNotParse: the generated Go does not parse.
	OutputDoesNotParse Code = "P2G8501"
	// OutputDoesNotCompile: the generated Go does not compile.
	OutputDoesNotCompile Code = "P2G8505"
	// OutputVetFinding: go vet reports something about the generated Go.
	OutputVetFinding Code = "P2G8510"
	// OutputNotFormatted: the generated Go could not be formatted.
	OutputNotFormatted Code = "P2G8515"
	// CleanOutputMentionsPerl: the clean output leaked conversion vocabulary.
	CleanOutputMentionsPerl Code = "P2G8520"
	// NoToolchain: no Go toolchain was available to build the output.
	NoToolchain Code = "P2G8530"
)

// AI mode.
const (
	// AIRuntimeUnreachable: no inference runtime answered.
	AIRuntimeUnreachable Code = "P2G9001"
	// AIRewriteRejectedBuild: the model's rewrite did not compile.
	AIRewriteRejectedBuild Code = "P2G9005"
	// AIRewriteRejectedBehaviour: the model's rewrite changed behaviour.
	AIRewriteRejectedBehaviour Code = "P2G9010"
	// AIModelTooLarge: the model does not fit in the free VRAM.
	AIModelTooLarge Code = "P2G9020"
	// AIPartialImprovement: AI mode improved some files and not others.
	AIPartialImprovement Code = "P2G9030"
)

// REPL.
const (
	// ReplIncomplete: the snippet is not a complete statement yet.
	ReplIncomplete Code = "P2G9501"
	// ReplParseError: the snippet did not parse.
	ReplParseError Code = "P2G9505"
)

// catalogue is the registry. Nothing outside this file writes a code literal.
var catalogue = map[Code]Entry{
	// -- Tool level ---------------------------------------------------------

	FlagConflict: {
		Severity: report.Refuse,
		Message:  "`%s` and `%s` cannot both be given",
		Short:    "conflicting flags",
		Advice:   "pass one of the two; `perl2go convert --help` says what each one does",
	},
	InputUnreadable: {
		Severity: report.Refuse,
		Message:  "input file `%s` cannot be read: %s",
		Short:    "input file cannot be read",
		Advice:   "check the path, or pass `-` to read Perl from standard input",
	},
	OutputDirNotEmpty: {
		Severity: report.Refuse,
		Message:  "output directory `%s` already exists and is not empty",
		Short:    "output directory is not empty",
		Advice:   "pass `-o` with a different directory, or `--force` to overwrite it",
	},
	InputNotUTF8: {
		Severity: report.Refuse,
		Message:  "input file `%s` is not valid UTF-8 at byte %d",
		Short:    "input is not valid UTF-8",
		Advice:   "re-encode with `iconv -f latin1 -t utf8`, or check for binary content after `__DATA__`",
		Concepts: []string{"strings-are-bytes"},
	},
	InternalPanic: {
		Severity: report.Refuse,
		Message:  "perl2go panicked while converting `%s`, which is always a bug in perl2go",
		Short:    "internal panic while converting",
		Advice:   "report this with the input file; nothing was written to disk",
	},
	UnknownCode: {
		Severity: report.Refuse,
		Message:  "diagnostic code `%s` is not in the registry, which is always a bug in perl2go",
		Short:    "unknown diagnostic code",
		Advice:   "report this with the input file; `perl2go explain` lists every code that exists",
	},
	PerlAssistRunsPerl: {
		Severity:  report.Warn,
		Message:   "`--perl-assist` runs `perl -MO=Deparse` on `%s`, which executes its `BEGIN` blocks",
		Short:     "--perl-assist executes the input",
		Advice:    "pass this flag only for code you trust; drop it to convert statically",
		Cost:      "the input runs under a real perl, with whatever side effects its `BEGIN` and `use` statements have",
		Converted: "the deparsed output is used only to disambiguate what the static parse could not decide",
	},
	VerifyRunsBoth: {
		Severity: report.Warn,
		Message:  "`--verify-behaviour` runs `%s` under perl and then the generated Go, and compares the output",
		Short:    "--verify-behaviour runs both programs",
		Advice:   "pass this flag only for code you trust and inputs you can afford to run twice",
		Cost:     "both programs run for real, so anything the script writes or deletes happens twice",
	},
	StrictFailed: {
		Severity: report.Refuse,
		Message:  "`--strict` was given and this run produced %d warnings",
		Short:    "strict mode failed on warnings",
		Advice:   "resolve the constructs listed above, or drop `--strict` to take the output as it is",
	},

	// -- Lexing -------------------------------------------------------------

	UnterminatedString: {
		Severity: report.Refuse,
		Message:  "the string opened here has no closing `%s` before end of file",
		Short:    "unterminated quoted string",
		Advice:   "check for an unescaped delimiter inside the string, or a missing terminator",
	},
	UnterminatedHeredoc: {
		Severity: report.Refuse,
		Message:  "heredoc `%s` has no `%s` terminator line before end of file",
		Short:    "unterminated heredoc",
		Advice:   "the terminator must start its own line with no trailing whitespace, unless `<<~` was used",
	},
	UnbalancedDelimiters: {
		Severity: report.Refuse,
		Message:  "the `%s` block opened here has unbalanced `%s` delimiters",
		Short:    "unbalanced quote-like delimiters",
		Advice:   "balance the delimiters, or switch to a delimiter pair the body does not contain",
	},
	SlashReadAsRegex: {
		Severity: report.Note,
		Message:  "`/` after `%s` was read as the start of a regex, not as division",
		Short:    "slash read as a regex, not division",
		Advice:   "add parentheses around the left operand if division was meant",
	},
	DataSection: {
		Severity:  report.Note,
		Message:   "`__DATA__` section read as %d bytes of embedded data",
		Short:     "__DATA__ section embedded as a file",
		Advice:    "read it through the generated `//go:embed` variable, which needs the file to stay in the package",
		Converted: "the data is written next to the program and embedded with `//go:embed`",
	},
	SourceFilter: {
		Severity:  report.Refuse,
		Message:   "source filter `%s` rewrites this file before perl compiles it",
		Short:     "source filter rewrites the input",
		Advice:    "run the filter yourself and convert its output, or replace the filter with ordinary Perl first",
		Converted: "the file is not converted: the text on disk is not the text perl compiles",
		Concepts:  []string{"compile-time-mindset"},
	},

	// -- Parsing ------------------------------------------------------------

	UnexpectedToken: {
		Severity: report.Refuse,
		Message:  "unexpected `%s` while the `%s` block opened earlier is still open",
		Short:    "unexpected token inside an open block",
		Advice:   "check for a missing `}` inside the block body",
	},
	UnexpectedFatComma: {
		Severity: report.Refuse,
		Message:  "`=>` found where an expression was expected",
		Short:    "fat comma without a left operand",
		Advice:   "check for a stray comma, or a bareword key that needs quoting",
	},
	Prototype: {
		Severity:  report.Warn,
		Message:   "prototype `%s` on `%s` changes how calls to it parse",
		Short:     "prototype has no Go counterpart",
		Advice:    "check the call sites: Go parses `f(a, b)` the same way whatever `f` is",
		Cost:      "a call that relied on the prototype, such as `pairwise { ... } @a, @b`, is emitted with explicit parentheses",
		Converted: "the sub became an ordinary function and every call site was parenthesised",
		Concepts:  []string{"variadic-and-no-defaults"},
	},
	StatementNotParsed: {
		Severity:  report.Warn,
		Message:   "the statement here did not parse and was replaced by a panic stub",
		Short:     "statement did not parse",
		Advice:    "translate the quoted Perl in the stub by hand, then delete the panic",
		Cost:      "the program compiles, and reaching this statement stops it",
		Converted: "the surrounding sub converted; this one statement panics with the original Perl text",
		Concepts:  []string{"panic-and-recover"},
	},
	SubNotParsed: {
		Severity:  report.Warn,
		Message:   "sub `%s` did not parse and was skipped, and the rest of the file converted",
		Short:     "sub skipped: it did not parse",
		Advice:    "translate the sub by hand, or report the construct that stopped the parser",
		Converted: "every other sub in the file converted; calls to this one reach a stub that panics",
		Concepts:  []string{"panic-and-recover"},
	},
	FeatureSignatures: {
		Severity: report.Note,
		Message:  "`use feature 'signatures'` changes how sub headers parse, and both forms are handled",
		Short:    "signature and @_ forms both handled",
		Advice:   "nothing to do; signatures and `@_` unpacking both become ordinary Go parameters",
		Concepts: []string{"variadic-and-no-defaults"},
	},

	// -- Scope and context --------------------------------------------------

	LocalDynamicScope: {
		Severity:  report.Warn,
		Message:   "`local %s` gives every sub this call reaches a different value until the scope ends",
		Short:     "local retargets a global dynamically",
		Advice:    "pass the value as a parameter, or set it and restore it with `defer` around the call",
		Cost:      "the restore runs at function exit rather than at block exit, so a `local` inside a bare block lives longer",
		Converted: "the emitted code saves the old value, assigns the new one, and restores it with `defer`",
		Concepts:  []string{"defer-timing"},
	},
	LocalRecordSeparator: {
		Severity:  report.Warn,
		Message:   "`local $/` changes the record separator for every sub this call reaches",
		Short:     "local $/ changes reads elsewhere",
		Advice:    "take the separator as an explicit parameter; the call sites that changed are in the report",
		Cost:      "a sub that read `$/` without being called from here keeps the default separator",
		Converted: "the separator is passed explicitly to each sub that reads it",
		Concepts:  []string{"bufio-scanner-limit"},
	},
	ArgAliasing: {
		Severity:  report.Warn,
		Message:   "assigning to `%s` writes through to the caller's variable, and the Go parameter is a copy",
		Short:     "@_ aliasing does not survive",
		Advice:    "take a pointer parameter, or return the new value and assign it at the call site",
		Cost:      "the converted sub mutates its own copy, so the caller's variable keeps its old value",
		Converted: "the sub took the argument by value; the write is local to it",
		Concepts:  []string{"pointers-vs-references"},
	},
	Wantarray: {
		Severity:  report.Warn,
		Message:   "`wantarray` makes `%s` return a different shape in list and scalar context",
		Short:     "wantarray split into two functions",
		Advice:    "call the list form or the count form directly; Go has no calling context to inspect",
		Cost:      "a caller that chose the shape at run time has to choose it at the call site instead",
		Converted: "the sub became two functions, one returning the slice and one returning the count",
		Concepts:  []string{"multiple-return-values"},
	},
	ForeachAliasing: {
		Severity:  report.Note,
		Message:   "the `foreach` variable `%s` aliases each element, and the Go loop variable is a copy",
		Short:     "foreach aliasing became indexing",
		Advice:    "write through the index, `rows[i].Field = x`, wherever the loop body has to mutate the element",
		Converted: "the emitted loop indexes the slice where the body assigns to the loop variable",
		Concepts:  []string{"range-is-not-foreach"},
	},

	// -- References and mutation --------------------------------------------

	Autovivification: {
		Severity:  report.Note,
		Message:   "`%s` autovivifies every level it touches, and Go maps do not",
		Short:     "autovivification made explicit",
		Advice:    "create the inner map before writing to it; `m[a][b]++` on a missing `m[a]` panics",
		Converted: "the emitted code creates each intermediate map before the write",
		Concepts:  []string{"nil-slices-vs-nil-maps", "maps-of-slices"},
	},
	MutateWhileIterating: {
		Severity:  report.Warn,
		Message:   "`%s` is modified inside the `foreach` that iterates it",
		Short:     "collection mutated while iterated",
		Advice:    "keep the index loop, or copy the collection before iterating if the growth is unbounded",
		Cost:      "the loop can run longer than the collection was when it started, exactly as the Perl does",
		Converted: "the emitted loop uses an index, so elements appended during the loop are visited too",
		Concepts:  []string{"range-is-not-foreach", "slice-aliasing-and-copy"},
	},

	// -- Type inference -----------------------------------------------------

	DynamicScalar: {
		Severity:  report.Warn,
		Message:   "the type of `%s` could not be inferred, so it stays a dynamic `perlval.Value`",
		Short:     "type stayed dynamic",
		Advice:    "give the variable one consistent type, or annotate the sub's parameters, and convert again",
		Cost:      "every use goes through run-time dispatch, so the compiler cannot check it and a wrong use is a panic",
		Converted: "the variable and everything it flows into carry `perlval.Value`; the conflicting uses are in the report",
		Concepts:  []string{"static-types-and-zero-values", "type-assertions-and-switches"},
	},
	MixedHash: {
		Severity: report.Warn,
		Message:  "`%s` holds values of more than one type, so it became `map[string]any`",
		Short:    "mixed-type hash became map[string]any",
		Advice:   "declare a struct with typed fields; the shape the tool inferred is in the report",
		Cost:     "each read needs a type assertion, and a wrong key is a run-time surprise rather than a compile error",
		Concepts: []string{"type-assertions-and-switches", "structs-and-embedding"},
	},
	NumericAndString: {
		Severity: report.Warn,
		Message:  "`%s` is used as both a number and a string, and it was converted as `float64`",
		Short:    "numeric and string uses merged",
		Advice:   "call `strconv.FormatFloat` where the string form matters, and pick the precision the output needs",
		Cost:     "the string form goes through `strconv.FormatFloat`, which differs from Perl above 15 significant digits",
		Concepts: []string{"strconv-parsing", "explicit-conversions-no-coercion"},
	},
	IntegerOverflow: {
		Severity: report.Warn,
		Message:  "integers past 2^63 promote to floats in Perl and wrap silently in Go",
		Short:    "integer overflow wraps in Go",
		Advice:   "use `math/big.Int` if the values can reach that range, or check the operands before multiplying",
		Cost:     "a product that Perl prints as 1.84467440737096e+19 comes out negative",
		Concepts: []string{"explicit-conversions-no-coercion"},
	},
	UndefBecameZeroValue: {
		Severity:  report.Note,
		Message:   "`undef` for `%s` became the zero value plus a separate `ok` boolean",
		Short:     "undef became a value plus an ok flag",
		Advice:    "test the `ok` boolean rather than comparing against the zero value",
		Converted: "`defined` became the boolean, so an empty string stays distinguishable from undef",
		Concepts:  []string{"nil-vs-undef", "comma-ok-idiom"},
	},
	NilDereference: {
		Severity:  report.Warn,
		Message:   "`%s` can be `undef` and is typed as a pointer, so a dereference of it panics",
		Short:     "nil dereference where Perl printed nothing",
		Advice:    "check for nil before every use; Perl printed an empty string where Go stops the program",
		Cost:      "an input that reached the undef path printed nothing in Perl and panics in Go",
		Converted: "the emitted code checks for nil at the uses the tool could see",
		Concepts:  []string{"nil-vs-undef", "panic-and-recover"},
	},
	InferredInt: {
		Severity: report.Note,
		Message:  "`%s` was inferred as `int` from `++` and `+=`",
		Short:    "inferred int from arithmetic use",
		Advice:   "nothing to do; assigning a string to it will not compile, which is the point",
		Concepts: []string{"static-types-and-zero-values"},
	},
	ContextDependentReturn: {
		Severity:  report.Warn,
		Message:   "`%s` returns a scalar on one path and a list on another",
		Short:     "sub returns two different shapes",
		Advice:    "pick one return shape and change the call sites, which is the change Go's signature forces",
		Converted: "the emitted signature returns `([]string, bool)`, and the scalar path returns a one-element slice",
		Concepts:  []string{"multiple-return-values"},
	},
	MixedArray: {
		Severity: report.Warn,
		Message:  "the array `%s` holds more than one element type",
		Short:    "mixed-type array became []any",
		Advice:   "split it into one variable per element type, and both will type cleanly",
		Cost:     "each read needs a type assertion, and the compiler cannot tell the two kinds apart",
		Concepts: []string{"slices-not-arrays", "type-assertions-and-switches"},
	},

	// -- IR lowering --------------------------------------------------------

	Redo: {
		Severity:  report.Warn,
		Message:   "`redo` restarts the loop body without re-running the loop's increment",
		Short:     "redo became an inner loop",
		Advice:    "read the emitted inner `for`; a guard variable is clearer when the redo happens rarely",
		Cost:      "a `redo` that never stops is an inner loop that never stops, exactly as in Perl",
		Converted: "the body is wrapped in an inner `for` that repeats until the body runs to its end",
		Concepts:  []string{"range-is-not-foreach"},
	},
	GotoLabel: {
		Severity:  report.Refuse,
		Message:   "`goto %s` moves control in a way Go's `goto` cannot express",
		Short:     "goto has no Go equivalent here",
		Advice:    "a labelled `break` or `continue` covers jumps out of loops; anything else wants a small state machine",
		Converted: "the jump is not converted: Go's `goto` cannot enter a block or jump over a declaration",
		Concepts:  []string{"var-vs-short-declaration"},
	},

	// -- Regex features RE2 lacks -------------------------------------------

	RegexBackreference: {
		Severity:  report.Warn,
		Message:   "backreference `%s` is not available in Go's `regexp` package",
		Short:     "backreference needs regexp2",
		Advice:    "capture once and compare the two parts in Go, which lets the pattern go back to `regexp`",
		Cost:      "linear-time matching is no longer guaranteed, so a crafted input can make one match take a long time",
		Converted: "the pattern compiles with `github.com/dlclark/regexp2`, which backtracks",
		Concepts:  []string{"regexp-is-re2"},
	},
	RegexLookahead: {
		Severity:  report.Warn,
		Message:   "lookahead `%s` is not available in Go's `regexp` package",
		Short:     "lookahead needs regexp2",
		Advice:    "drop the lookahead and test the surrounding text with `strings.HasPrefix` or a second match",
		Cost:      "linear-time matching is no longer guaranteed; a 5s match timeout is set on the compiled pattern",
		Converted: "the pattern compiles with `github.com/dlclark/regexp2`, which backtracks",
		Concepts:  []string{"regexp-is-re2", "mustcompile-pattern"},
	},
	RegexLookbehind: {
		Severity:  report.Warn,
		Message:   "lookbehind `%s` is not available in Go's `regexp` package",
		Short:     "lookbehind needs regexp2",
		Advice:    "capture the preceding text in a group and use the capture instead of the assertion",
		Cost:      "linear-time matching is no longer guaranteed; a 5s match timeout is set on the compiled pattern",
		Converted: "the pattern compiles with `github.com/dlclark/regexp2`, which backtracks",
		Concepts:  []string{"regexp-is-re2", "submatch-and-named-groups"},
	},
	RegexRecursion: {
		Severity:  report.Refuse,
		Message:   "recursive pattern `%s` needs a backtracking engine, and neither Go regexp engine recurses",
		Short:     "recursive pattern has no engine",
		Advice:    "write a small scanner for the grammar this pattern matches; balanced nesting is not a regular language",
		Converted: "the pattern is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"regexp-is-re2"},
	},
	RegexEmbeddedCode: {
		Severity:  report.Refuse,
		Message:   "embedded code `(?{ ... })` runs Perl during matching, which no Go regexp engine does",
		Short:     "embedded code in a pattern",
		Advice:    "match first, then run the code over the captures the match returns",
		Converted: "the pattern is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"regexp-is-re2", "submatch-and-named-groups"},
	},
	RegexKeepOut: {
		Severity: report.Warn,
		Message:  "`\\K` is available in neither Go regexp engine",
		Short:    "\\K has no Go equivalent",
		Advice:   "capture the part before `\\K` in a group and rebuild the string from the captures",
		Cost:     "the replacement now rewrites the whole match, so the emitted code puts the prefix back by hand",
		Concepts: []string{"regexp-is-re2", "replace-and-expansion"},
	},
	RegexFreeSpacing: {
		Severity:  report.Note,
		Message:   "`/x` free-spacing was applied at conversion time, so the emitted pattern has no whitespace",
		Short:     "/x expanded at conversion time",
		Advice:    "keep the readable form in a comment above the pattern if it is worth reading",
		Converted: "the emitted pattern is the expanded one, with the comments and whitespace removed",
		Concepts:  []string{"mustcompile-pattern"},
	},
	SubstEval: {
		Severity: report.Warn,
		Message:  "the `/e` replacement in `s///e` became a Go function passed to `ReplaceAllStringFunc`",
		Short:    "s///e became a replacement func",
		Advice:   "check the function body: it receives the whole match, not the capture groups",
		Cost:     "capture groups are not passed in, so a replacement that used `$1` has to match again inside the function",
		Concepts: []string{"replace-and-expansion", "submatch-and-named-groups"},
	},
	SubstDoubleEval: {
		Severity:  report.Refuse,
		Message:   "`s///ee` evaluates the replacement as Perl source on every match",
		Short:     "s///ee evaluates Perl at run time",
		Advice:    "make the replacement a Go function and call it from `regexp.Regexp.ReplaceAllStringFunc`",
		Converted: "the substitution is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"compile-time-mindset", "replace-and-expansion"},
	},
	RuntimePattern: {
		Severity: report.Warn,
		Message:  "the pattern is built from `%s` at run time, so it compiles at run time too",
		Short:    "pattern compiled at run time",
		Advice:   "hoist the pattern to a package-level `regexp.MustCompile` wherever the text turns out to be constant",
		Cost:     "`MustCompile` cannot be used, so the emitted code handles a compile error as a value on every call",
		Concepts: []string{"mustcompile-pattern", "errors-are-values"},
	},

	// -- Regex and splitting semantics --------------------------------------

	SplitSingleSpace: {
		Severity:  report.Note,
		Message:   "`split ' '` splits on runs of whitespace and drops leading whitespace",
		Short:     "split with one space is not Split",
		Advice:    "use `strings.Fields`, which has exactly that behaviour, rather than `strings.Split`",
		Converted: "the emitted code uses `strings.Fields`",
		Concepts:  []string{"strings-are-bytes"},
	},
	TrModifiers: {
		Severity:  report.Warn,
		Message:   "`tr///` with the `%s` modifiers does more than map one character to another",
		Short:     "tr/// modifiers need a helper",
		Advice:    "read the emitted helper: `strings.Map` covers plain translation, and `c`, `d`, `s` and `r` each change the rule",
		Cost:      "the helper walks the string once per call, where Perl's `tr` uses a prebuilt table",
		Converted: "the emitted helper reproduces the modifiers and returns the count where the Perl used it",
		Concepts:  []string{"strings-are-bytes", "replace-and-expansion"},
	},
	MatchPosition: {
		Severity:  report.Note,
		Message:   "`pos` and the `/g` match state live on the scalar in Perl, and on the loop in Go",
		Short:     "/g match state became a loop index",
		Advice:    "iterate `FindAllStringSubmatchIndex`, whose returned offsets are the `pos` equivalent",
		Converted: "the emitted loop keeps the offset in a local variable instead of on the string",
		Concepts:  []string{"submatch-and-named-groups"},
	},

	// -- Strings ------------------------------------------------------------

	LengthCountsRunes: {
		Severity:  report.Note,
		Message:   "`length` counts characters in Perl under `use utf8` and `len` counts bytes in Go",
		Short:     "length counts bytes in Go",
		Advice:    "use `utf8.RuneCountInString` for characters and `len` for bytes, and pick the one this code meant",
		Converted: "the emitted code uses `utf8.RuneCountInString`, which matches the Perl",
		Concepts:  []string{"strings-are-bytes"},
	},
	SubstrReplacement: {
		Severity:  report.Warn,
		Message:   "four-argument `substr` replaces in place, and Go strings are immutable",
		Short:     "4-arg substr rebuilds the string",
		Advice:    "hold the value as `[]byte` and write into the slice where this runs on a hot path",
		Cost:      "the emitted code allocates a new string on every replacement",
		Converted: "the emitted code rebuilds the string from the part before and the part after",
		Concepts:  []string{"strings-are-bytes", "slice-aliasing-and-copy"},
	},
	Chomp: {
		Severity: report.Note,
		Message:  "`chomp` removes one trailing `$/`, which is a newline here",
		Short:    "chomp removes one separator",
		Advice:   "use `strings.TrimSuffix` with the separator, which removes one, not `strings.TrimRight`",
		Cost:     "differs from `strings.TrimRight` when a line ends in several newlines",
		Concepts: []string{"strings-are-bytes"},
	},
	MagicIncrement: {
		Severity:  report.Warn,
		Message:   "the magic string increment on `%s` turns `az` into `ba`, and Go has no operator for it",
		Short:     "magic string increment needs a helper",
		Advice:    "use a plain integer counter when the string form is only an identifier; the helper is the Perl sequence",
		Cost:      "the helper is a function call where Perl had an operator, and it panics on a string outside the pattern",
		Converted: "the emitted helper reproduces the sequence for strings matching `/^[a-zA-Z]*[0-9]*$/`",
		Concepts:  []string{"strings-are-bytes", "explicit-conversions-no-coercion"},
	},
	SprintfFormat: {
		Severity: report.Warn,
		Message:  "the `sprintf` format `%s` has no `fmt` verb with the same meaning",
		Short:    "sprintf format has no fmt verb",
		Advice:   "build the text with `strconv.Format*` and pad it with `fmt.Sprintf`, which has the width syntax",
		Cost:     "the emitted code approximates the field with the nearest verb, so the width or the rounding can differ",
		Concepts: []string{"strconv-parsing"},
	},

	// -- Numbers and sort ---------------------------------------------------

	FloatFormatting: {
		Severity:  report.Note,
		Message:   "Perl prints floats to 15 significant digits and Go prints the shortest round-trip form",
		Short:     "float printing differs from Perl",
		Advice:    "call `strconv.FormatFloat(f, 'g', 15, 64)` wherever the output has to match Perl byte for byte",
		Cost:      "`0.1+0.2` prints `0.3` in Perl and `0.30000000000000004` from Go's `%v`",
		Converted: "the emitted code formats floats with `%.15g`",
		Concepts:  []string{"strconv-parsing", "explicit-conversions-no-coercion"},
	},
	ModuloSign: {
		Severity:  report.Warn,
		Message:   "`%%` takes the sign of its right operand in Perl and of its left operand in Go",
		Short:     "modulo sign follows a different operand",
		Advice:    "normalise with `((a % b) + b) % b` when the right operand is positive, which is what the helper does",
		Cost:      "`-7 % 3` is `2` in Perl and `-1` in Go",
		Converted: "the emitted code calls a helper that reproduces Perl's sign rule",
		Concepts:  []string{"explicit-conversions-no-coercion"},
	},
	DefaultSortIsStringwise: {
		Severity:  report.Warn,
		Message:   "`sort` with no comparator sorts stringwise, and Go has no default order at all",
		Short:     "default sort is stringwise",
		Advice:    "sort `[]string` with `slices.Sort` for the Perl order, and pass a comparator when the data is numeric",
		Cost:      "`sort 9, 10` gives `10, 9` in Perl, and a numeric sort would give `9, 10`",
		Converted: "the emitted code sorts the string forms, which matches Perl's default",
		Concepts:  []string{"sort-slice", "explicit-conversions-no-coercion"},
	},
	SortStability: {
		Severity:  report.Warn,
		Message:   "the comparator returns 0 for entries that are not equal, and `slices.SortFunc` is not stable",
		Short:     "sort stability had to be chosen",
		Advice:    "keep `slices.SortStableFunc`, or add a tie-breaker to the comparator so the order is total",
		Converted: "the emitted code uses `slices.SortStableFunc`, which matches Perl's mergesort",
		Concepts:  []string{"sort-slice"},
	},
	HashOrder: {
		Severity:  report.Note,
		Message:   "hash iteration order is randomised in both languages, and differently",
		Short:     "hash order differs run to run",
		Advice:    "sort the keys before iterating wherever the output order matters",
		Converted: "the emitted code sorts the keys before iterating, so the output is reproducible",
		Concepts:  []string{"map-iteration-order", "sort-slice"},
	},

	// -- File I/O -----------------------------------------------------------

	TwoArgOpen: {
		Severity:  report.Warn,
		Message:   "two-argument `open` takes its mode from the front of `%s`",
		Short:     "2-arg open mode is in the filename",
		Advice:    "use `os.Open` to read, `os.Create` to write, and `os.OpenFile` with `O_APPEND` to append",
		Cost:      "a filename built at run time can no longer change the mode, which also closes an injection path",
		Converted: "the emitted code opens the file with the mode the string named, decided at conversion time",
		Concepts:  []string{"io-reader-writer", "errors-are-values"},
	},
	ReadLineLoop: {
		Severity: report.Note,
		Message:  "`while (<%s>)` became a `bufio.Scanner` with its buffer raised to 1 MiB",
		Short:    "line reads became bufio.Scanner",
		Advice:   "read with `bufio.Reader.ReadString` where a line can exceed 1 MiB; `Scanner` stops with `bufio.ErrTooLong`",
		Cost:     "a line longer than 1 MiB ends the loop with an error, where Perl read it",
		Concepts: []string{"bufio-scanner-limit", "sentinel-and-custom-errors"},
	},
	SlurpFile: {
		Severity: report.Warn,
		Message:  "`$/ = undef` slurps the whole file, and the emitted code uses `os.ReadFile`",
		Short:    "slurp became os.ReadFile",
		Advice:   "stream with `bufio.Reader` for files that do not fit in memory",
		Cost:     "the whole file is held in memory at once, as it was in Perl",
		Concepts: []string{"io-reader-writer", "bufio-scanner-limit"},
	},
	AutoflushNoOp: {
		Severity:  report.Warn,
		Message:   "`$| = 1` disables buffering, and the emitted code writes to `os.Stdout` unbuffered already",
		Short:     "$| had nothing to disable",
		Advice:    "defer a `Flush` if a `bufio.Writer` is added later, or the last lines go missing",
		Converted: "the assignment was dropped, because `os.Stdout` in Go is not buffered",
		Concepts:  []string{"io-reader-writer", "defer-timing"},
	},
	FileTest: {
		Severity:  report.Note,
		Message:   "the `%s` file test became `os.Stat` and a check on the mode",
		Short:     "file test became os.Stat",
		Advice:    "treat `os.IsNotExist` as the false case, which is what Perl's file tests return silently",
		Converted: "the emitted code calls `os.Stat` and reads `Mode()`",
		Concepts:  []string{"errors-are-values", "if-err-nil-rhythm"},
	},
	WriteErrorChecked: {
		Severity:  report.Warn,
		Message:   "`print` to a closed handle is silent in Perl, and the emitted code checks the error",
		Short:     "write errors are now checked",
		Advice:    "keep the check; drop it only where losing the output is acceptable",
		Converted: "every write returns an error that the emitted code handles",
		Concepts:  []string{"errors-are-values", "if-err-nil-rhythm"},
	},
	EncodingLayer: {
		Severity:  report.Note,
		Message:   "the `:encoding(UTF-8)` layer disappears because Go strings are already UTF-8 bytes",
		Short:     "encoding layer is not needed",
		Advice:    "validate untrusted input with `utf8.ValidString`, which is the check the layer performed",
		Converted: "the layer was dropped and the bytes are read as they are",
		Concepts:  []string{"strings-are-bytes"},
	},

	// -- Processes ----------------------------------------------------------

	Backticks: {
		Severity:  report.Warn,
		Message:   "backticks run `%s` through a shell and capture its output",
		Short:     "backticks became exec.Command",
		Advice:    "pass the arguments to `exec.Command` one per element, and call `sh -c` only where the shell was wanted",
		Cost:      "globbing, redirection and pipes in the command string are no longer expanded",
		Converted: "the emitted code runs the program directly and reads `exec.Cmd.Output`",
		Concepts:  []string{"os-exec"},
	},
	SystemCall: {
		Severity:  report.Warn,
		Message:   "`system(%s)` interpolates into a shell, and the emitted code runs the program directly",
		Short:     "system no longer goes through a shell",
		Advice:    "call `sh -c` explicitly where the shell was wanted for globbing or redirection",
		Cost:      "shell quoting no longer applies, which also removes a shell-injection path",
		Converted: "the emitted code uses `exec.Command` with the arguments split at conversion time",
		Concepts:  []string{"os-exec"},
	},
	ExitStatusShift: {
		Severity:  report.Note,
		Message:   "`$? >> 8` decodes a wait status, and Go hands over the exit code directly",
		Short:     "exit status needs no shifting",
		Advice:    "read `exec.ExitError.ExitCode` after matching the error with `errors.As`",
		Converted: "the shift was dropped and the emitted code reads the code from the error",
		Concepts:  []string{"os-exec", "errors-are-values"},
	},
	Fork: {
		Severity:  report.Warn,
		Message:   "`fork` copies the process, and a goroutine shares memory with the rest of the program",
		Short:     "fork became a goroutine",
		Advice:    "guard the shared state with `sync.Mutex`, or re-exec the program with `exec.Command` for real isolation",
		Cost:      "a child that mutated its own copy of a variable now mutates the parent's",
		Converted: "the emitted code starts a goroutine and waits for it with `sync.WaitGroup`",
		Concepts:  []string{"goroutines-not-fork", "waitgroup-and-mutex", "race-detector"},
	},
	SignalHandler: {
		Severity: report.Warn,
		Message:  "the `%%SIG` handler for `%s` became a `signal.Notify` channel and a goroutine",
		Short:    "signal handler became a channel",
		Advice:   "receive in one place, and use `signal.NotifyContext` to cancel the work the signal interrupts",
		Cost:     "Perl runs the handler between opcodes and Go delivers on a channel, so the handler runs later",
		Concepts: []string{"channels-and-select", "context-cancellation"},
	},
	EnvForChildren: {
		Severity:  report.Note,
		Message:   "writes to `%%ENV` reach child processes in Perl, and Go needs them on `exec.Cmd.Env`",
		Short:     "ENV writes need cmd.Env",
		Advice:    "build `cmd.Env` from `os.Environ()` plus the changes, which is what the emitted code does",
		Converted: "the emitted code sets `cmd.Env` at each spawn site",
		Concepts:  []string{"os-exec"},
	},

	// -- Object orientation -------------------------------------------------

	Bless: {
		Severity:  report.Note,
		Message:   "`bless` became a struct with its methods on a pointer receiver",
		Short:     "bless became a struct",
		Advice:    "keep the pointer receiver: a value receiver copies the struct on every call, which `bless` never did",
		Converted: "the hash keys became struct fields and the package's subs became methods",
		Concepts:  []string{"methods-and-receivers", "structs-and-embedding"},
	},
	MultipleInheritance: {
		Severity:  report.Warn,
		Message:   "`@ISA` lists %d parents, and Go embedding has no method resolution order",
		Short:     "multiple inheritance has no MRO",
		Advice:    "name the parent explicitly at the ambiguous call sites, or move the shared method into one type",
		Cost:      "a method name both parents define does not compile until the call site says which one it means",
		Converted: "the emitted struct embeds both parents",
		Concepts:  []string{"structs-and-embedding", "implicit-interfaces"},
	},
	SuperCall: {
		Severity:  report.Note,
		Message:   "`SUPER::%s` became an explicit call to the embedded parent's method",
		Short:     "SUPER:: resolved at compile time",
		Advice:    "call the embedded field's method by name; Go resolves it at compile time, so a parent change is visible",
		Converted: "the emitted code calls the method on the embedded parent field",
		Concepts:  []string{"structs-and-embedding", "methods-and-receivers"},
	},
	DynamicMethodName: {
		Severity: report.Warn,
		Message:  "the method name in `%s` is a variable, so it is not known until run time",
		Short:    "method name computed at run time",
		Advice:   "declare an interface holding the methods the script can call, or dispatch through a `map[string]func`",
		Cost:     "a name with no matching method is a run-time error where a direct call would not compile",
		Concepts: []string{"implicit-interfaces", "type-assertions-and-switches"},
	},
	CanCheck: {
		Severity:  report.Warn,
		Message:   "`->can(%s)` asks at run time a question Go answers at compile time",
		Short:     "can() became a type assertion",
		Advice:    "declare the small interface the call needs and assert to it with the comma-ok form",
		Converted: "the emitted code uses an interface assertion, which is a compile-time contract instead",
		Concepts:  []string{"implicit-interfaces", "comma-ok-idiom"},
	},
	Destroy: {
		Severity:  report.Warn,
		Message:   "`DESTROY` runs at scope exit in Perl, and Go frees memory when the collector decides",
		Short:     "DESTROY became defer",
		Advice:    "close the resource with `defer` where it is acquired; a finaliser is not a substitute for that",
		Cost:      "sites where the object outlives the sub cannot take a `defer` and are listed in the report",
		Converted: "the emitted code defers the cleanup at the sites where the object does not escape",
		Concepts:  []string{"defer-timing"},
	},

	// -- Modules ------------------------------------------------------------

	GetoptPermutation: {
		Severity: report.Warn,
		Message:  "`Getopt::Long` permutes options and arguments, and the `flag` package stops at the first argument",
		Short:    "flag stops at the first argument",
		Advice:   "put the options before the positional arguments, or parse with `github.com/spf13/pflag` to keep permutation",
		Cost:     "`prog a.txt --file x` leaves `--file x` among the positional arguments",
		Concepts: []string{"small-stdlib-philosophy"},
	},
	JSONNumbers: {
		Severity: report.Note,
		Message:  "JSON numbers decode to `float64` through `map[string]any`, so large integers lose precision",
		Short:    "JSON numbers decode as float64",
		Advice:   "decode into a struct with typed fields, or call `json.Decoder.UseNumber`",
		Cost:     "an integer past 2^53 comes back rounded",
		Concepts: []string{"encoding-json", "struct-tags"},
	},
	DumperEval: {
		Severity:  report.Refuse,
		Message:   "this script reads `Data::Dumper` output back with `eval`, which is persisted Perl source",
		Short:     "Data::Dumper read back with eval",
		Advice:    "change the producer to write JSON, and read it with `encoding/json`",
		Converted: "the read is not converted: the file format is Perl source that only perl can evaluate",
		Concepts:  []string{"encoding-json", "compile-time-mindset"},
	},
	HTTPTimeout: {
		Severity:  report.Warn,
		Message:   "`http.DefaultClient` has no timeout, and `LWP::UserAgent` defaults to 180 seconds",
		Short:     "HTTP client timeout differs",
		Advice:    "build an `http.Client` with the timeout this script needs rather than using the default",
		Converted: "the emitted code builds a client with a 30 second timeout",
		Concepts:  []string{"context-cancellation"},
	},
	SQLPlaceholders: {
		Severity: report.Note,
		Message:  "`DBI` placeholders are `?` everywhere, and in Go the driver decides the syntax",
		Short:    "SQL placeholder syntax is per driver",
		Advice:   "keep `?` for MySQL and SQLite, and use `$1` for the `pq` and `pgx` drivers",
		Cost:     "the emitted SQL uses `?`, which is wrong for PostgreSQL drivers",
	},
	ModuleUnmapped: {
		Severity:  report.Warn,
		Message:   "`%s` has no entry in the module mapping table",
		Short:     "module has no Go mapping",
		Advice:    "search pkg.go.dev for a package with the same job, or write the functions the script calls",
		Cost:      "the stubs compile and return zero values, so the program builds and does not do this work",
		Converted: "the emitted code declares a stub for each function the script calls, each carrying a TODO",
		Concepts:  []string{"go-mod-vs-cpan", "small-stdlib-philosophy"},
	},
	StrictWarnings: {
		Severity: report.Note,
		Message:  "`use strict` and `use warnings` have no Go counterpart because the compiler enforces both",
		Short:    "strict and warnings are the default",
		Advice:   "nothing to do; run `go vet` for the checks the compiler leaves out",
		Concepts: []string{"compile-time-mindset", "vet-and-staticcheck"},
	},
	PosixFloor: {
		Severity: report.Note,
		Message:  "`POSIX::floor` maps to `math.Floor`, which takes and returns `float64`",
		Short:    "POSIX floor maps to math.Floor",
		Advice:   "use plain `/` on integers, which truncates towards zero in Go without a conversion",
		Concepts: []string{"explicit-conversions-no-coercion"},
	},
	Base64Wrapping: {
		Severity:  report.Note,
		Message:   "`encode_base64` wraps at 76 columns and `encoding/base64` emits one line",
		Short:     "base64 line wrapping differs",
		Advice:    "wrap the output where it is compared byte for byte; the emitted helper wraps at 76 columns",
		Concepts:  []string{"strings-are-bytes"},
		Converted: "the emitted code calls a helper that reproduces the wrapping",
	},
	YAMLDependency: {
		Severity:  report.Warn,
		Message:   "YAML has no Go standard library package, so the converted program needs `gopkg.in/yaml.v3`",
		Short:     "YAML needs a third-party package",
		Advice:    "run `go get gopkg.in/yaml.v3` in the output directory, or move the config file to JSON",
		Cost:      "the generated module is no longer free of dependencies",
		Converted: "the emitted code unmarshals with `yaml.Unmarshal` and `go.mod` requires the package",
		Concepts:  []string{"go-mod-vs-cpan", "encoding-json"},
	},
	StorableFormat: {
		Severity:  report.Warn,
		Message:   "Storable files have no Go reader",
		Short:     "Storable files cannot be read",
		Advice:    "write a one-time Perl script that converts the store to JSON, and read the JSON with `encoding/json`",
		Cost:      "existing store files stay unreadable until they are converted once",
		Converted: "the emitted code reads and writes JSON at the same paths",
		Concepts:  []string{"encoding-json", "go-mod-vs-cpan"},
	},
	WalkOrder: {
		Severity: report.Note,
		Message:  "`File::Find` walks in readdir order and `filepath.WalkDir` walks in lexical order",
		Short:    "directory walk order differs",
		Advice:   "rely on the lexical order, or sort the collected paths yourself if a different order is wanted",
		Concepts: []string{"sort-slice"},
	},

	// -- Dynamic Perl -------------------------------------------------------

	EvalString: {
		Severity: report.Refuse,
		Message:  "`eval STRING` builds Perl source at run time, and Go has no compiler at run time",
		Short:    "eval STRING has no Go form",
		Advice: "if the expressions come from a config file, embed an expression evaluator such as " +
			"github.com/expr-lang/expr and pass the values as its environment; if there are only a handful of " +
			"known expressions, replace them with named functions and dispatch on the name",
		Converted: "this call site raises a panic carrying the original Perl text",
		Concepts:  []string{"compile-time-mindset", "panic-and-recover"},
	},
	SymbolicRefMapped: {
		Severity:  report.Warn,
		Message:   "the symbolic reference `%s` became a lookup in a generated dispatch map",
		Short:     "symbolic ref became a map lookup",
		Advice:    "check that every name the script can produce is a key in the map; a missing key is a run-time error",
		Cost:      "names the converter could not enumerate are absent from the map",
		Converted: "the emitted map holds the names the converter found in this file",
		Concepts:  []string{"compile-time-mindset", "maps-of-slices"},
	},
	SymbolicRefOpen: {
		Severity:  report.Refuse,
		Message:   "the symbolic reference `%s` names a variable that is not statically enumerable",
		Short:     "symbolic ref cannot be enumerated",
		Advice:    "build an explicit `map[string]*T` registry and look the name up in it",
		Converted: "the access is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"compile-time-mindset", "maps-of-slices"},
	},
	GlobAssignment: {
		Severity: report.Warn,
		Message:  "`%s` became an `io.Writer` parameter threaded through the call chain",
		Short:    "glob assignment became a parameter",
		Advice:   "keep the writer as a parameter; the signatures that changed are listed in the report",
		Cost:     "every sub between the assignment and the write gained a parameter",
		Concepts: []string{"io-reader-writer", "accept-interfaces-return-structs"},
	},
	GlobComputed: {
		Severity:  report.Refuse,
		Message:   "the typeglob assignment target is built at run time from `%s`",
		Short:     "glob target computed at run time",
		Advice:    "replace the glob with an explicit dispatch table of `func` values",
		Converted: "the assignment is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"compile-time-mindset"},
	},
	AutoloadExpanded: {
		Severity:  report.Warn,
		Message:   "`AUTOLOAD` was expanded at conversion time into the %d methods this file calls",
		Short:     "AUTOLOAD expanded to named methods",
		Advice:    "add the methods by hand, or give the type one `Call(name string, args ...any)` method to route through",
		Cost:      "a method name the script assembles at run time is not among the expanded ones",
		Converted: "the emitted type declares one method per name found in this file",
		Concepts:  []string{"methods-and-receivers", "compile-time-mindset"},
	},
	AutoloadOpen: {
		Severity:  report.Refuse,
		Message:   "`AUTOLOAD` here answers method names that are not visible in this file",
		Short:     "AUTOLOAD names are not visible",
		Advice:    "give the type one `Call(name string, args ...any) (any, error)` method and route the calls through it",
		Converted: "the class is not converted: the set of methods it answers is not knowable from this file",
		Concepts:  []string{"methods-and-receivers", "compile-time-mindset"},
	},
	TieConverted: {
		Severity:  report.Warn,
		Message:   "`tie %s` became explicit method calls, which loses the hash syntax",
		Short:     "tie became explicit method calls",
		Advice:    "for a real key-value store, `go.etcd.io/bbolt` or `modernc.org/sqlite` fit this usage",
		Cost:      "each access is a method call, so a read that Perl wrote as an index is now two lines",
		Converted: "the emitted code calls `Get`, `Set` and `Delete` at each access",
		Concepts:  []string{"methods-and-receivers", "maps-of-slices"},
	},
	TieEscapes: {
		Severity:  report.Refuse,
		Message:   "the tied variable `%s` is passed to code outside this file, where the tie is invisible",
		Short:     "tied variable escapes this file",
		Advice:    "convert the tied class to a type with `Get`, `Set`, `Delete` and `Each`, and call those explicitly",
		Converted: "the variable is not converted: the receiving code would see a plain map with no tie behind it",
		Concepts:  []string{"methods-and-receivers"},
	},
	OperatorOverload: {
		Severity:  report.Warn,
		Message:   "the `%s` overload on `%s` was applied at the call sites whose types are known",
		Short:     "operator overload became method calls",
		Advice:    "call the method by name at the sites the report lists; Go has no operator overloading",
		Cost:      "call sites whose operand types stayed dynamic carry a TODO instead of the method call",
		Converted: "the resolved sites call the method the overload named",
		Concepts:  []string{"methods-and-receivers", "explicit-conversions-no-coercion"},
	},
	MonkeyPatch: {
		Severity:  report.Warn,
		Message:   "the run-time replacement of `%s` became a package-level `var` the test can swap",
		Short:     "run-time replacement became a var",
		Advice:    "swap the `var` in the test and restore it with `defer`, which is the Go seam for this job",
		Cost:      "the call goes through a function value, so it is not inlined and it is not a compile-time constant",
		Converted: "the function is declared as a `var` of func type and called through it",
		Concepts:  []string{"var-vs-short-declaration", "defer-timing"},
	},
	FormatWrite: {
		Severity:  report.Warn,
		Message:   "`format %s` became a `text/tabwriter` function with the same columns",
		Short:     "format became text/tabwriter",
		Advice:    "check the column widths, and write the wrapping by hand where the picture line used `^<<<`",
		Cost:      "fill mode and multi-line wrapping are not reproduced",
		Converted: "the emitted function writes the fields through a `tabwriter.Writer`",
		Concepts:  []string{"io-reader-writer"},
	},
	GotoSub: {
		Severity:  report.Refuse,
		Message:   "`goto &%s` replaces the current frame with a sub chosen at run time",
		Short:     "goto &sub replaces the frame",
		Advice:    "call the function through a `map[string]func(...)` and return its result instead",
		Converted: "the jump is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"compile-time-mindset"},
	},
	GotoOverDeclaration: {
		Severity:  report.Refuse,
		Message:   "`goto %s` jumps backwards over the declaration of `%s`, which Go's `goto` forbids",
		Short:     "goto jumps over a declaration",
		Advice:    "restructure the jump as a `for` loop with a labelled `continue`",
		Converted: "the jump is not converted: Go rejects a `goto` that skips a variable declaration",
		Concepts:  []string{"var-vs-short-declaration"},
	},
	BeginDecidesParse: {
		Severity:  report.Refuse,
		Message:   "the `BEGIN` block computes a value that decides how the rest of this file parses",
		Short:     "BEGIN changes later parsing",
		Advice:    "move the computation out of `BEGIN`, or convert the region it affects by hand",
		Converted: "the file is not converted past this point: what follows depends on running the Perl",
		Concepts:  []string{"compile-time-mindset"},
	},
	BeginEndBlocks: {
		Severity:  report.Note,
		Message:   "`BEGIN` and `END` blocks became `init` and a `defer` at the top of `main`",
		Short:     "BEGIN and END became init and defer",
		Advice:    "keep `init` small: it runs on package load, earlier than `BEGIN` runs relative to the rest of the file",
		Converted: "each `BEGIN` became an `init` function and each `END` became a deferred call in `main`",
		Concepts:  []string{"defer-timing", "packages-and-exported-names"},
	},
	InlineC: {
		Severity:  report.Refuse,
		Message:   "`Inline::C` embeds C source, which has no static Go translation",
		Short:     "Inline::C has no static translation",
		Advice:    "port the C to Go, or keep it behind cgo and call it from the converted program",
		Converted: "the C is not converted: translating C to Go is a different tool's job",
		Concepts:  []string{"go-mod-vs-cpan"},
	},

	// -- Verification of the tool's own output ------------------------------

	OutputDoesNotParse: {
		Severity:  report.Refuse,
		Message:   "the generated Go does not parse, which is a bug in perl2go and not in the input",
		Short:     "generated Go does not parse",
		Advice:    "report this with the input file; nothing was written to disk",
		Converted: "nothing was written: output that does not parse is worse than no output",
		Concepts:  []string{"toolchain-gofmt-godoc"},
	},
	OutputDoesNotCompile: {
		Severity:  report.Refuse,
		Message:   "the generated program does not compile: %s",
		Short:     "generated program does not compile",
		Advice:    "report this with the input file; the failing source is kept in the verification directory named above",
		Cost:      "the conversion cannot be trusted where the compiler disagrees with it",
		Converted: "nothing was written to the output directory",
		Concepts:  []string{"toolchain-gofmt-godoc", "compile-time-mindset"},
	},
	OutputVetFinding: {
		Severity:  report.Warn,
		Message:   "`go vet` reports `%s` in the generated program",
		Short:     "go vet flagged the generated code",
		Advice:    "report this; the program still builds and runs",
		Converted: "the output was written; the finding is about its style, not its correctness",
		Concepts:  []string{"vet-and-staticcheck"},
	},
	OutputNotFormatted: {
		Severity:  report.Refuse,
		Message:   "the generated Go could not be formatted, which is a bug in perl2go",
		Short:     "generated Go could not be formatted",
		Advice:    "report this with the input file; `gofmt` rejects what the emitter produced",
		Converted: "nothing was written to disk",
		Concepts:  []string{"toolchain-gofmt-godoc"},
	},
	CleanOutputMentionsPerl: {
		Severity:  report.Refuse,
		Message:   "the clean output mentions Perl, which the clean output is not allowed to do",
		Short:     "clean output mentions Perl",
		Advice:    "report this with the input file; the annotated output is unaffected",
		Converted: "the annotated output was written and the clean output was withheld",
	},
	NoToolchain: {
		Severity: report.Note,
		Message:  "no Go toolchain was found, so the generated program was parsed but not compiled",
		Short:    "no Go toolchain to build with",
		Advice:   "install Go and run `go build ./...` in the output directory",
		Cost:     "the strongest check the tool has on its own output did not run",
		Concepts: []string{"toolchain-gofmt-godoc"},
	},

	// -- AI mode ------------------------------------------------------------

	AIRuntimeUnreachable: {
		Severity:  report.Warn,
		Message:   "no inference runtime answered at `%s`, and the deterministic output was kept",
		Short:     "no inference runtime answered",
		Advice:    "start the runtime, or run `perl2go ai status` to see what is configured",
		Converted: "the conversion is the deterministic one, unchanged",
	},
	AIRewriteRejectedBuild: {
		Severity:  report.Note,
		Message:   "the model's rewrite of `%s` was rejected because it did not compile",
		Short:     "model rewrite did not compile",
		Advice:    "run with `--debug` to see the rejected text",
		Converted: "the deterministic output was kept",
	},
	AIRewriteRejectedBehaviour: {
		Severity:  report.Warn,
		Message:   "the model's rewrite changed the program's output on the corpus check, and it was rejected",
		Short:     "model rewrite changed behaviour",
		Advice:    "nothing to do; the guard rejected the rewrite before it reached the output",
		Converted: "the deterministic output was kept",
	},
	AIModelTooLarge: {
		Severity: report.Warn,
		Message:  "`%s` needs about %s of VRAM and %s is free",
		Short:    "model does not fit in free VRAM",
		Advice:   "run `perl2go ai setup`, which lists the models that fit the free VRAM",
	},
	AIPartialImprovement: {
		Severity:  report.Note,
		Message:   "AI mode improved %d of %d files, and the rest kept their deterministic output",
		Short:     "AI mode improved some of the files",
		Advice:    "read the per-file result in the conversion report",
		Converted: "each file carries whichever version passed the guards",
	},

	// -- REPL ---------------------------------------------------------------

	ReplIncomplete: {
		Severity: report.Note,
		Message:  "the snippet is incomplete, so the REPL is still reading",
		Short:    "snippet is incomplete",
		Advice:   "finish the statement, or press Ctrl-C to discard it",
	},
	ReplParseError: {
		Severity: report.Warn,
		Message:  "the snippet did not parse, and the session state is unchanged",
		Short:    "snippet did not parse",
		Advice:   "`:help` lists the meta commands",
	},
}
