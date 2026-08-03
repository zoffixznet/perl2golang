# Tier 2 corpus - script-shaped Perl programs

38 entries. Each is a self-contained, realistic Perl script of the kind a
sysadmin or data wrangler actually writes, not a snippet.

## Layout of an entry

```
NN-short-slug/
  input.pl          the Perl program (use strict; use warnings; throughout)
  cmd               single line of command-line arguments (may be empty)
  stdin             stdin to feed the program (absent when the program reads none)
  files/            input fixtures referenced by relative path from the entry dir
  expected_stdout   exact expected stdout, byte for byte
  expected_exit     expected exit status, a bare integer
  notes.md          what it exercises, Perl constructs, Go concepts to teach
```

## How to run an entry

From **inside** the entry directory, so relative fixture paths resolve:

```sh
cd NN-short-slug
perl input.pl $(cat cmd) < stdin > got_stdout ; echo $?
```

(omit the redirect when there is no `stdin` file)

## Guarantees

Every `expected_stdout` and `expected_exit` in this tier was produced by
running the script with perl 5.42.2, not written by hand. Every entry was run
twice and the two runs compared byte for byte. All 35 entries:

- produce **nothing on stderr** under `use strict; use warnings;`
- are **deterministic**: no timestamps (entry 34 pins a fixed epoch, UTC and
  the C time locale, because `%A`/`%B` print locale-dependent names),
  no randomness, no unsorted hash iteration reaching the output, no absolute
  paths in the output
- leave the working tree unchanged (entry 20 writes a report file and unlinks
  it again as part of the script)

Two entries exit non-zero on purpose: **27** (65) and **32** (1). The other 33 exit 0.

## Entries

| # | Entry | One line | Constructs covered |
|---|---|---|---|
| 01 | `01-sub-calling-conventions` | Batch-size helper exercising every plain positional calling convention | `@_` unpacking, bare `shift`, defaults via `unless defined`, early return, `return;` as empty list, list vs scalar returns, implicit return, `my $n = () = f()`, statement-modifier `for` |
| 02 | `02-named-args-hash` | Account provisioning with named arguments merged over defaults | `my %args = @_`, `(%defaults, %args)` merge, required-arg `die`, returning a hash vs a hashref, `sort keys %$href`, `50_000` literals |
| 03 | `03-recursion-and-wantarray` | Fibonacci/factorial/mutual recursion plus all three `wantarray` branches | recursion, memoisation, mutual recursion, subs called before declaration, `wantarray` list/scalar/void, tree recursion with depth, `deep_sum` over nested arrayrefs |
| 04 | `04-reference-syntax-tour` | Every dereference spelling Perl offers, side by side | `\$ \@ \%`, `$$r`, `@$r`, `@{$r}`, `$$r[0]`, `${$r}[0]`, `$r->[0]`, `$#$r`, slices `@{$r}[..]` and `@{$h}{..}`, `&$c`, `&{$c}`, refs to refs, `ref()` incl. `REF` |
| 05 | `05-modify-through-refs` | Subs that mutate their caller's data through refs and through `@_` | in-place edit via aliased loop var, `push @$aref`, `@$aref = @new`, **`\@_` aliasing**, `@_[0,1] = @_[1,0]` swap, nested `$cfg->{a}{b} *= n` |
| 06 | `06-array-of-hashes` | Server inventory report from a pipe-delimited file | array of hashrefs, `split /\|/`, three-key sort with `\|\|`, `sprintf` table, `grep` filter, `my ($first) = grep`, `close or die` |
| 07 | `07-hash-of-arrays` | Log triage grouping messages by severity, then inverting the index | `push @{ $h{$k} }, $v`, first-seen order tracking, rank-table sort, structure inversion, `$a[-1]`, list-context match capture |
| 08 | `08-hash-of-hashes` | Host x metric matrix walked row-wise and column-wise | nested hash literals, union of inner keys, comparator indexing the outer hash, third-level derived structure, `exists`/`delete` on nested keys |
| 09 | `09-autovivification-counters` | Access-log roll-up where every counter bucket autovivifies | **autovivification** at 1/2/3 levels, `push @{ $h{$k} }`, `grep { !$seen{$_}++ }`, `split ' '`, read-through-intermediate creating a key |
| 10 | `10-autovivification-tree` | Directory tree grown from flat path strings with no node-creation code | **autovivification via a walking cursor** `$node = \%{ $node->{$p} }`, `undef` as a file sentinel, recursive render, prefix-accumulating totals, `scalar(() = /x/g)` |
| 11 | `11-closures-and-generators` | Counters, a bank account, iterators, and the loop-variable capture question | closures over lexicals, several closures sharing one variable, **`foreach my` fresh binding vs C-style shared**, generator returning `undef`, memoising wrapper |
| 12 | `12-dispatch-table` | A tiny command interpreter driven by a hash of code refs | dispatch table, lookup-then-call with fallback, callbacks as named args, recursive walker dispatching on `ref`, code refs built in a loop, sub returning a sub |
| 13 | `13-map-grep-sort` | List pipelines with multi-statement blocks | `map` returning several elements, `map` returning hashrefs, multi-statement `grep`, **`grep` in scalar context = count**, multi-key sort, Schwartzian transform, named comparator |
| 14 | `14-regex-captures` | Syslog scraper with a large `/x` named-capture pattern | `qr//`, `(?<name>)` and `%+`, optional groups yielding `undef`, `$1..$4` with alternation, greedy vs non-greedy, anchors, `[[:space:]]`, list-context capture |
| 15 | `15-regex-global-match` | `/g` in both contexts, `pos()` as an lvalue, and a `\G` tokeniser | list-context `//g`, `while (//g)` with `pos`, count-of idiom, `pos()=` assignment, `\G` + `/gc` lexer, per-variable match position, `/g` vs `/gc` on failure |
| 16 | `16-substitution-and-tr` | Path munging, a template engine and DNA complementing | `(my $c = $s) =~ s///`, `s///` return count, `/g`, backrefs, **`s///e`** twice, `/r`, `\u\L`, `tr///` translate, **`tr///` counting**, `tr` with `/c /d /s` |
| 17 | `17-regex-modifiers` | Each of `/x /i /m /s` shown with and without the flag | `/m` line anchors, `/i`, `/s` dot-matches-newline, `/x` commented IPv4 validator, `qr//` reuse and interpolation, `/xi` with named captures |
| 18 | `18-split-and-join` | Everything `split` does, plus `join` round trips | trailing-empty-field drop vs `-1` limit, positive limits, **`split ' '` magic** vs `/ /` vs `/\s+/`, **capturing separators**, `split //`, no-match and empty-string cases, query-string parser |
| 19 | `19-read-lines-chomp` | Hosts-file reader with comment/blank skipping | three-arg `open`, lexical handle, `while (my $l = <$fh>)`, `chomp`, list-context slurp, `chomp @array`, `$.`, `$!`, `$0`, `chomp` return value |
| 20 | `20-write-and-append` | Sales report written, appended to, read back and unlinked | `open '>'` and `'>>'`, `print $fh`, `printf {$fh}`, **`close or die`**, `-s`, in-memory handle on `\$scalar`, `unlink`, sub defined after use |
| 21 | `21-slurp-and-paragraphs` | Release notes read three different ways by changing `$/` | **`local $/`** slurp, `do {}` as an expression, paragraph mode `$/ = ''`, custom `$/ = '::'`, handle on a scalar ref, whole-file `s///mge` |
| 22 | `22-file-tests-and-dirs` | Directory listing and recursive descent with file tests | `opendir`/`readdir`/`closedir`, sorted + dot filtering, `-e -f -d -s -r`, the **`_` stat cache**, recursive walk into a hash of arrays |
| 23 | `23-stdin-line-filter` | grep-shaped filter: pattern from `@ARGV`, lines from STDIN | `<STDIN>`, `shift @ARGV`, **runtime `qr/$pattern/` inside `eval`**, invert flag, per-line byte counting, the `scalar(() = split)` trap noted |
| 24 | `24-diamond-argv` | Web-log summariser reading two files through `<>` | **`<>` magic ARGV handle**, `$ARGV`, `$.`, `close ARGV if eof`, `@ARGV` consumed, `$0`, nested autovivified counters, `eval {}` as an expression |
| 25 | `25-argv-manual-parsing` | Hand-rolled option parser: long, short, clustered, `--` | `while (@ARGV) { shift }`, `--k=v`, `--flag`, `-rv` clusters, `-v` counting, `--` terminator, runtime-selected comparator code ref, computed array slice |
| 26 | `26-getopt-long-options` | Metrics report driven by a full Getopt::Long option set | `header!` negatable, `t=i`, `f=s`, `i=s@` array, `rename=s%` hash, `v+` counter, aliases, `bundling`, `GetOptionsFromArray`, `@ARGV` remainder |
| 27 | `27-usage-and-exit-codes` | Argument validation with a distinct exit code per failure (**exits 65**) | `use constant`, usage sub, `$0`, `exit` from several points, dispatch table of reducers, numeric validation via `grep` |
| 28 | `28-word-frequency` | Word counter over STDIN with fully deterministic tie-breaking | set from `qw`, `//g` in list context driving `foreach`, count-then-alpha sort, stopword filter, clamped array slices, `'#' x n` histogram, order-preserving dedupe |
| 29 | `29-csv-report-columns` | Order report: header-driven CSV, two-level grouping, aligned columns | `%idx` name-to-position map, `split` with a limit, implicit numeric coercion, autovivified two-level totals, percentage bars, **a `\G` quoted-CSV parser** |
| 30 | `30-log-state-machine` | Build-log parser with BEGIN/END markers and consistency assertions | explicit state variable, `undef`-when-outside record cursor, per-branch captures, `die` as an assertion, post-loop invariant, flattening `map { @{...} }` |
| 31 | `31-die-and-eval` | Config loader showing every corner of `die`/`eval`/`$@` | `die "...\n"`, `eval {}` + `$@`, catching **runtime** errors (divide by zero), nested eval with rethrow, `warn` via `$SIG{__WARN__}`, **`local $@`**, `$@` cleared by success |
| 32 | `32-error-objects-and-cleanup` | Structured errors, type dispatch and a destructor guard (**exits 1**) | `bless`, `@ISA`, **`die $object`**, `blessed`, `->isa` dispatch, plain hashref errors, wrap-and-rethrow with cause, **`DESTROY` guard** for commit/rollback, `exit` with a code |
| 33 | `33-list-util-toolbox` | Load-average table processed with the List::Util toolbox | `sum`/`sum0` (undef vs 0), `max`/`min`/`maxstr`/`minstr`, `first`, `reduce` (numeric, string and **hashref accumulator**), `any`/`all`/`none`, `uniq` vs `uniqnum`, `pairs` |
| 34 | `34-scalar-util-and-posix` | Type interrogation over 15 values plus fixed-timestamp date formatting | `blessed`, **`reftype` vs `ref`**, `looks_like_number` edge cases, inline `package`/`@ISA`, `floor`/`ceil`/`int`, banker's rounding, `fmod` vs `%`, `strftime` on a pinned UTC epoch |
| 35 | `35-paths-and-dumper` | Path helpers and structure dumping, kept location-independent | `basename`/`dirname`/`fileparse` with a suffix regex, `File::Spec` cat/split, `getcwd`/`abs_path`/`abs2rel`, `Data::Dumper` with `Sortkeys`/`Indent`/`Terse`, `local` on package globals, Dumper-as-deep-compare |
| 36 | `36-class-with-accessors` | The shape most scripts write a class in: constructor, hand-written accessors, chained mutators | `bless` on a hashref, named constructor arguments, accessors, method chaining, a class held in another class, a file-scope `my` every instance shares |
| 37 | `37-getopt-options-hash` | An option block naming every option and its type, with the leftovers in `@ARGV` | `GetOptions` into a hash of defaults, `=i` `=s` `=f` `!` `+` `=s@` `=s%`, aliases, `or die` usage message |
| 38 | `38-do-block-values` | `do BLOCK` as a term, in every shape a script uses it | setup-then-value, `if`/`elsif`/`else` as a term, a block whose value is a list, `EXPR or do {}`, `EXPR and do {}`, `return do {}`, nested blocks |

## Coverage map

- **Subroutines** - 01, 02, 03 (also 12, 25, 27 for code refs as values)
- **References, all deref syntaxes** - 04, 05
- **Nested data structures** - 06, 07, 08, 29, 30
- **Autovivification (load-bearing)** - **09, 10**; incidental in 07, 22, 24, 29
- **Closures** - 11, 12, 25; complex `map`/`grep`/`sort` blocks in 13
- **Regex** - 14, 15, 16, 17, 18; applied in 23, 28, 29, 30
- **File I/O** - 19, 20, 21, 22; STDIN in 23 and 28; `<>` in 24
- **Command line** - 23, 24, 25, 26, 27
- **Text-processing shapes** - 06, 28, 29, 30 (and the filter in 23)
- **Error handling** - 31, 32; `die` as an assertion in 30
- **Standard modules** - 33 (List::Util), 34 (Scalar::Util, POSIX),
  35 (File::Basename, File::Spec, Cwd, Data::Dumper), 26 and 37
  (Getopt::Long), 32 (Scalar::Util)
- **Object shapes** - 36
- **Blocks as terms** - 38; the slurp form of it in 21
