# Tier 4 corpus — adversarial entries

Tier 4 entries verify the converter FAILS HONESTLY. An entry passes when the
tool says the right true thing about it — a precise diagnostic, a correct
report entry, a specific TODO — not when it emits Go. Each entry directory
contains `input.pl`, `notes.md` (the behavioural specification), and
`expectation.md` (pass criteria). Where the Perl has a runnable meaning,
`expected_stdout` and `expected_exit` were captured by running the real perl
(5.42.2, x86_64-linux). Two entries need a footnote: `25-hash-order` prints
hash keys in per-process random order on purpose, so it has an
`expected_exit` but no `expected_stdout` and is checked against invariants
instead; `35-buffering` writes to both streams on purpose (hence its
`allow_stderr` marker), and its `expected_stdout` holds stdout only — the
merged-stream order that makes the entry interesting is described in its
notes and checked by replaying the program under `2>&1`.

## Category vocabulary (used in every expectation.md)

- `refuse-file` — decline the whole file; diagnostic explains why.
- `refuse-statement` — convert the rest; replace the construct with a stub
  that panics if reached; diagnostic per site.
- `todo` — emit compilable Go plus a `// TODO:` comment at the site and a
  report entry; behaviour knowingly diverges until a human acts.
- `shim` — emit/use a runtime helper reproducing Perl semantics exactly;
  report entry notes the shim.
- `approximate` — convert with a documented, reported semantic difference.
- `convert-verify` — full conversion expected; passes only if the built
  program reproduces `expected_stdout`/`expected_exit` (Group C).

## Entries

### Group A — genuinely impossible without an interpreter

| Entry | Construct | The tool must... |
|---|---|---|
| 01-string-eval | `eval EXPR` of a computed string | refuse both eval statements with panicking stubs and per-site diagnostics; never guess the evaluated code. |
| 02-symbolic-refs | `$$name`, `${computed}`, `&{"sub_$x"}()` | refuse each symbolic lookup unless it proves the name set closed and generates an explicit dispatch map. |
| 03-glob-aliasing | `*glob = \@array` / `\&sub` / `sub {}` | refuse glob assignments (or lower unconditional top-level ones to true sharing — never copies) and diagnose each site. |
| 04-monkey-patch | `*Other::method = sub {...}` at runtime | refuse the patch site so later calls cannot silently use the original implementation. |
| 05-begin-parse | BEGIN block choosing a prototype from %ENV | refuse the FILE: the parse of later lines depends on compile-time execution; no single AST exists. |
| 06-prototypes | `(&@)`, `(\@\@)`, `($)` altering call parsing | honour prototypes when parsing (proving `one=3`) or refuse any file containing one; never mis-parse quietly. |
| 07-autoload | AUTOLOAD inventing methods from names | refuse calls to nonexistent methods with diagnostics naming AUTOLOAD, or statically specialize the enumerable call set. |
| 08-tie | tied scalar: reads run FETCH, writes STORE | refuse the tie AND poison every later use of the variable; plain-read conversion of a tied variable is the canonical fail. |
| 09-local-special-var | `local $"`, `$,`, `$\` changing distant subs | emit TODOs naming the affected subs and the concrete output divergence, or thread a punctuation-variable shim. |
| 10-format-write | `format`/`write` report DSL | refuse formats and `write` with stubs (or lower simple pictures to printf with byte-identical output). |
| 11-destroy-timing | DESTROY at refcount-zero instants | convert to explicit Destroy() calls at statically known death points; finalizers are forbidden. |
| 12-wantarray | context-polymorphic returns | refuse the sub, or split into per-context variants with context propagated through wrappers, documented per call site. |

### Group B — convertible only with a semantics-changing approximation

| Entry | Construct | The tool must... |
|---|---|---|
| 13-deep-autoviv | reads/sub-args vivifying intermediate levels | use a vivifying-accessor shim so later `exists` agrees with Perl; report each vivification site. |
| 14-args-aliasing | `@_` writes mutating the caller | detect @_ mutation and convert with pointer parameters (rewriting call sites), or refuse the sub; value-copy conversion without diagnostic is the fail. |
| 15-local-dynamic | `local $global` dynamic scoping | emit save/defer-restore on the global (die-safe); never a fresh lexical. |
| 16-goto-sub | `goto &sub` frame replacement | approximate as a tail call and, because this file also uses caller(), state the observable stack divergence. |
| 17-overload | overloaded `+ * "" ==` | lower operator sites to method calls where types are proven; refuse statements whose operands might or might not be objects. |
| 18-mro-c3 | diamond inheritance, DFS vs C3 | precompute each class's linearization with the right algorithm per class (D: DFS = A wins; D3: C3 = C wins) and flatten; no Go embedding. |
| 19-regex-nonre2 | `\1`, `(?=)`, `(?<=)`, `(?>)` | switch those patterns to a PCRE-class engine or prove a rewrite equivalent; never emit a "cleaned" RE2 pattern. |
| 20-regex-code-recursion | `(?&grp)` recursion, `(?{code})`, `(*FAIL)` | refuse: recursion is non-regular, embedded code cannot run mid-match; side-effect counts (1, sum 60) pin the semantics. |
| 21-string-increment | magic `++`/ranges on strings | emit a StrInc shim matching the carrying table (az→ba, zz→aaa, Zz→AAa, z9→aa0); numeric ++ fallback must be diagnosed, not silent. |
| 22-int-overflow-float | IV→UV→NV silent promotion | declare a numeric model; either checked-arithmetic shim or per-site overflow diagnostics; silent int64 wrap is the fail. |
| 23-sprintf-formats | `%vd`, `%s`-of-float (%.15g), `%#b`, `%d` clamp | route formatting through Perl-semantics helpers (floats stringify as %.15g everywhere); refuse `%vd` on runtime strings. |
| 24-flipflop | scalar `..` stateful toggle with E0 values | generate independent per-site state (flag + counter, E0 suffix when the value is observed); never a range. |

### Group C — convertible, but the naive conversion is subtly wrong

| Entry | Construct | The tool must... |
|---|---|---|
| 25-hash-order | key order escaping into output | warn on order-dependent output; keep within-run stability of repeated iteration; invariant check (no expected_stdout). |
| 26-sort-stability | ties relying on stable sort | always emit stable sort (SliceStable) and reproduce tie order byte-for-byte. |
| 27-negative-modulus | `%` sign rules differ from Go | emit a Perl-semantics mod helper (-7%3=2, -1%5=4) unless operands are provably non-negative. |
| 28-int-division | `/` always float; `use integer` scopes | float division everywhere except inside tracked `use integer` scopes; avg = 3.5, not 3. |
| 29-str2num-coercion | `"3abc"`, `"0x10"`, `" 12 "` coercion | implement longest-prefix numification (0x10→0, 010→10, junk→prefix, never error); no strconv-with-ignored-errors. |
| 30-negative-index | negative indices/length, tolerant OOB | index/substr helpers with Perl rules; no panics, no clamping to 0; `substr($s,1,-1)` = "ell". |
| 31-array-autoextend | store past end; `$#a` truncate/extend | extend with real undefs (defined? no), truncation without value resurrection; no append-based growth. |
| 32-undef-context | undef as 0/""/false; "00" is true | distinct undef state plus Perl truthiness helper ("" and "0" false, "0E0" true). |
| 33-return-vs-undef | empty list vs one-element undef | preserve list arity: 0 vs 1 elements, and the shifted hash keys `30,name`. |
| 34-open-unchecked | open failure ignored, exit 0 | convert faithfully (no inserted fatal, no nil-handle panic, errno text preserved) AND warn that the failure is unhandled. |
| 35-buffering | stdout buffered, stderr not, under 2>&1 | declare its I/O model; reproduce merged-stream order (err before out) or report the interleaving change; never lose unflushed output. |
| 36-untyped-nesting | a structure whose shape only the data knows | say the element types did not resolve, and still run to the last line: a wrong guess about a value leaves an empty collection rather than a stack trace. |
| 37-pack-esoteric | `%` checksum fold, BER `w`, `l>` byte-order modifier | refuse at each call, at conversion time, naming the template code; the statements around the templates keep converting. |

## Priority ranking — what matters for real scripts

Invest effort top-down. "Silently wrong" beats "loudly impossible": the
constructs that compile-and-misbehave cost users the most.

**Tier 1 — will bite ordinary scripts silently (do these first).**
Group C's arithmetic/coercion cluster and the two aliasing entries:
27-negative-modulus, 28-int-division, 29-str2num-coercion, 32-undef-context,
14-args-aliasing, 33-return-vs-undef, 30-negative-index, 31-array-autoextend,
26-sort-stability, 25-hash-order, 23-sprintf-formats (the %s-of-float rule
alone touches nearly every numeric print). These appear in ordinary
workaday Perl, produce no crash, and diff only in output values.

**Tier 2 — common constructs needing a correct mechanical strategy.**
34-open-unchecked, 13-deep-autoviv, 15-local-dynamic, 19-regex-nonre2
(lookaround/backrefs are everywhere), 24-flipflop, 21-string-increment,
35-buffering, 11-destroy-timing (guard objects are pervasive in OO Perl),
12-wantarray (context-polymorphic returns are a standard module idiom).

**Tier 3 — must be DETECTED reliably, conversion can wait.**
01-string-eval, 02-symbolic-refs, 07-autoload, 17-overload, 03-glob-aliasing,
04-monkey-patch, 18-mro-c3, 08-tie. Real code containing these is usually
framework/plugin machinery; the converter's job is a crisp refusal that names
the construct and the manual rewrite.

**Curiosities — low frequency, keep as honesty tests.**
05-begin-parse (parse-time environment dependence), 06-prototypes
(parse-altering prototypes beyond `(&@)`), 10-format-write, 16-goto-sub,
20-regex-code-recursion, 09-local-special-var. Rare in the wild, but each is
a cheap check that the tool refuses rather than guesses.
