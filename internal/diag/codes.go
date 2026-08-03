// Package diag is the registry of everything perl2golang can say about one place in
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

import "perl2golang/internal/report"

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
	// InternalPanic: perl2golang panicked while converting.
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
	// HashFromOddList: a hash is built from a list of unknown length.
	HashFromOddList Code = "P2G2050"
	// DefinedOnValueType: `defined` cannot see the absence of a value here.
	DefinedOnValueType Code = "P2G2110"
	// BareReturnZeroValues: a bare `return` became the declared zero values.
	BareReturnZeroValues Code = "P2G2120"
	// MissingArguments: a call passes fewer arguments than the sub unpacks.
	MissingArguments Code = "P2G2130"
	// BlockArgument: a bare block argument became a function literal.
	BlockArgument Code = "P2G2135"
	// ReturnsList: a sub returning a list returns the values, not the count.
	ReturnsList Code = "P2G2121"
	// UndefClearsToZero: `undef $x` leaves the type's zero value behind.
	UndefClearsToZero Code = "P2G2115"
	// ValuelessCall: a call to a sub nothing reads cannot stand in an
	// expression, so it runs on its own line.
	ValuelessCall Code = "P2G2125"
)

// References, aliasing, and mutation.
const (
	// Autovivification: a nested access creates the intermediate containers.
	Autovivification Code = "P2G2510"
	// SliceAssignment: an assignment writes several elements through a slice.
	SliceAssignment Code = "P2G2530"
	// AssignTargetShape: the left side of an assignment has no lowering rule.
	AssignTargetShape Code = "P2G2540"
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
	// CollectionWidened: a collection of one element type was copied into one
	// that holds anything.
	CollectionWidened Code = "P2G3021"
	// CollectionNarrowed: a collection of values of no fixed type was copied
	// into one with a single element type, asserting each value.
	CollectionNarrowed Code = "P2G3022"
	// SubstrAsTarget: `substr(...) = ...` edits a string in place.
	SubstrAsTarget Code = "P2G2545"
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
	// OperatorNoRule: an operator has no lowering rule in the converter.
	OperatorNoRule Code = "P2G3511"
	// GotoLabel: `goto LABEL` moves control in a way Go's goto cannot.
	GotoLabel Code = "P2G3520"
	// ForPostList: a C-style for header carries several post expressions.
	ForPostList Code = "P2G3530"
	// DoBlockNoValue: a `do BLOCK` used as an expression ends in a statement
	// that produces no value.
	DoBlockNoValue Code = "P2G3540"
	// DoFile: `do FILE` compiles and runs another Perl file at run time.
	DoFile Code = "P2G3541"
	// ConstructNoRule: a construct has no lowering rule in the converter.
	ConstructNoRule Code = "P2G3599"
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
	// RegexAtomicGroup: a pattern uses an atomic group.
	RegexAtomicGroup Code = "P2G4012"
	// RegexKeepOut: a pattern uses `\K`.
	RegexKeepOut Code = "P2G4014"
	// RegexFreeSpacing: `/x` was expanded at conversion time.
	RegexFreeSpacing Code = "P2G4030"
	// SubstEval: `s///e` evaluates its replacement as Perl.
	SubstEval Code = "P2G4040"
	// SubstDoubleEval: `s///ee` evaluates the replacement's result again.
	SubstDoubleEval Code = "P2G4041"
	// RegexDollarAnchor: `$` matches in one more place in Perl than in Go.
	RegexDollarAnchor Code = "P2G4060"
	// RuntimePattern: the pattern text is only known at run time.
	RuntimePattern Code = "P2G4080"
)

// Regex and string-splitting semantics.
const (
	// SplitSingleSpace: `split ' '` is not a split on one space.
	SplitSingleSpace Code = "P2G4510"
	// TrModifiers: `tr///` carries modifiers that change the rule.
	TrModifiers Code = "P2G4520"
	// TrCounts: `tr///` with an empty replacement list counts characters.
	TrCounts Code = "P2G4521"
	// MatchPosition: `/g` and `pos` keep match state on the scalar.
	MatchPosition Code = "P2G4530"
	// PosNotStarted: `pos` on a scalar no scan has walked reads as 0.
	PosNotStarted Code = "P2G4531"
	// BindingRightSide: the right side of `=~` is not a static pattern.
	BindingRightSide Code = "P2G4590"
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
	// HexOctParseError: `hex` and `oct` answer 0 where Go returns an error.
	HexOctParseError Code = "P2G5050"
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
	// ArrayLengthAssignment: assigning to `$#array` sets the array's length.
	ArrayLengthAssignment Code = "P2G5560"
	// AssignPastEnd: an assignment writes past the end of an array.
	AssignPastEnd Code = "P2G5561"
	// EachIterator: `each` walks a hash through state kept on the hash.
	EachIterator Code = "P2G5570"
	// SpliceReturn: `splice` returns the elements it removed.
	SpliceReturn Code = "P2G5580"
	// SpliceForm: a `splice` inserts, removes or replaces in one call.
	SpliceForm Code = "P2G5581"
	// SortNamedComparator: `sort` names a sub that reads `$a` and `$b`.
	SortNamedComparator Code = "P2G5590"
	// SortRefComparator: `sort $cmp LIST` holds the comparator in a variable.
	SortRefComparator Code = "P2G5592"
	// SortRefUnresolved: `sort $cmp LIST` where the comparator did not resolve.
	SortRefUnresolved Code = "P2G5593"
	// SortBlockNoOrder: a `sort` block does not end in an ordering.
	SortBlockNoOrder Code = "P2G5591"
	// MapBlockNoValue: a `map` block does not end in an expression.
	MapBlockNoValue Code = "P2G5595"
	// GrepBlockNoTest: a `grep` block does not end in an expression.
	GrepBlockNoTest Code = "P2G5596"
)

// File I/O.
const (
	// TwoArgOpen: `open` takes its mode from the front of the filename.
	TwoArgOpen Code = "P2G6001"
	// OpenModePipe: an `open` mode selects a pipe or a duplicated handle.
	OpenModePipe Code = "P2G6002"
	// OpenModeComputed: the `open` mode is only known at run time.
	OpenModeComputed Code = "P2G6003"
	// OpenUnchecked: the original does not check whether `open` succeeded.
	OpenUnchecked Code = "P2G6005"
	// ReadLineLoop: `while (<FH>)` became a bufio.Scanner.
	ReadLineLoop Code = "P2G6010"
	// ReadLineKeepsNewline: readline keeps the newline and Scanner strips it.
	ReadLineKeepsNewline Code = "P2G6011"
	// SlurpFile: `$/ = undef` reads the whole file at once.
	SlurpFile Code = "P2G6012"
	// InputLineNumber: `$.` counts lines across every handle at once.
	InputLineNumber Code = "P2G6015"
	// OutputFormatVars: a global changes how `print` and `split` behave.
	OutputFormatVars Code = "P2G6016"
	// LastSystemError: `$!` holds the error of the last failed system call.
	LastSystemError Code = "P2G6017"
	// SeparatorNotStatic: a separator variable is set to a value the
	// converter cannot read while converting.
	SeparatorNotStatic Code = "P2G6018"
	// SeparatorFolded: a separator variable's value was written into the
	// calls it governs instead of into a variable.
	SeparatorFolded Code = "P2G6019"
	// AutoflushNoOp: `$| = 1` has nothing to disable in the emitted code.
	AutoflushNoOp Code = "P2G6020"
	// FileTest: a `-f` style file test became os.Stat.
	FileTest Code = "P2G6030"
	// StatReuse: the `_` handle reuses the previous test's stat, and the
	// generated code inspects the path again.
	StatReuse Code = "P2G6031"
	// SizeOfMissingFile: `-s` on a file that cannot be inspected reports 0.
	SizeOfMissingFile Code = "P2G6032"
	// PermissionBits: `-w` and `-x` are answered from the permission bits.
	PermissionBits Code = "P2G6033"
	// DirRead: opendir reads the whole directory at once.
	DirRead Code = "P2G6042"
	// DirClosed: the directory was read in one call, so closedir was dropped.
	DirClosed Code = "P2G6041"
	// UnlinkReturnsError: `unlink` counts removals and `os.Remove` returns an
	// error.
	UnlinkReturnsError Code = "P2G6045"
	// WriteErrorChecked: print errors are silent in Perl and checked in Go.
	WriteErrorChecked Code = "P2G6040"
	// EncodingLayer: an `:encoding` layer has no Go counterpart.
	EncodingLayer Code = "P2G6060"
	// EnvAssignment: writing to `%ENV` becomes os.Setenv, which returns an
	// error the assignment had nowhere to put.
	EnvAssignment Code = "P2G6070"
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
	// DynamicInvocant: a value of unknown class was asserted before a call.
	DynamicInvocant Code = "P2G7002"
	// IsaPredicate: `isa` became a predicate listing the concrete types.
	IsaPredicate Code = "P2G7003"
	// ClassForwarder: a class method forwarding to $class->new is inlined.
	ClassForwarder Code = "P2G7004"
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
	// AccessorField: a sub that only reads one key became an exported field.
	AccessorField Code = "P2G7040"
	// MethodNotFound: no sub of that name exists in the class or its parents.
	MethodNotFound Code = "P2G7041"
	// MethodNeedsObject: an instance method was reached without an object.
	MethodNeedsObject Code = "P2G7042"
	// ConstructorArgs: the named arguments were not a written-out list.
	ConstructorArgs Code = "P2G7043"
	// ConstructorArgUnread: a named argument the constructor never reads.
	ConstructorArgUnread Code = "P2G7044"
	// IsaCheck: `->isa` was answered from the class hierarchy in the file.
	IsaCheck Code = "P2G7045"
	// LateBinding: an override cannot be reached from an embedded parent.
	LateBinding Code = "P2G7046"
	// InheritedConstructor: a class inherits its constructor from its parent.
	InheritedConstructor Code = "P2G7047"
	// ComputedFieldName: a blessed hash was given a key worked out at run time.
	ComputedFieldName Code = "P2G7048"
	// AccessorReadOnly: a read-only accessor was called with a value.
	AccessorReadOnly Code = "P2G7049"
	// ClassAlias: `ref($proto) || $proto` has one answer in Go.
	ClassAlias Code = "P2G7050"
	// Autoload: `AUTOLOAD` catches calls to methods that were never written.
	Autoload Code = "P2G7035"
	// Overload: `use overload` makes an operator a method call.
	Overload Code = "P2G7025"
)

// Modules and the CPAN mapping table.
const (
	// GetoptPermutation: Getopt::Long permutes and flag does not.
	GetoptPermutation Code = "P2G7505"
	// GetoptBlock: an option block the converter could not take apart.
	GetoptBlock Code = "P2G7500"
	// GetoptSpec: one specification string the converter did not understand.
	GetoptSpec Code = "P2G7501"
	// GetoptDestination: an option destination that is not a plain variable.
	GetoptDestination Code = "P2G7502"
	// GetoptRepetition: `{n,m}` swallows several words and flag cannot.
	GetoptRepetition Code = "P2G7503"
	// GetoptOptionalValue: `:s` no longer takes the word after it.
	GetoptOptionalValue Code = "P2G7504"
	// GetoptAliases: an option with several spellings is several registrations.
	GetoptAliases Code = "P2G7506"
	// GetoptBundling: `-abc` as three options has no counterpart.
	GetoptBundling Code = "P2G7507"
	// GetoptPassThrough: an unknown option is an error rather than an operand.
	GetoptPassThrough Code = "P2G7508"
	// GetoptConfigure: a Configure setting with nothing to change.
	GetoptConfigure Code = "P2G7509"
	// JSONNumbers: JSON numbers decode as float64 through any.
	JSONNumbers Code = "P2G7510"
	// DumperEval: Data::Dumper output is read back with eval.
	DumperEval Code = "P2G7515"
	// HTTPTimeout: the HTTP client default timeout differs.
	HTTPTimeout Code = "P2G7525"
	// SQLPlaceholders: placeholder syntax is decided by the driver.
	SQLPlaceholders Code = "P2G7530"
	// ModuleFunctionUnmapped: a mapped module's function has no rule.
	ModuleFunctionUnmapped Code = "P2G7540"
	// ListUtilFirst: `first` yields the zero value when nothing matches.
	ListUtilFirst Code = "P2G7541"
	// ListUtilQuantifier: `any`, `all` and `none` became a loop with a break.
	ListUtilQuantifier Code = "P2G7542"
	// ListUtilReduce: `reduce` folds to the zero value on an empty list.
	ListUtilReduce Code = "P2G7543"
	// ListUtilPairs: `pairs` drops the last element of an odd-length list.
	ListUtilPairs Code = "P2G7544"
	// ModuleUnmapped: a module has no entry in the mapping table.
	ModuleUnmapped Code = "P2G7550"
	// ModuleInlined: a module beside the script was converted with it.
	ModuleInlined Code = "P2G7551"
	// ParentEmbedded: `use parent` became embedding.
	ParentEmbedded Code = "P2G7552"
	// StrictWarnings: `use strict` and `use warnings` have no counterpart.
	StrictWarnings Code = "P2G7555"
	// PosixFloor: POSIX::floor maps to math.Floor.
	PosixFloor Code = "P2G7560"
	// ListUtilMapped: List::Util maps to slices and to explicit loops.
	ListUtilMapped Code = "P2G7561"
	// DumperMapped: Data::Dumper maps to fmt or to encoding/json.
	DumperMapped Code = "P2G7562"
	// ScalarUtilMapped: Scalar::Util maps to a type switch.
	ScalarUtilMapped Code = "P2G7563"
	// BasenameMapped: File::Basename maps to path/filepath.
	BasenameMapped Code = "P2G7564"
	// TimeModuleMapped: the Time:: modules map to the time package.
	TimeModuleMapped Code = "P2G7565"
	// FileSpecMapped: File::Spec's class methods map to path/filepath.
	FileSpecMapped Code = "P2G7566"
	// CwdMapped: Cwd maps to os.Getwd and filepath.Abs.
	CwdMapped Code = "P2G7567"
	// AbsPathMissing: abs_path of a missing path has no undef to return.
	AbsPathMissing Code = "P2G7568"
	// DumperFormat: a structure dump comes out as Go syntax, not Perl syntax.
	DumperFormat Code = "P2G7569"
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
	// EvalBlock: `eval { }` traps a die, which becomes panic and recover.
	EvalBlock Code = "P2G8002"
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
		Advice:   "pass one of the two; `perl2golang convert --help` says what each one does",
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
		Message:  "perl2golang panicked while converting `%s`, which is always a bug in perl2golang",
		Short:    "internal panic while converting",
		Advice:   "report this with the input file; nothing was written to disk",
	},
	UnknownCode: {
		Severity: report.Refuse,
		Message:  "diagnostic code `%s` is not in the registry, which is always a bug in perl2golang",
		Short:    "unknown diagnostic code",
		Advice:   "report this with the input file; `perl2golang explain` lists every code that exists",
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
		Message:  "`--strict` was given and this run produced %s",
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
	HashFromOddList: {
		Severity:  report.Warn,
		Message:   "the list this hash is built from has no length the converter can see",
		Short:     "hash built from a list of unknown length",
		Advice:    "build the map with an explicit loop over the list, two elements at a time",
		Cost:      "the pairs cannot be matched up at conversion time, so none of them reach the map",
		Converted: "the emitted code declares the map empty and leaves the filling to the loop",
	},
	DefinedOnValueType: {
		Severity:  report.Warn,
		Message:   "`defined` cannot tell an unassigned `%s` from one holding the zero value",
		Short:     "defined compares against the zero value",
		Advice:    "declare the variable as a pointer, where `nil` is the absence Perl's undef means",
		Cost:      "a variable that was never assigned reads the same as one assigned 0 or the empty string",
		Converted: "the emitted code tests against the zero value",
		Concepts:  []string{"nil-vs-undef", "static-types-and-zero-values"},
	},
	BareReturnZeroValues: {
		Severity:  report.Warn,
		Message:   "a bare `return` yields undef or the empty list, and a Go function returns its results",
		Short:     "bare return became explicit zero values",
		Advice:    "add an error or a bool result and check it, which is how Go says there is no result",
		Cost:      "a caller cannot tell the empty answer from a real one that happens to be the zero value",
		Converted: "the emitted `return` passes the zero value of each declared result",
		Concepts:  []string{"multiple-return-values", "comma-ok-idiom"},
	},
	ReturnsList: {
		Severity:  report.Note,
		Message:   "a sub returning a list hands back the values here, not how many there were",
		Short:     "the list is returned, not its length",
		Advice:    "where a caller wanted the count, take `len` of what comes back",
		Converted: "the emitted function returns a slice",
		Concepts:  []string{"context-is-gone", "multiple-return-values"},
	},
	BlockArgument: {
		Severity:  report.Note,
		Message:   "a bare block argument became the function literal the `&` prototype stood for",
		Short:     "the block became an argument",
		Advice:    "nothing to change: the block was always a code reference, and Go writes it as one",
		Converted: "the emitted call passes a function literal first",
		Concepts:  []string{"variadic-and-no-defaults"},
	},
	MissingArguments: {
		Severity:  report.Warn,
		Message:   "this call passes fewer arguments than `%s` unpacks, and Go requires every parameter",
		Short:     "missing argument became a zero value",
		Advice:    "give the parameter a default inside the function, or split the function in two",
		Cost:      "Perl left the unpassed variables undef, which a zero value does not always stand in for",
		Converted: "the missing arguments are passed as their type's zero value",
		Concepts:  []string{"variadic-and-no-defaults"},
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
	SliceAssignment: {
		Severity:  report.Refuse,
		Message:   "a slice assignment writes several elements at once, and Go has no syntax for it",
		Short:     "slice assignment has no Go syntax",
		Advice:    "write one assignment per element, or a loop over the index and value pairs",
		Converted: "the assignment is not converted; the statement panics with the original Perl text",
		Concepts:  []string{"slices-not-arrays"},
	},
	AssignTargetShape: {
		Severity:  report.Refuse,
		Message:   "the shape of the left side of this assignment has no rule in the converter",
		Short:     "assignment target has no rule",
		Advice:    "translate the assignment by hand; the original is quoted above the stub",
		Converted: "the assignment is not converted; the statement panics with the original Perl text",
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
	SubstrAsTarget: {
		Severity:  report.Warn,
		Message:   "`substr` on the left of an assignment edits a string in place, and a Go string cannot be edited",
		Short:     "the string is rebuilt, not edited",
		Advice:    "use a `[]byte` or a `strings.Builder` where a string is edited repeatedly",
		Cost:      "the whole string is copied on every write, where Perl edited the window",
		Converted: "the window is replaced and the result assigned back over the variable",
		Concepts:  []string{"strings-are-bytes"},
	},
	CollectionWidened: {
		Severity:  report.Warn,
		Message:   "a collection of one element type has no conversion to one that holds anything",
		Short:     "the collection was copied element by element",
		Advice:    "give the destination the same element type as the source and the copy goes away",
		Cost:      "the two are separate collections: changing one does not change the other",
		Converted: "the elements were copied into a new collection of `any`",
		Concepts:  []string{"collections-hold-one-type", "slice-aliasing-and-copy"},
	},
	CollectionNarrowed: {
		Severity:  report.Warn,
		Message:   "a collection of values of no fixed type has no conversion to one with a single element type",
		Short:     "each value was asserted on the way across",
		Advice:    "give the source the same element type as the destination and the assertions go away",
		Cost:      "a value that is not of that type lands as the type's zero value",
		Converted: "the values were copied across with a type assertion on each",
		Concepts:  []string{"collections-hold-one-type", "type-assertions-and-switches"},
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
	OperatorNoRule: {
		Severity:  report.Refuse,
		Message:   "the converter has no rule for the `%s` operator",
		Short:     "operator has no rule",
		Advice:    "translate the expression by hand; the original is quoted above the stub",
		Converted: "the operator is not converted; the call site panics with the original Perl text",
	},
	GotoLabel: {
		Severity:  report.Refuse,
		Message:   "`goto %s` moves control in a way Go's `goto` cannot express",
		Short:     "goto has no Go equivalent here",
		Advice:    "a labelled `break` or `continue` covers jumps out of loops; anything else wants a small state machine",
		Converted: "the jump is not converted: Go's `goto` cannot enter a block or jump over a declaration",
		Concepts:  []string{"var-vs-short-declaration"},
	},
	ForPostList: {
		Severity:  report.Warn,
		Message:   "a C-style `for` takes a comma expression in its third slot, and Go takes one statement",
		Short:     "for header takes one post expression",
		Advice:    "fold the work into one expression where a `next` in the body has to run it",
		Cost:      "a `next` in the body skips the moved expressions, where Perl ran them",
		Converted: "the extra post expressions were moved to the end of the loop body",
	},
	DoBlockNoValue: {
		Severity:  report.Refuse,
		Message:   "a `do BLOCK` used as an expression ends in a statement that has no value to hand back",
		Short:     "the block hands nothing back",
		Advice:    "assign the value to a variable inside the block and read that variable afterwards",
		Converted: "the block's statements converted and stand above the stub; only its value is missing",
		Concepts:  []string{"statements-vs-expressions"},
	},
	DoFile: {
		Severity:  report.Refuse,
		Message:   "`do FILE` compiles and evaluates another Perl file while the program runs",
		Short:     "running another file has no Go equivalent",
		Advice:    "compile the second file into the program, or read it as data if it holds configuration",
		Converted: "the call is not converted; the stub names the file it would have run",
		Concepts:  []string{"packages-and-exported-names"},
	},
	ConstructNoRule: {
		Severity:  report.Refuse,
		Message:   "the converter has no rule for this construct, and a guessed one would look right and run differently",
		Short:     "construct has no rule",
		Advice:    "translate it by hand; the original is quoted above the stub",
		Converted: "the construct is not converted; the stub panics with the original Perl text",
	},

	// -- Regex features RE2 lacks -------------------------------------------

	RegexBackreference: {
		Severity:  report.Refuse,
		Message:   "backreference `%s` is not available in Go's `regexp` package",
		Short:     "backreference has no RE2 spelling",
		Advice:    "capture once and compare the two parts in Go, which lets the pattern go back to `regexp`",
		Converted: "the match site is marked and yields no match",
		Concepts:  []string{"regexp-is-re2"},
	},
	RegexLookahead: {
		Severity:  report.Refuse,
		Message:   "lookahead `%s` is not available in Go's `regexp` package",
		Short:     "lookahead has no RE2 spelling",
		Advice:    "drop the lookahead and test the surrounding text with `strings.HasPrefix` or a second match",
		Converted: "the match site is marked and yields no match",
		Concepts:  []string{"regexp-is-re2", "mustcompile-pattern"},
	},
	RegexLookbehind: {
		Severity:  report.Refuse,
		Message:   "lookbehind `%s` is not available in Go's `regexp` package",
		Short:     "lookbehind has no RE2 spelling",
		Advice:    "capture the preceding text in a group and use the capture instead of the assertion",
		Converted: "the match site is marked and yields no match",
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
	RegexAtomicGroup: {
		Severity:  report.Refuse,
		Message:   "an atomic group forbids backtracking into it, and RE2 has no backtracking to forbid",
		Short:     "atomic group has no RE2 spelling",
		Advice:    "remove the atomic group: RE2's linear-time guarantee is what it was protecting against",
		Converted: "the pattern is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"regexp-is-re2"},
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
	RegexDollarAnchor: {
		Severity:  report.Note,
		Message:   "without `/m`, Perl's `$` also matches before a final newline, and Go's matches at the end",
		Short:     "$ matches in one more place in Perl",
		Advice:    "write `\\n?$` where the text still carries its newline, which restores the Perl meaning",
		Converted: "the emitted pattern keeps `$`, which agrees for lines that have been chomped",
		Concepts:  []string{"regexp-is-re2"},
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
	TrCounts: {
		Severity:  report.Note,
		Message:   "`tr///` with an empty replacement list counts the characters rather than changing them",
		Short:     "tr with no replacement counts",
		Advice:    "`strings.Count` counts one substring; a set of characters needs a loop",
		Converted: "the emitted code calls a helper that counts the characters in the search list",
		Concepts:  []string{"strings-are-bytes"},
	},
	UndefClearsToZero: {
		Severity:  report.Warn,
		Message:   "`undef $x` leaves the type's zero value, which is not a state `defined` can see",
		Short:     "clearing a variable leaves its zero value",
		Advice:    "declare the variable as a pointer where `nil` really has to mean absent",
		Converted: "the emitted code assigns the zero value of the declared type",
		Concepts:  []string{"nil-vs-undef", "static-types-and-zero-values"},
	},
	ValuelessCall: {
		Severity:  report.Warn,
		Message:   "nothing reads what this sub returns, so the Go function returns nothing and a call to it is not a value",
		Short:     "a call with no value runs on its own line",
		Advice:    "return the value explicitly if it is wanted, and the signature will follow",
		Converted: "the call is emitted as its own statement",
		Concepts:  []string{"multiple-return-values"},
	},
	PosNotStarted: {
		Severity:  report.Warn,
		Message:   "`pos` is undef until a global match has walked the scalar, and an int has no undef",
		Short:     "an unstarted scan reads as 0",
		Advice:    "keep a separate bool where \"not started\" has to be told from \"at the beginning\"",
		Converted: "the emitted code reads the position variable, which starts at 0",
		Concepts:  []string{"nil-vs-undef"},
	},
	StatReuse: {
		Severity:  report.Warn,
		Message:   "`_` reuses the previous test's stat, and Go keeps no such cache",
		Short:     "the path is inspected again",
		Advice:    "call `os.Stat` once, keep the `FileInfo`, and read every answer off it",
		Converted: "the emitted code tests the same path a second time",
		Concepts:  []string{"errors-are-values"},
	},
	SizeOfMissingFile: {
		Severity:  report.Warn,
		Message:   "`-s` returns undef for a file it cannot inspect, and an int cannot say that",
		Short:     "a missing file reports a size of zero",
		Advice:    "call `os.Stat` and look at the error where a missing file differs from an empty one",
		Converted: "the emitted helper answers 0 for both",
		Concepts:  []string{"nil-vs-undef"},
	},
	PermissionBits: {
		Severity:  report.Warn,
		Message:   "`-w` and `-x` ask what this process may do; the permission bits do not know who runs it",
		Short:     "writability is read off the permission bits",
		Advice:    "try the operation and handle the error, which is the only answer that cannot go stale",
		Converted: "the emitted helper reads `Mode().Perm()`",
		Concepts:  []string{"errors-are-values"},
	},
	DirRead: {
		Severity:  report.Warn,
		Message:   "`opendir` hands back a cursor and `os.ReadDir` reads the whole directory at once",
		Short:     "the whole directory is read at once",
		Advice:    "`os.File.ReadDir(n)` reads in batches, for a directory too large to hold",
		Converted: "the emitted code reads every name before the loop starts",
	},
	DirClosed: {
		Severity:  report.Note,
		Message:   "the directory was read in one call, so there is no handle left for `closedir` to close",
		Short:     "closedir was dropped",
		Advice:    "nothing leaks: `os.ReadDir` closes the directory before it returns",
		Converted: "the call is not emitted",
	},
	UnlinkReturnsError: {
		Severity:  report.Warn,
		Message:   "`unlink` returns how many files it removed, and `os.Remove` returns an error instead",
		Short:     "unlink's count became an error value",
		Advice:    "test the returned error; `errors.Is(err, fs.ErrNotExist)` is often the case worth ignoring",
		Converted: "the emitted code calls `os.Remove`",
		Concepts:  []string{"errors-are-values"},
	},
	EnvAssignment: {
		Severity:  report.Warn,
		Message:   "setting `%%ENV` becomes `os.Setenv`, which returns an error the assignment had nowhere to put",
		Short:     "the error os.Setenv returns is not checked",
		Advice:    "check the returned error where the setting matters",
		Converted: "the emitted code calls `os.Setenv` and drops the error",
		Concepts:  []string{"errors-are-values"},
	},
	ListUtilFirst: {
		Severity:  report.Warn,
		Message:   "`first` returns undef when nothing matches, and a variable of the element type has no undef",
		Short:     "no match yields the zero value",
		Advice:    "return the index and use -1 for no match, or a second bool as the standard library does",
		Converted: "the emitted loop leaves the result at its zero value",
		Concepts:  []string{"nil-vs-undef", "comma-ok-idiom"},
	},
	ListUtilQuantifier: {
		Severity:  report.Refuse,
		Message:   "`%s` was given a block that does not end in an expression, so there is no test",
		Short:     "this quantifier block has no test",
		Advice:    "write the loop directly, with an `if` and a `break`",
		Converted: "the expression panics with the original Perl text",
	},
	ListUtilReduce: {
		Severity:  report.Warn,
		Message:   "`reduce` returns undef for an empty list, and the accumulator is an ordinary variable",
		Short:     "an empty list folds to the zero value",
		Advice:    "check the length before folding where the difference matters",
		Converted: "the emitted loop seeds the accumulator with the first element",
		Concepts:  []string{"nil-vs-undef"},
	},
	ListUtilPairs: {
		Severity:  report.Warn,
		Message:   "`pairs` pairs the last element of an odd list with undef, and there is no undef here",
		Short:     "an odd-length list loses its last element",
		Advice:    "check the length first where an odd list is possible",
		Converted: "the emitted loop stops before the unpaired element",
		Concepts:  []string{"nil-vs-undef"},
	},
	MatchPosition: {
		Severity:  report.Note,
		Message:   "`pos` and the `/g` match state live on the scalar in Perl, and on the loop in Go",
		Short:     "/g match state became a loop index",
		Advice:    "iterate `FindAllStringSubmatchIndex`, whose returned offsets are the `pos` equivalent",
		Converted: "the emitted loop keeps the offset in a local variable instead of on the string",
		Concepts:  []string{"submatch-and-named-groups"},
	},
	BindingRightSide: {
		Severity:  report.Refuse,
		Message:   "the right side of `=~` is not a pattern the converter can read at conversion time",
		Short:     "the =~ right side is not a pattern",
		Advice:    "compile the pattern with `regexp.MustCompile` and call its methods directly",
		Converted: "the match is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"mustcompile-pattern"},
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
	HexOctParseError: {
		Severity: report.Warn,
		Message:  "`hex` and `oct` answer 0 for text they cannot read, and `strconv.ParseInt` returns an error",
		Short:    "the hex and oct parse error is dropped",
		Advice:   "check the second result and decide what unreadable text means, rather than taking 0",
		Cost:     "the emitted code discards the error, so text that is not a number still becomes 0",
		Concepts: []string{"errors-are-values", "strconv-parsing"},
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
	ArrayLengthAssignment: {
		Severity:  report.Warn,
		Message:   "assigning to `$#array` sets the length, and a Go reslice cannot pass the capacity",
		Short:     "$#array assignment only shortens",
		Advice:    "append the zero value the required number of times wherever the array has to grow",
		Cost:      "a larger value padded the Perl array with undef, which the reslice cannot do",
		Converted: "the emitted code reslices, which shortens exactly as the Perl did",
		Concepts:  []string{"slices-not-arrays", "slice-aliasing-and-copy"},
	},
	AssignPastEnd: {
		Severity:  report.Warn,
		Message:   "assigning past the end extends a Perl array, and the same write panics on a Go slice",
		Short:     "assignment past the end panics in Go",
		Advice:    "use `append` to add elements, and index assignment only for positions that exist",
		Cost:      "an index Perl filled with undef stops the program instead",
		Converted: "the emitted code writes at the index, which needs the position to be there already",
		Concepts:  []string{"slices-not-arrays"},
	},
	EachIterator: {
		Severity:  report.Warn,
		Message:   "`each` walks a hash through an iterator kept on the hash, and Go keeps no such state",
		Short:     "each keeps hidden iterator state",
		Advice:    "sort the keys and range over those where the visiting order matters",
		Cost:      "a loop that resumed where an earlier one stopped now starts at the beginning",
		Converted: "the pair-at-a-time loop became `for k, v := range m`",
		Concepts:  []string{"map-iteration-order", "range-is-not-foreach"},
	},
	SpliceReturn: {
		Severity:  report.Warn,
		Message:   "`splice` removes, inserts and reports in one call, and Go has a separate function for each",
		Short:     "the removed elements are a copy",
		Advice:    "where the removed part is unused the call stands alone and the copy costs nothing",
		Cost:      "the removed run is copied out before the list is rebuilt around it",
		Converted: "one helper does the whole operation and hands back what it removed",
		Concepts:  []string{"slice-surgery", "slice-aliasing-and-copy"},
	},
	SpliceForm: {
		Severity:  report.Refuse,
		Message:   "this `splice` works on something that did not resolve to a list the generated code can change",
		Short:     "this splice has no list to work on",
		Advice:    "use `slices.Delete`, `slices.Insert` or `slices.Replace` on a slice variable",
		Converted: "the call is not converted; the call site yields the zero value and names the gap",
		Concepts:  []string{"slice-surgery", "slices-not-arrays"},
	},
	SortNamedComparator: {
		Severity:  report.Refuse,
		Message:   "a named `sort` sub reads the globals `$a` and `$b`, and Go passes the two values in",
		Short:     "named sort comparator reads globals",
		Advice:    "make the comparator `func(a, b T) int` and pass it to `slices.SortFunc`",
		Converted: "the sort is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"sort-slice"},
	},
	SortRefComparator: {
		Severity:  report.Warn,
		Message:   "a comparator held in a variable reads `$a` and `$b`, and a Go comparator takes them as arguments",
		Short:     "the comparator is called through a wrapper",
		Advice:    "rewrite the comparators as `func(a, b T) int` so each can be passed directly",
		Cost:      "the two package variables stay, and the wrapper fills them in before each call",
		Converted: "`slices.SortFunc` is given a wrapper that sets the two variables and calls the comparator",
		Concepts:  []string{"sort-slice"},
	},
	SortRefUnresolved: {
		Severity:  report.Refuse,
		Message:   "the comparator this `sort` reads out of a variable did not resolve to something callable",
		Short:     "the comparator did not resolve",
		Advice:    "give every comparator the shape `func(a, b T) int` and pass one directly to `slices.SortFunc`",
		Converted: "the list is copied and left in the order it already had",
		Concepts:  []string{"sort-slice"},
	},
	SortBlockNoOrder: {
		Severity:  report.Refuse,
		Message:   "the `sort` block does not end in an expression that produces an ordering",
		Short:     "sort block produces no ordering",
		Advice:    "end the block with an int expression: negative, zero, or positive",
		Converted: "the sort is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"sort-slice"},
	},
	MapBlockNoValue: {
		Severity:  report.Refuse,
		Message:   "the `map` block does not end in an expression, so there is nothing to collect",
		Short:     "map block collects nothing",
		Advice:    "rewrite it as a loop that appends what the block was meant to produce",
		Converted: "the call is not converted; the call site panics with the original Perl text",
	},
	GrepBlockNoTest: {
		Severity:  report.Refuse,
		Message:   "the `grep` block does not end in an expression, so there is no test to apply",
		Short:     "grep block applies no test",
		Advice:    "rewrite it as a loop with an `if` around what the block was testing",
		Converted: "the call is not converted; the call site panics with the original Perl text",
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
	OpenModePipe: {
		Severity:  report.Refuse,
		Message:   "the `%s` open mode selects a pipe or a duplicated handle, which is a different call in Go",
		Short:     "this open mode is a different call",
		Advice:    "a pipe open becomes `exec.Cmd.StdoutPipe`; a duplicated handle becomes the `*os.File`",
		Converted: "the open is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"os-exec", "io-reader-writer"},
	},
	OpenModeComputed: {
		Severity:  report.Refuse,
		Message:   "the `open` mode is built at run time, so which call it means cannot be decided here",
		Short:     "open mode is not known until run time",
		Advice:    "call the function the mode names, or `os.OpenFile` with the flags built the same way",
		Converted: "the open is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"errors-are-values"},
	},
	OpenUnchecked: {
		Severity:  report.Warn,
		Message:   "this `open` is not checked, and Go will not compile a call that drops the error silently",
		Short:     "an unchecked open now ends the program",
		Advice:    "decide what a missing file means here; reporting it and stopping is the safe default",
		Cost:      "the Perl carried on with an unusable handle, and the emitted program stops at the open",
		Converted: "the emitted code reports the error and exits",
		Concepts:  []string{"errors-are-values", "if-err-nil-rhythm"},
	},
	ReadLineLoop: {
		Severity: report.Note,
		Message:  "`while (<%s>)` became a `bufio.Scanner` with its buffer raised to 1 MiB",
		Short:    "line reads became bufio.Scanner",
		Advice:   "read with `bufio.Reader.ReadString` where a line can exceed 1 MiB; `Scanner` stops with `bufio.ErrTooLong`",
		Cost:     "a line longer than 1 MiB ends the loop with an error, where Perl read it",
		Concepts: []string{"bufio-scanner-limit", "sentinel-and-custom-errors"},
	},
	ReadLineKeepsNewline: {
		Severity:  report.Note,
		Message:   "`readline` keeps the newline on the end of each line, and `bufio.Scanner` strips it",
		Short:     "readline keeps the newline, Scanner does not",
		Advice:    "drop the newline the emitted code puts back wherever the loop body does not want it",
		Converted: "the newline is added back, because the loop body never chomps",
		Concepts:  []string{"strings-are-bytes"},
	},
	SlurpFile: {
		Severity: report.Warn,
		Message:  "`$/ = undef` slurps the whole file, and the emitted code uses `os.ReadFile`",
		Short:    "slurp became os.ReadFile",
		Advice:   "stream with `bufio.Reader` for files that do not fit in memory",
		Cost:     "the whole file is held in memory at once, as it was in Perl",
		Concepts: []string{"io-reader-writer", "bufio-scanner-limit"},
	},
	InputLineNumber: {
		Severity:  report.Warn,
		Message:   "`$.` counts lines globally and follows whichever handle was read last",
		Short:     "the line counter is an ordinary variable",
		Advice:    "give each loop its own counter where two handles are read in turn",
		Cost:      "one variable is shared by every read loop, which matches one loop at a time and not two interleaved reads",
		Converted: "the emitted code keeps a package-level int that each line-reading loop increments",
	},
	OutputFormatVars: {
		Severity:  report.Refuse,
		Message:   "`%s` is a global that changes how `print` and `split` behave, and Go has no such state",
		Short:     "output formatting variable has no Go form",
		Advice:    "pass the separator to the call that needs it; `strings.Join` and `bufio.Writer` take one",
		Converted: "the variable is not converted; the expression panics with the original Perl text",
		Concepts:  []string{"io-reader-writer"},
	},
	SeparatorNotStatic: {
		Severity:  report.Warn,
		Message:   "`%s` is set to a value known only while the program runs, and it has to be known while converting",
		Short:     "separator value is not known here",
		Advice:    "pass the separator to the calls that use it; `strings.Join` and a read with a separator both take one",
		Cost:      "the default separator stays in force in the generated code",
		Converted: "the assignment produced no code and the separator was left alone",
		Concepts:  []string{"small-stdlib-philosophy"},
	},
	SeparatorFolded: {
		Severity:  report.Note,
		Message:   "`%s` is a global other operations read, and its value went into the calls it governs",
		Short:     "separator folded into its calls",
		Advice:    "pass the separator as an argument where a called sub was meant to see the change",
		Cost:      "the change reaches the calls in this block, not a sub called from inside it",
		Converted: "the separator's effect is written into the reads and writes it applies to",
		Concepts:  []string{"small-stdlib-philosophy", "io-reader-writer"},
	},
	LastSystemError: {
		Severity:  report.Refuse,
		Message:   "`$!` holds the error of the last failed system call, and a Go error is returned by the call",
		Short:     "$! has no global in Go",
		Advice:    "use the error the call returned, and wrap it with `fmt.Errorf` and `%w` where it travels",
		Converted: "the variable is not converted; the expression panics with the original Perl text",
		Concepts:  []string{"errors-are-values", "error-wrapping"},
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
	DynamicInvocant: {
		Severity:  report.Warn,
		Message:   "the class of the value was not known, so it is asserted to the type that answers to the method",
		Short:     "the invocant is asserted before the call",
		Advice:    "a type switch answers and hands back the typed value at once where the value really varies",
		Converted: "the call went through a type assertion",
		Concepts:  []string{"type-assertions-and-switches", "implicit-interfaces"},
	},
	IsaPredicate: {
		Severity:  report.Warn,
		Message:   "`->isa(%s)` became a predicate listing the concrete types that inherit from it",
		Short:     "the inheritance test is spelled out",
		Advice:    "embedding is not subtyping, so a new class in the hierarchy has to be added to the predicate too",
		Converted: "the emitted code switches on the value's concrete type",
		Concepts:  []string{"type-assertions-and-switches", "structs-and-embedding"},
	},
	ClassForwarder: {
		Severity:  report.Warn,
		Message:   "a class method that only forwards to `$class->new` is resolved at each call site",
		Short:     "the forwarding class method is inlined",
		Advice:    "a factory that picks a type from a name is a map from string to function in Go",
		Converted: "each call builds the type it named",
		Concepts:  []string{"methods-and-receivers", "compile-time-mindset"},
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
	AccessorField: {
		Severity:  report.Note,
		Message:   "a sub that only reads one hash key became an exported field",
		Short:     "the accessor became a field",
		Advice:    "read and write the field directly; Go adds the method later without changing any caller",
		Converted: "the sub is gone and its callers name the field",
		Concepts:  []string{"methods-and-receivers", "structs-and-embedding"},
	},
	MethodNotFound: {
		Severity: report.Warn,
		Message:  "no `sub %s` was found in the class or in anything it inherits from",
		Short:    "no such method in this file",
		Advice:   "convert the module that declares it too, so the type and its methods land in one package",
		Cost:     "the call has nothing to resolve to and is left as a refusal",
		Concepts: []string{"methods-and-receivers"},
	},
	MethodNeedsObject: {
		Severity: report.Warn,
		Message:  "`%s` reads the fields of an object and was called without one",
		Short:    "an instance method needs a receiver",
		Advice:   "build the object first and call the method on it",
		Concepts: []string{"methods-and-receivers"},
	},
	ConstructorArgs: {
		Severity: report.Warn,
		Message:  "the constructor's arguments are built at run time, so no key can be matched to a parameter",
		Short:    "constructor arguments not matchable",
		Advice:   "pass the values positionally, in the order the generated constructor declares them",
		Cost:     "every parameter is passed its zero value",
		Concepts: []string{"variadic-and-no-defaults"},
	},
	ConstructorArgUnread: {
		Severity: report.Warn,
		Message:  "the call names `%s` and the constructor never reads that key",
		Short:    "an unread constructor argument was dropped",
		Advice:   "remove it from the call, or read it in the constructor so it becomes a field",
		Concepts: []string{"variadic-and-no-defaults"},
	},
	IsaCheck: {
		Severity:  report.Warn,
		Message:   "`->isa(%s)` was answered from the class hierarchy this file declares",
		Short:     "isa decided at conversion time",
		Advice:    "where the value can hold more than one class, give it an interface type and use a type switch",
		Converted: "the emitted code has the constant answer",
		Concepts:  []string{"type-assertions-and-switches", "structs-and-embedding"},
	},
	LateBinding: {
		Severity: report.Warn,
		Message:  "Go resolves `%s` against the embedded parent, where Perl looked it up on the object's real class",
		Short:    "embedding cannot express late binding",
		Advice:   "declare an interface for the methods the base calls on itself, hold it in the base struct, and set it in each constructor",
		Cost:     "the base class's own version runs where Perl would have run the subclass's",
		Concepts: []string{"late-binding-vs-embedding", "implicit-interfaces", "structs-and-embedding"},
	},
	InheritedConstructor: {
		Severity: report.Warn,
		Message:  "`%s` has no constructor of its own and Perl finds the parent's by walking `@ISA`",
		Short:    "the constructor is inherited",
		Advice:   "write a constructor for this type that fills in the embedded parent and returns a pointer to it",
		Concepts: []string{"structs-and-embedding", "late-binding-vs-embedding"},
	},
	ComputedFieldName: {
		Severity: report.Warn,
		Message:  "a blessed hash was given a key worked out at run time, and a struct's fields are fixed",
		Short:    "a computed field name was dropped",
		Advice:   "give the type a map field for the part whose keys vary, and keep the fixed keys as fields",
		Concepts: []string{"structs-and-embedding"},
	},
	AccessorReadOnly: {
		Severity: report.Warn,
		Message:  "`%s` ignores anything passed to it, so the argument does nothing",
		Short:    "this accessor only reads",
		Advice:   "drop the argument, or assign to the field directly",
		Concepts: []string{"methods-and-receivers"},
	},
	Overload: {
		Severity: report.Refuse,
		Message:  "`use overload` makes an operator a method call, and Go has no operator overloading at all",
		Short:    "operator overloading has no equivalent",
		Advice:   "give the type named methods, and implement `fmt.Stringer` in place of the `\"\"` overload",
		Cost:     "`\"\"` does not fire for concatenation even with a Stringer, only for the fmt verbs",
		Concepts: []string{"methods-and-receivers", "fmt-and-verbs"},
	},
	Autoload: {
		Severity: report.Refuse,
		Message:  "`AUTOLOAD` runs for any method the class does not define, and Go resolves method names as it compiles",
		Short:    "AUTOLOAD has no Go equivalent",
		Advice:   "write the methods out, or hold a map from name to function value and index it",
		Cost:     "a call that AUTOLOAD would have caught has nothing to resolve to",
		Concepts: []string{"methods-and-receivers", "implicit-interfaces"},
	},
	ClassAlias: {
		Severity:  report.Note,
		Message:   "`ref($proto) || $proto` picks the class to bless into, and Go has one type here either way",
		Short:     "the class alias disappeared",
		Advice:    "nothing to do: the constructor returns the one type whichever way it was called",
		Converted: "the line is gone",
		Concepts:  []string{"methods-and-receivers"},
	},

	// -- Modules ------------------------------------------------------------

	GetoptBlock: {
		Severity: report.Refuse,
		Message:  "this option block is not a shape the converter could take apart",
		Short:    "the option block was not understood",
		Advice:   "register the options by hand: one `flag` call per name, on a `FlagSet` built with `ContinueOnError`",
		Cost:     "the options are not parsed at all and every destination keeps its default",
		Concepts: []string{"flag-package"},
	},
	GetoptSpec: {
		Severity: report.Warn,
		Message:  "the option specification `%s` is not a shape the converter reads",
		Short:    "an option specification was skipped",
		Advice:   "register that option by hand with the `flag` call that matches its type",
		Cost:     "the option is not registered, so giving it is an error",
		Concepts: []string{"flag-package"},
	},
	GetoptDestination: {
		Severity: report.Warn,
		Message:  "the destination of `%s` is not a variable the registration can write through",
		Short:    "an option destination was skipped",
		Advice:   "give the option a plain variable and do the validation after parsing, where a reader will find it",
		Concepts: []string{"flag-package", "pointers-vs-references"},
	},
	GetoptRepetition: {
		Severity: report.Warn,
		Message:  "`%s` swallowed several words per occurrence, and `flag` hands a value exactly one",
		Short:    "a multi-word option became repeatable",
		Advice:   "callers repeat the option instead: `--pair a --pair b` in place of `--pair a b`",
		Cost:     "the old call form is now an error rather than a longer list",
		Concepts: []string{"flag-package"},
	},
	GetoptOptionalValue: {
		Severity: report.Warn,
		Message:  "`%s` may be written without a value, so `flag` will not let it take the word after it",
		Short:    "a detached optional value is lost",
		Advice:   "rewrite the callers to attach it: `--tag=VALUE` behaves the same either way",
		Cost:     "`--tag VALUE` leaves VALUE among the operands and the tag empty",
		Concepts: []string{"flag-package"},
	},
	GetoptAliases: {
		Severity:  report.Note,
		Message:   "an option with several spellings became one registration per name against one variable",
		Short:     "aliases became separate registrations",
		Advice:    "nothing to do; `flag` has no aliases but a second name on the same destination works",
		Converted: "every spelling is registered and they share a destination",
		Concepts:  []string{"flag-package"},
	},
	GetoptBundling: {
		Severity: report.Warn,
		Message:  "single-letter options run together, as `-abc`, have no counterpart in `flag`",
		Short:    "bundled options no longer parse",
		Advice:   "write the options out separately, or parse with `github.com/spf13/pflag`, which bundles",
		Cost:     "`-vv` becomes an unknown option rather than a count of two",
		Concepts: []string{"flag-package", "go-mod-vs-cpan"},
	},
	GetoptPassThrough: {
		Severity: report.Warn,
		Message:  "an unknown option stops the parse rather than being left among the operands",
		Short:    "unknown options are no longer passed through",
		Advice:   "split the argument list before parsing, or use `github.com/spf13/pflag` and its allowlist",
		Cost:     "a script that forwards arguments to another command stops at the first of them",
		Concepts: []string{"flag-package"},
	},
	GetoptConfigure: {
		Severity: report.Note,
		Message:  "the `flag` package has one behaviour and no settings to change",
		Short:    "a parser setting has nothing to change",
		Advice:   "check what the setting did and whether the emitted block still does what the callers expect",
		Concepts: []string{"flag-package", "small-stdlib-philosophy"},
	},
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
	ModuleFunctionUnmapped: {
		Severity:  report.Refuse,
		Message:   "`%s` has no rule of its own, though the module it comes from is in the mapping table",
		Short:     "module function has no Go mapping",
		Advice:    "write the call by hand; `go doc` over the standard library finds what corresponds",
		Converted: "the call is not converted; the call site panics with the original Perl text",
		Concepts:  []string{"small-stdlib-philosophy", "go-mod-vs-cpan"},
	},
	ParentEmbedded: {
		Severity:  report.Note,
		Message:   "`use parent` became embedding, which promotes the parent's fields and methods",
		Short:     "the parent is embedded",
		Advice:    "nothing to do unless the parent calls a method the child overrides, which embedding cannot reach",
		Converted: "the emitted struct embeds the parent type",
		Concepts:  []string{"structs-and-embedding", "late-binding-vs-embedding"},
	},
	ModuleInlined: {
		Severity:  report.Note,
		Message:   "the module beside this file was converted with it, into the same Go package",
		Short:     "the module came along",
		Advice:    "to split it out again, move its declarations into their own directory and export the names the script uses",
		Converted: "its classes are types here and the names it exported are ordinary functions",
		Concepts:  []string{"packages-and-exported-names", "go-mod-vs-cpan"},
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
	ListUtilMapped: {
		Severity: report.Note,
		Message:  "`List::Util` maps to `slices.Max` and `slices.Min`, and to a plain loop for the rest",
		Short:    "List::Util maps to slices and loops",
		Advice:   "write the loop for `sum`, `first` and `reduce`, which keeps the accumulator in sight",
		Concepts: []string{"small-stdlib-philosophy"},
	},
	DumperMapped: {
		Severity: report.Note,
		Message:  "`Data::Dumper` maps to `fmt` with `%%#v`, and to `encoding/json` when the dump is read back",
		Short:    "Data::Dumper maps to fmt or to json",
		Advice:   "use `json.MarshalIndent` where the output is meant to be read or parsed again",
		Concepts: []string{"encoding-json"},
	},
	ScalarUtilMapped: {
		Severity: report.Note,
		Message:  "`Scalar::Util` asks what a scalar holds, and Go answers most of that at compile time",
		Short:    "Scalar::Util maps to a type switch",
		Advice:   "use a type switch on an interface value for the questions that are left at run time",
		Concepts: []string{"type-assertions-and-switches", "compile-time-mindset"},
	},
	BasenameMapped: {
		Severity: report.Note,
		Message:  "`basename` and `dirname` map to `filepath.Base` and `filepath.Dir`",
		Short:    "File::Basename maps to path/filepath",
		Advice:   "use `path` rather than `path/filepath` for slash-separated names such as URL paths",
	},
	TimeModuleMapped: {
		Severity: report.Note,
		Message:  "the `Time::` modules map to `time`, whose duration is a type rather than a number",
		Short:    "the Time modules map to time",
		Advice:   "write `2 * time.Second` rather than a bare `2`, which the compiler insists on",
		Concepts: []string{"explicit-conversions-no-coercion"},
	},
	FileSpecMapped: {
		Severity: report.Note,
		Message:  "`File::Spec`'s class methods are plain functions in `path/filepath`",
		Short:    "File::Spec maps to path/filepath",
		Advice:   "`catfile` and `catdir` are both `filepath.Join`, which cleans the result as it builds it",
		Concepts: []string{"filepath-and-paths"},
	},
	CwdMapped: {
		Severity: report.Note,
		Message:  "`getcwd` is `os.Getwd` and `abs_path` is `filepath.Abs` followed by `filepath.EvalSymlinks`",
		Short:    "Cwd maps to os and path/filepath",
		Advice:   "handle the error os.Getwd returns: the directory a process is in can be deleted while it runs",
		Concepts: []string{"filepath-and-paths", "errors-are-values"},
	},
	DumperFormat: {
		Severity: report.Warn,
		Message:  "a structure dump comes out as Go syntax from `%#v`, not as source that can be read back",
		Short:    "the dump is Go syntax, not Perl syntax",
		Advice:   "use `encoding/json` where something else has to parse the result",
		Concepts: []string{"fmt-and-verbs", "encoding-json"},
	},
	AbsPathMissing: {
		Severity: report.Warn,
		Message:  "`abs_path` of a path that is not there returns undef, and a Go string has no undef",
		Short:    "a missing path resolves to the empty string",
		Advice:   "compare the result against \"\", or call `os.Stat` first and handle the error",
		Concepts: []string{"nil-vs-undef", "filepath-and-paths"},
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

	EvalBlock: {
		Severity:  report.Warn,
		Message:   "`eval { }` traps a `die` from anywhere inside it, which in Go is `panic` and `recover`",
		Short:     "a trapped failure arrives as a panic",
		Advice:    "return an error from the code inside and check it here, which is what Go code does",
		Cost:      "Go reserves panic for genuine bugs; a function that can fail returns an error instead",
		Converted: "the block runs in a function literal with a deferred `recover`",
		Concepts:  []string{"errors-are-values", "panic-and-recover", "if-err-nil-rhythm"},
	},
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
		Message:   "the generated Go does not parse, which is a bug in perl2golang and not in the input",
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
		Message:   "the generated Go could not be formatted, which is a bug in perl2golang",
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
		Advice:    "start the runtime, or run `perl2golang ai status` to see what is configured",
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
		Advice:   "run `perl2golang ai setup`, which lists the models that fit the free VRAM",
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
