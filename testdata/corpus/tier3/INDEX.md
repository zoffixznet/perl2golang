# Tier 3 corpus: real-world programs

Hard, realistic Perl programs that decide whether the converter is
actually good. Every entry was executed with perl 5.42.2; the
`expected_stdout` / `expected_exit` files are captured output, verified
identical across repeated runs, with empty stderr, under
`use strict; use warnings;`. Only core modules are used.

Entry layout: `input.pl` (plus `.pm` files for multi-file entries),
`cmd` (arguments passed to the program), optional `stdin`, optional
`files/` fixtures, `expected_stdout`, `expected_exit`, `notes.md`.

Difficulty scale: 1 = routine translation, 5 = requires recognizing
intent and making documented semantic decisions.

| # | Entry | Description | Key constructs | Diff | Why |
|---|-------|-------------|----------------|------|-----|
| 01 | `01-inventory-manager` | Warehouse inventory with two collaborating classes, chained mutators, and a formatted report | bless hashref, `ref($proto)\|\|$proto`, class vs instance methods, class-level counter, can/isa, getter-setter arity, eval/die | 3 | Dual-mode constructor and runtime class/instance distinction have no direct Go shape |
| 02 | `02-shape-hierarchy` | Abstract Shape base with Rectangle/Square/Circle across four `.pm` files | `use lib '.'`, `use parent`, `our @ISA`, two-level SUPER::, abstract-method dies, template-method new/init, runtime-defined subclass | 4 | Base-class constructor dispatching virtually into subclass init inverts under Go embedding |
| 03 | `03-vector-overload` | 3-D vector math on a blessed arrayref with heavy operator overloading | `use overload` (`+ - * == != <=> "" bool neg`), swapped operands, type-polymorphic `*` (scale vs dot), sort via overloaded `<=>` | 4 | Every operator use site must be found and rewritten; `*` returns different types by operand |
| 04 | `04-textutil-module` | Text utility module pair with export tags and call accounting | Exporter, `@EXPORT_OK`, `%EXPORT_TAGS`, nested package in subdir, `__PACKAGE__`, `use constant`, `tr///s`, `\u\L` case folding, lookahead commify, external read of `our %CALLS` | 3 | Import-resolution bookkeeping plus two regex idioms RE2 cannot express directly |
| 05 | `05-json-pipeline` | Orders JSON to per-customer rollup, canonical pretty JSON out | JSON::PP canonical/pretty, JSON booleans, `//=` slot init, nested hashref/arrayref traversal, float normalisation, `(sort)[0..1]` | 3 | Byte-exact JSON::PP formatting and Perl number stringification (`22.5`, `0`) fight Go's marshaler |
| 06 | `06-serialize-roundtrip` | Storable freeze/thaw/dclone/store, MD5 content ids, binary Base64 | Storable, hand-rolled deep-equal, deep-copy isolation proof, `md5_hex`/`md5_base64`, unpadded Base64, `\x00`-laden strings | 3 | No Storable in Go: converter must preserve observable round-trip/deep-copy semantics, not bytes |
| 07 | `07-log-analyzer` | Access-log summary with CLI flags and runtime-computed column widths | Getopt::Long (hyphenated + negatable opts), `/x` regex, `or ++$x, next`, autovivified 2-level counters, `%-*s`/`%*d`, `splice`, `%ENV` default | 3 | Comma-operator loop control and star-width printf are quietly dropped by naive translators |
| 08 | `08-csv-report` | Hand-rolled CSV state machine handling quotes, `""`, embedded commas and newlines | char-by-char parser, `$row[-1] .=`, hash slice `@rec{@$header}`, grouping, minimal-quoting emitter, round-trip check | 3 | Whole-file (not per-line) parsing unit plus negative-index append trip line-oriented converters |
| 09 | `09-mini-template` | Mustache-ish engine: compile to node tree, render with context frames | capture-keeping `split`, stack-built AST, recursive render, filter coderef table, dotted-path lookup, Perl truthiness for `#if` | 4 | Heterogeneous AST + `interface{}`-style lookup + "0" falsiness demand real design decisions |
| 10 | `10-ini-config` | INI parser: DEFAULT layer, `extends` inheritance, `${var}` interpolation | hash-spread merging, recursion with `$seen`, `\Q\E`, `$.`, typed getters, probe table of coderefs | 4 | Expected output pins eager parent-resolution ordering; lazy-interpolation "fixes" are wrong |
| 11 | `11-fixed-width` | Bank-ledger fixed-width reader/writer with trailer validation | `pack`/`unpack` (`a` vs `A`), nested unpack, zero-padded numifies, integer cents, byte-identical re-emission | 3 | pack/unpack templates must become offset slicing with exact trim semantics, verified by byte count |
| 12 | `12-tag-stripper` | HTML-to-text with comments, script/style, `>`-in-attribute hazards | backreference `</\1>`, `/e` substitutions, alternation-with-quotes tag regex, `while (//g)`, harvest-then-strip pipeline | 4 | The load-bearing backreference is inexpressible in RE2; converter must restructure the algorithm |
| 13 | `13-fs-walker` | Directory auditor over a fixture tree with pruning and rollups | File::Find (`preprocess` sort, `prune`, `_` stat cache, `$File::Find::name`), File::Spec/`fileparse`, lookahead prune regex | 3 | Callback-and-globals traversal maps awkwardly onto WalkDir; `canonpath` non-collapse is a fidelity trap |
| 14 | `14-workspace-builder` | Builds a skeleton in a File::Temp sandbox, audits it, probes error paths | tempdir/tempfile templates, make_path count/idempotence/error-collector, `-z -x` tests, nondeterministic-name filtering | 3 | make_path's three observable behaviors and tempfile templates all differ from the os package |
| 15 | `15-subprocess-capture` | Process control using `$^X` only: backticks, both pipe opens, system | backticks scalar/list, `open '-\|'`/`'\|-'` list form, `$? >> 8`/signal/core bits, die=255 child, stderr-only capture, `$?` persistence | 4 | Four constructs share the `$?` global and split across shell/no-shell exec semantics |
| 16 | `16-env-and-handlers` | Deployment preflight: pinned clock, scoped env, handler taxonomy | `local $ENV{}` nested 3 deep, `$SIG{__WARN__}` collector, `%SIG` coderef/'IGNORE'/'DEFAULT', gmtime/strftime, computed exit | 3 | Nested dynamic env scoping needs exact save/restore ordering Go never provides for free |
| 17 | `17-error-hierarchy` | Blessed-hashref exception classes, rethrow with context, DESTROY during unwind | error hierarchy via @ISA, isa-ladder catch, string-to-object promotion, nested eval fallback, `local $@` guard in DESTROY | 4 | `$@`'s string/object duality and unwind-time cleanup map onto errors.As/defer only with care |
| 18 | `18-expr-eval` | Recursive descent calculator with variables over stdin | `\G`/`pos()`/`/gc` lexer, closure-shared parser state, right-assoc `^`, Perl `%` on floats, `%g` formatting, per-line eval recovery | 4 | Incremental regex lexing and closure-state parsing must be re-architected, not transliterated |
| 19 | `19-build-scheduler` | Toposort with priority tie-break, 2-worker simulation, cycle detection | inline PQueue package (hash-of-arrays), Kahn with pinned sorts, `--$x == 0`, memoized `//=` DFS, graph copy, die/eval cycle report | 4 | FIFO-within-priority and engineered sort determinism are semantics randomized Go maps love to break |
| 20 | `20-route-planner` | Dijkstra transit planner, mixed directed edges, reachability report | `s///` return as flag, hash-of-hashes graph, deterministic extract-min, prev-chain unshift walk, `($cost, @path)` returns | 3 | Mixed scalar+list returns and empty-return unreachability signalling mangle easily |
| 21 | `21-fuzzy-match` | CLI "did you mean": Text::Abbrev, Levenshtein, LCS ratio | rolling-row DP, single-row LCS with `$diag`, hashref record sort on 3 keys, slice-past-end tolerance, abbrev semantics | 3 | Text::Abbrev must be reimplemented to spec and `@x[0..2]` past-end slicing panics in Go |
| 22 | `22-vending-fsm` | Vending machine FSM: coderef transition table, three failure channels | hash-of-hashes of coderefs, absence-as-illegal, die/eval per event, `()` optional arg, audit log, package-level machine state | 3 | Distinguishing illegal/rejected/refused paths through one dispatch table resists mechanical mapping |
| 23 | `23-perl-idioms` | Grab bag: the awkward constructs real code actually contains | `local` on `our` global, `@_` aliasing mutations, `wantarray`, `goto &sub`, do-block expressions, until, `%*d`, `%vd` v-strings, kv/negative/computed slices, closure-memoized fib | 5 | Aliasing, dynamic scope, context sensitivity, and v-strings each require semantic (not syntactic) translation |
| 24 | `24-autoload-accessors` | Accessors materialized on first call via AUTOLOAD + symbol-table writes | `our $AUTOLOAD`, DESTROY guard, `no strict 'refs'` glob assignment, can() flipping, SUPER:: into generated method, `$obj->$field` | 5 | Runtime method synthesis is impossible in Go; converter must recognize the pattern and document the compromise |
| 25 | `25-wrap-and-diff` | Word wrap with hyphenation + LCS traceback diff and similarity score | greedy wrap with loop-var mutation, hanging indent via index-as-boolean map, 2-D DP table, `$a[--$i]` traceback, arrays in numeric context | 3 | Pre-decrement-in-index evaluation order and the diff tie-break must survive translation exactly |
| 26 | `26-class-inheritance` | Two levels of `@ISA`, `SUPER::` extending a method, and a template method that does not convert | `@ISA`, `SUPER::`, template method calling an overridden step, per-class defaults | 4 | Embedding covers the inheritance but not the base class calling back into the subclass |
| 27 | `27-getopt-bundling` | The three option forms `flag` has no answer for | `-vvq` bundling, an unknown option left in `@ARGV`, `:s` with an optional value | 4 | Each is a parser behaviour rather than a spelling, so none of them has a flag equivalent |
| 28 | `28-record-separators` | Every mode of `$/`, including one only known while the program runs | default line reads, `local $/` slurp through a sub, paragraph mode, a literal separator, a separator read out of a config file | 3 | The first four fold into the call that reads; the fifth cannot, and has to degrade honestly |

## Notes for harness authors

- `cmd` holds the arguments passed to the program, e.g.
  `files/access.log`. The script name is not in it. Run
  `perl input.pl ARGS...` from inside the entry directory.
- Entries 18, 21, 22 read `stdin`; all others take no standard input.
- Entry 16 exits per its own logic (currently 0); all entries currently
  expect exit 0, but `expected_exit` is authoritative.
- Entries 06, 14, 15 create files only under a `File::Temp` tempdir and
  never print machine-specific paths; entry 15 shells out only to `$^X`.
- Scripts that conceptually need "now" (entry 16) use a pinned epoch, so
  runs are reproducible. Entry 16 also clears any inherited `DEPLOY_*`
  variables before reading its configuration, so an operator's shell
  cannot change its output.
