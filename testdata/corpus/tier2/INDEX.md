# Tier 2 corpus - script-shaped Perl programs

112 entries. Each is a self-contained, realistic Perl script of the kind a
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
twice and the two runs compared byte for byte. Every entry:

- produce **nothing on stderr** under `use strict; use warnings;`
- are **deterministic**: no timestamps (entry 34 pins a fixed epoch, UTC and
  the C time locale, because `%A`/`%B` print locale-dependent names),
  no randomness, no unsorted hash iteration reaching the output, no absolute
  paths in the output
- leave the working tree unchanged (entry 20 writes a report file and unlinks
  it again as part of the script)

Two entries exit non-zero on purpose: **27** (65) and **32** (1). The rest exit 0.

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
| 39 | `39-numeric-accumulators` | Totals built out of text fields, and every compound arithmetic operator | `+=` over split fields, `/=`, `*=`, `-=`, `%=`, `**=`, `.=` on text, `++` on a hash element |
| 40 | `40-flattened-arguments` | Argument lists that flatten arrays into `@_`, in every mixture | one array, single values, a value in front of an array, two arrays, interleaved, a fixed parameter plus a tail, an empty array |
| 41 | `41-list-surgery` | A work queue edited with splice, hash slices and each | `splice` remove/insert/replace/truncate, negative offset and length, splice through a hashref, `@h{qw(...)} = (...)`, `each`, lvalue `substr`, a ternary as an assignment target |
| 42 | `42-nested-structures` | Hash of arrays, hash of hashes, arrays of records, and a comparator taken out of a hash | autovivified `push @{ $h{$k} }`, `$h{$a}{$b} = $v`, per-key totals, records returned from a sub, `sort $cmp @list`, `(my $copy = $orig) =~ s///`, `scalar @{ $ref }` |
| 43 | `43-portable-paths` | File::Spec's class methods and the Cwd pair, kept location-independent | `catfile`/`catdir` including `catfile(@parts)`, `splitpath` returning three values, `splitdir`, `canonpath` (which does **not** resolve `..`), `file_name_is_absolute`, `rel2abs`/`abs2rel`, `getcwd`, `basename` with literal suffixes |
| 44 | `44-temp-trees` | File::Temp and File::Path, with the neighbours still refused | `tempdir(CLEANUP => 1)`, `tempfile(DIR/SUFFIX)` returning a handle **and** a name, `make_path` returning what it created, `remove_tree` returning a count, `opendir`/`readdir` |
| 45 | `45-record-structs` | Hash references used as records, which become structs | a constructor sub returning a hashref, a field added after construction, `push @{ $r->{notes} }`, sorting by a field, `||= { ... }`, a record inside a record, `@{ $r }{qw(...)}`, a field named by a variable |
| 46 | `46-record-tables` | The record shapes deliberately left as maps | a **named** `%hash` used as a record, `keys`/`values`/`exists`/`delete` asked of a record, copying key by key |
| 47 | `47-time-formatting` | Taking a timestamp apart and formatting it, plus run-time type questions | `gmtime` in list context with its zero-based month and 1900-based year, an array slice flattened into `printf`, `strftime` formats that map onto a Go layout and ones that do not, `reftype`, `looks_like_number`, `blessed` |
| 48 | `48-time-arithmetic` | Time as a quantity, which does not convert yet | `$end - $start` as a plain number, divmod into h/m/s, rounding down with `% 3600`, **`timegm`**, stepping a whole month by bumping the month field, comparing moments numerically |
| 49 | `49-tree-walk` | Building, walking and removing a directory tree | `tempdir`/`tempfile`, `print {$fh}`, `make_path`, `File::Spec->catfile` with a list in the middle, `find(sub {...})` with `$File::Find::name`, `$File::Find::prune`, `remove_tree` |
| 50 | `50-tree-permissions` | What a script asks about a file, which does not convert yet | `stat` read by position, `chmod 0640` and the mode read back with `& 07777`, `-r`/`-w`/`-x`/`-f`/`-l`/`-e`/`-s`, `symlink`/`readlink`, `utime`, `rename`, `unlink` |
| 51 | `51-transliteration` | `tr///` and its four modifiers | plain replacement, `tr/ACGT//` counting, `c` complement, `d` delete, `s` squash, `r` non-destructive, ranges on both sides, `( my $copy = $orig ) =~ tr/.../.../` |
| 52 | `52-json-and-digests` | JSON, checksums and base64: the three ways a script turns data into bytes | `JSON::PP->new->canonical(1)` reused, `JSON::PP::true` inside a structure, an encode/decode round trip, `md5_hex`/`md5_base64`, `Digest::MD5->new` with two `add` calls, `encode_base64` and its 76-column wrapping |
| 53 | `53-json-into-shapes` | The JSON edges an untyped decode cannot keep | integers past 2**53, `undef` vs missing key, booleans, empty array vs empty object, pretty-printing |
| 54 | `54-callback-tables` | The three shapes a script puts a function in a slot | a hash of anonymous subs with different arities and return kinds, calling through the table by literal key and by loop variable, an array of subs applied as a pipeline, a closure made inside a loop |
| 55 | `55-callback-context` | The context a callback's caller cannot ask for | a callback returning a list, `wantarray` inside one, the same callback used for one value and for many |
| 56 | `56-file-metadata` | Asking a file about itself, and changing the answers | `( stat $file )[2]` and `[8, 9]`, list slices of `stat`, `chmod` with an octal mode, `-f`/`-s`/`-w`/`-e`/`-l`, `utime` with `timegm`, `symlink`/`readlink`, `rename`, `unlink` |
| 57 | `57-process-and-shell` | Running another program, which does not convert yet | `system`, backticks in scalar and list context, `open` on a pipe both ways, `$?` decoded into status and signal, an argument with a space in it |
| 58 | `58-captures-in-list-context` | A match read for its captures rather than for its truth | `my ($x) = $s =~ /(...)/`, several groups at once, a failed match yielding undef, `/g` in list context, a match with no groups, the whole thing as an `if` condition |
| 59 | `59-captures-through-a-sub` | Captures that leave the sub that made them, which does not convert yet | `return $text =~ /(..)(..)/`, the result taken as a list, as a count and as a truth value |
| 60 | `60-option-pairs-and-bundling` | An option block written as pairs, with bundling and pass-through | `GetOptions` in pair form where the hash key is the destination's, `Configure('bundling', 'pass_through')`, `+` `=i` `:s` `=s@` `=s%`, `defined` on an option, `@ARGV` after the block |
| 61 | `61-option-callbacks-and-abbrev` | Option callbacks and abbreviation, which do not convert yet | a `sub` as a destination, the `<>` operand catch-all, unique-prefix abbreviation, a mixed block where only some destinations are hash elements |
| 62 | `62-counting-hashes` | The counting hash, at one, two and three levels | `$h{$a}{$b}{$c}++`, `+=` on a fractional leaf, `keys %{ $h{$k} }`, `scalar keys` of an inner hash, `push @{ $h{$k} }` beside it, sorted walks at every level |
| 63 | `63-nested-mixed-and-read-autoviv` | Nested hashes that are records, and read-autovivification, neither of which converts yet | literal keys with mixed value kinds under one key, `push` into a nested list field, a nested read that creates the level above it |
| 64 | `64-code-refs-called` | Calling through a code reference, in every shape a script does it | a factory returning several closures over one variable, `$code->(@array)` and the mixed forms, a hash of code refs read by variable key, `$h{$k} \|\| sub {...}` as a fallback |
| 65 | `65-recursive-closures` | Closures that take records and closures that call themselves, neither of which converts yet | a ternary of comparators reading `$_[0]{field}`, `my $f; $f = sub { ... $f->(...) }`, a table whose members call each other through it |
| 66 | `66-reads-past-the-end` | Reading and writing an array outside its range, which does not convert yet | `$a[99]` and `$empty[0]`, `$a[6] = ...` growing the array, `$a[-1]` on both sides of an assignment, `$lines[-1] .= ...`, `$#a = 2` as a place |
| 67 | `67-running-a-program` | Running another program, in every shape a script does it | `system LIST` for its value and for `$?`, backticks in scalar and list context, an argument with a space in it passed whole, `open '-|'` and `open '\|-'` with `close` |
| 68 | `68-child-status-and-forks` | The process half of running a program, which does not convert yet | `$?` after a pipe close, `2>&1` and `2>/dev/null` inside a command, `fork` and `waitpid`, `$^X` |
| 69 | `69-flattening-lists` | Lists are flat, and that rule decides how many results half the idioms produce | `map` blocks whose value is a list, a dereferenced array or a repetition, `( $_ ) x 2` versus `"ab" x 2`, two hashes spliced into one literal, an array in the middle of a list |
| 70 | `70-hash-slices-as-places` | A hash slice on the left of an assignment or inside a delete, which does not convert yet | `@h{@keys} = @vals`, a literal key slice, a short right-hand side leaving undefs, `delete @h{...}` returning the removed values |
| 71 | `71-optional-values-in-a-hash` | A settings table where "unset" and "zero" are different answers | a key set to 0, a key set to undef, a key never mentioned, `exists` against `defined`, `//` and `||` side by side |
| 72 | `72-undef-through-a-sub` | The same absence crossing a sub boundary, which does not convert yet | undef returned from a sub, passed as an argument, and stored in a container it does not otherwise widen |
| 73 | `73-growing-an-array-by-writing` | An array that stretches to fit whatever is written into it | a write past the end at a literal index, the gap it opens holding undef, `$#a` after the growth |
| 74 | `74-a-computed-index-past-the-end` | The same stretching with an index the converter cannot see | `+=` at an index from a modulus, `defined` over a computed hole, a read past the end in a loop |
| 75 | `75-case-folding-and-splicing` | What a replacement template cannot say, and a call that edits its argument | `\U` and `\E` in a replacement, `substr` as a place, four-argument `substr` |
| 76 | `76-captures-after-the-match` | Capture variables that outlive the match, which does not convert | `$1` read after the block that matched, a failed match leaving the last answer standing, `` $` `` and `$'` |
| 77 | `77-draining-a-worklist` | Emptying a list, where the test is "was there one" and not "was it true" | `while (defined(my $job = shift @queue))`, a queue holding 0, the truth-test version beside it |
| 78 | `78-a-lookup-that-finds-nothing` | A lookup that can fail, which does not convert | a key absent from the hash rather than set to undef, `exists`, a default supplied at the read |
| 79 | `79-a-scanning-tokeniser` | A hand-written lexer, where the position lives on the string | `\G` with `/g` in scalar context, `pos`, alternation over token kinds |
| 80 | `80-a-scan-anchor-in-the-middle` | The scan anchor where it cannot be an anchor, which does not convert | `\G` away from the start of the pattern, and `\G` without `/g` |
| 81 | `81-a-one-line-constructor` | A sub's value is whatever it evaluated last, in the four shapes ordinary code writes | `sub new { bless {...}, shift }`, a `$_[0]{field}` accessor, a sub whose value is a call, a sub returning nothing assigned to a scalar, `map { KEY => VALUE }` needing per-element setup |
| 82 | `82-a-table-built-by-index` | An array's length, in the three places Perl never mentions it | `$d[$i][$j]` building a table with no declaration, `0 .. @a` and `0 .. $#a` as ranges, `$a[$i-1]` counting from one, a read past the end of the shorter list |
| 83 | `83-a-strip-that-keeps-what-it-took` | A substitution used as a condition, with the groups read in the branch | `if ($line =~ s/^(\S+)\s+//) { $owner = $1 }`, two groups read after the edit, `push` through a two-level autovivified hash, `split ' ', $line, 2` |
| 84 | `84-captures-in-the-caller-s-context` | The same sub read in three contexts, and one that asks which, neither of which fully converts | a sub returning a match read as a list, as a test and as a scalar, `wantarray ? @got : scalar @got` |
| 85 | `85-a-record-split-into-slices` | A header row zipped against a data row, which is what a hash slice is for | `@rec{@header} = @fields` with a run-time key list and a short value list, a slice on both sides, `delete @h{@keys}` read for its value |
| 86 | `86-an-array-slice-as-a-place` | The same construct over an array, which does not convert | computed indices on the left, a write past the end through a slice, holes at named and unnamed indices, a swap through overlapping slices |
| 87 | `87-whole-number-arithmetic` | Dividing bytes into pages, with and without `use integer` | the pragma over a loop and a nested block, `/` and `%` on the same operands inside and outside it, the round-up idiom, `int($n/$d)` with a negative numerator |
| 88 | `88-a-header-then-the-rest` | A journal with a fixed header, a line body, and a footer read from the end | `read` into a `my` scalar, `tell` between reads, `seek` with whence 2 and a negative offset, `seek ... or die "...: $!"`, a line loop after the seek back |
| 89 | `89-a-position-under-a-line-reader` | The positioning corners that do not convert yet | `tell` between line reads, where a buffered reader has walked ahead, and four-argument `read`, which patches bytes into the middle of its target |
| 90 | `90-a-config-hash-is-a-record` | A named hash whose keys are written out and whose values differ in kind | static reads and writes, a key added after the initialiser, `sort keys`, a field picked from a written-out set at run time, an arrayref field |
| 91 | `91-a-config-hash-with-defaults` | The same shape leaning on undef, which keeps it a map | `//` defaults over a stored 0, `defined` and `exists` probes, a key that is only sometimes there |
| 92 | `92-reading-the-argument-list` | @_ read without being named, in every form that only reads | `@_[0, 1]`, `scalar @_`, `$_[2] //` past the end, `\@_` into a read-only loop |
| 93 | `93-list-shapes-through-subs` | Values changing shape at boundaries: flat returns, eval in list context, a sub ending in //= | `return ($cost, @crates)`, `my ($c, @r) = eval { ... }`, memoise-by-`//=`, a list built from scalar plus array plus slice, a parenthesised ternary that is grouping |
| 94 | `94-chained-builders-and-guards` | A chaining builder through a hierarchy, and three guard lifetimes | `return $self` through a promoted method, `ref $self` in a base method, DESTROY at a brace, at an undef, and at a sub return |
| 95 | `95-lookahead-at-the-tail` | Every pattern ends in a lookaround, which is the position that converts | commify `(?=\d)`, `(?=\d{10}$)`, `(?=/|$)`, negative `(?!\.?$)`, a counted lookahead substitution |
| 96 | `96-names-the-language-took` | Subs named fmt, json and toText, and one buried in a block | identifier collisions with imports and emitted helpers, hoisting a nested named sub |
| 97 | `97-qualified-package-variables` | One hash spelled %TALLY inside its package and %Counter::TALLY outside | `our` with an initialiser, qualified reads and writes from main, package state shared with the script |
| 98 | `98-reads-of-every-shape` | Four shapes of read through one handle, sharing one position | a scalar `<STDIN>`, a line loop, a mid-loop continuation read behind `and`, `do { local $/; <STDIN> }` for the rest |
| 99 | `99-targets-and-topics` | List-assignment targets of every shape, and the arrow that keeps $_->[0] off $_[0] | inline `my` mid-list, a hash element target, `@+{qw(...)}`, `$_->[0]` in map vs `$_[0]` in a sub |
| 100 | `100-a-byte-level-decoder` | A Q-encoding decoder built from multi-statement /e, byte-wise chr, and interpolated named captures | `s{...}{ several; statements; }ge`, `chr hex $1` building UTF-8 bytes, `"$+{key}"`, undef placeholders in a split unpack |
| 101 | `101-a-table-with-one-signature` | Closures sharing a slot that can keep one written signature | mixed-arity dispatch table, a `$_[0]` member, an int position, a `\|\|` fallback sub, a pipeline array, a counter typed only by its calls |
| 102 | `102-named-subs-in-a-table` | A dispatch table of `\&named_sub` references, which does not type yet | `\&clean_case` values, a `\|\| \&clean_passthrough` fallback, calls through the table |
| 103 | `103-text-and-numbers-in-one-slot` | Slots fed both text and numbers, resolved to the scalar's string form | a mixed config hash, `$carried += 34` on a quoted number, a label/count list, arithmetic on an unpack field |
| 104 | `104-a-number-beside-a-hash` | Mixes of shape rather than of scalar kind, which honestly stay dynamic | a count beside a hashref in one hash, strings and hashrefs alternating in one list, `ref` dispatch |
| 105 | `105-an-env-var-deleted` | %ENV edited in place: set, delete with the value kept, read through a default | `$ENV{K} = v`, `my $x = delete $ENV{K}`, `// '(gone)'` after removal, delete of an unset name |
| 106 | `106-exists-on-an-env-var` | exists on %ENV for values "0", empty, unset, and deleted | `exists $ENV{K}` vs truthiness, set-to-"0" and set-to-empty both existing, delete then exists |
| 107 | `107-closing-every-kind-of-handle` | Every kind of close a script performs, and what each one is really for | `close` on a written file checked with `or die`, on a read file, on a pipe leaving `$?`, on handles walked out of an array |
| 108 | `108-a-handle-passed-as-a-glob` | The pre-lexical way of passing a handle around, which module code still uses | `\*STDOUT`, a glob reference passed to a sub, `print {$fh}` through it, a lexical handle through the same sub |
| 109 | `109-handles-kept-in-a-hash` | Three log files open at once, with the handles filed under names in a hash | `open($h{k}, ...)` straight into a slot, `print { $h{k} }`, a close walking the keys, `-s` on what was written |
| 110 | `110-a-handle-in-a-record` | A handle as one field of a record beside a path and a counter | `open($r->{fh}, ...)`, `print { $r->{fh} }`, a record mixing a handle with text and a number |
| 111 | `111-readline-as-a-function` | The spelling of a read that works on a handle kept in a container | `readline($h{in})` in scalar and list context, and why `<$h{in}>` is a glob instead |
| 112 | `112-a-stable-sort-and-a-checked-open` | Ties keeping their arrival order through three sorts, and an open whose failure is a value | `sort { $a->{k} <=> $b->{k} }` with ties, descending and `cmp` forms, `my $ok = open(...)`, `$!` printed by the script |

## Coverage map

- **Subroutines** - 01, 02, 03 (also 12, 25, 27 for code refs as values)
- **References, all deref syntaxes** - 04, 05
- **Nested data structures** - 06, 07, 08, 29, 30
- **Autovivification (load-bearing)** - **09, 10, 62, 63**; incidental in 07, 22, 24, 29
- **Closures** - 11, 12, 25, 54, 55, 64, 65; complex `map`/`grep`/`sort` blocks in 13
- **Regex** - 14, 15, 16, 17, 18; applied in 23, 28, 29, 30
- **File I/O** - 19, 20, 21, 22, 107, 108, 109, 110, 111, 112; STDIN in 23 and 28; `<>` in 24
- **Command line** - 23, 24, 25, 26, 27
- **Text-processing shapes** - 06, 28, 29, 30 (and the filter in 23)
- **Error handling** - 31, 32; `die` as an assertion in 30
- **Standard modules** - 33 (List::Util), 34 (Scalar::Util, POSIX),
  35 (File::Basename, File::Spec, Cwd, Data::Dumper), 26 and 37
  (Getopt::Long), 32 (Scalar::Util)
- **Object shapes** - 36
- **Blocks as terms** - 38; the slurp form of it in 21
- **Arithmetic and argument typing** - 39, 40, 87; how far the pragma reaches in tier3 36
- **List and string surgery** - 41, 66, 69, 70, 85, 86; the whole of `splice` in tier1 12
- **Nested data and its types** - 42, 45, 46; also 06, 07, 08
- **Paths and temporary trees** - 43, 44, 49, 50, 56
- **Time** - 47, 48
- **Serialisation** - 51, 52, 53
- **Callbacks as values** - 54, 55; also 12, 25
- **Processes** - 57, 67, 68; the status a pipe close leaves in 107
- **Regex in list context** - 58, 59, 83, 84
- **Option blocks** - 26, 37, 60, 61; manual parsing in 25
- **Absence: undef against missing** - 71, 72, 78; the tier1 study of it in 45
- **Arrays outside their range** - 66, 73, 74, 82; behind a reference in tier3 35
- **Match state that outlives the match** - 76, 79, 80
- **Implicit returns** - 81; the constructor shape of it in tier3 34
