# Tier 1 corpus -- fundamentals

35 self-contained Perl programs covering the core of the language: scalars,
numbers, comparison, booleans, arrays, hashes, control flow, sorting, strings,
and output. Every entry runs clean under `use strict; use warnings;` with no
output on stderr (except the two entries marked below, which write to stderr on
purpose).

## Layout of an entry

```
NN-short-slug/
  input.pl          the program
  cmd               one line of command-line arguments (empty file = none)
  stdin             stdin to feed it (file absent = no stdin)
  expected_stdout   byte-exact stdout, captured from a real perl run
  expected_exit     exit status as a bare integer
  allow_stderr      present only when stderr output is intentional
  notes.md          constructs exercised and the Go concepts a converter needs
```

## How the expectations were produced

Every `expected_stdout` and `expected_exit` was captured by running the script
under perl 5.42.2, not written by hand. Each script was run twice and the two
stdout captures, stderr captures and exit statuses compared, so every entry is
proven deterministic. No entry uses time, randomness, the filesystem, locale
sensitive formatting, or unsorted hash iteration order.

## Entries

| # | Entry | What it covers |
|---|-------|----------------|
| 01 | `01-scalar-basics` | Scalar literals, single vs double quotes, interpolation and `${name}`, `.` concatenation, `x` repetition, chained assignment, escapes, `length` |
| 02 | `02-string-number-duality` | `"42" + 8` vs `"42" . 8`, `==` vs `eq` on `"007"`, `1_000_000`/`0xff`/`0755`/`0b1010`/`1.5e3`, `hex`, `oct` |
| 03 | `03-undef-and-defined-or` | `undef`, `defined`, `//`, `//=`, `||=`, `//` vs `||` on `0` and `""`, missing hash key, `undef $x` |
| 04 | `04-numeric-arithmetic` | `+ - * / % **`, `int` truncation, `abs`, `sqrt`, float representation error, `%.0f` round-half-even, accumulation, `%.15g` stringification |
| 05 | `05-modulo-with-negatives` | `%` with mixed-sign operands (floored, unlike Go), `int($n/$d)` truncation, the `use integer` pragma switching to C semantics |
| 06 | `06-increment-and-magic-strings` | Pre/post `++`/`--` as expressions, magic string autoincrement (`aa`, `Az`, `zz`, `a9`, `ID001`), `+= -= *= /= **= %= .= x=` |
| 07 | `07-comparison-operators` | `== != < > <= >=` vs `eq ne lt gt le ge`, `<=>` vs `cmp`, `"10" == "10.0"`, string sort vs numeric sort |
| 08 | `08-truthiness` | Exactly which values are false, `"0.0"`/`"00"`/`"0E0"`/`" "`/`"0 but true"` being true, array/hash in boolean context, comparisons returning `1` and `""` |
| 09 | `09-boolean-logic` | `&& || !` vs `and or not`, operators returning an operand not a bool, short circuiting proven with a counter, `xor`, the `my $r = $falsy or ...` precedence trap |
| 10 | `10-array-basics` | Literals, `qw`, `1 .. 10` and `'a' .. 'e'`, `(LIST) x N`, `scalar(@a)`, `$#a`, negative indices, copy semantics, growing/shrinking/clearing |
| 11 | `11-array-push-pop-shift-unshift` | `push`/`pop`/`shift`/`unshift`, return values, `pop` on an empty array, argument-list flattening, queue draining |
| 12 | `12-array-splice` | `splice` remove/insert/replace/truncate, negative offset, negative length, scalar vs list context return |
| 13 | `13-array-slices-and-reverse` | `@a[LIST]` read/write/swap, range and negative slices, slice by an index array, `reverse`/`sort` not mutating, `reverse` in scalar vs list context |
| 14 | `14-list-assignment` | Parallel assignment, swap, short RHS, `my ($x) = ...` vs `my $x = ...`, `(LIST)[i]`, `my ($h, @t) = @a`, `my $n = () = LIST`, flattening, fat comma |
| 15 | `15-hash-basics` | Hash literals, quoted keys, `keys`/`values`, sorted iteration, `exists` vs `defined` vs truth, `delete`, non-creating rvalue lookup, `%h = ()` |
| 16 | `16-hash-slices-and-list-context` | `@h{LIST}` read/write/delete, hash flattened to a list and back, `reverse %h` inversion, `@h{@k} = @v` zip, merge with later keys winning, hash as a set |
| 17 | `17-hash-counting` | Word frequency over STDIN: `while (<STDIN>)`, `chomp`, `split /\s+/`, `lc`, `$h{$k}++` on a missing key, multi-key sort by count then name |
| 18 | `18-hash-each-and-iteration` | `each`, `keys` in scalar context, order-independent aggregation over `values`, sorting keys by value, mutating values during iteration |
| 19 | `19-if-unless-ternary` | `if`/`elsif`/`else`, `unless` and `unless`/`else`, `?:`, chained `?:`, and the ternary in **lvalue** position |
| 20 | `20-while-until-dowhile` | `while`, `until`, `do {} while` including `while (0)`, `my` inside a condition, `while (defined(my $x = shift @q))` vs the truthiness version |
| 21 | `21-for-and-foreach` | C-style `for` with comma sequencing, `foreach my $x`, implicit `$_`, `0 .. $#a`, **loop variable aliasing**, nesting, `reverse` over a range, `'aa' .. 'ad'` |
| 22 | `22-loop-control` | `next`/`last`/`redo`, loop labels, `next LABEL`/`last LABEL`, an unused label, `last` out of a bare block |
| 23 | `23-statement-modifiers` | All six modifiers: `if`, `unless`, `while`, `until`, `for`, `foreach`; `$_` bound by a modifier `for`; a condition with a side effect |
| 24 | `24-sort-basics` | Default (string) sort even on numbers, `{ $a <=> $b }`, `{ $b cmp $a }`, `reverse sort`, case-insensitive sort, computed keys, chained comparators |
| 25 | `25-sort-comparators` | `sort SUBNAME LIST`, named comparator subs, `$a`/`$b` as package globals, comparators closing over an outer hash, multi-field record sorting |
| 26 | `26-string-functions` | `length`, `uc`/`lc`/`ucfirst`/`lcfirst`, `index`/`rindex` with and without a position, `ord`/`chr`, `x`, scalar `reverse`, `split //`, manual padding |
| 27 | `27-substr` | 2/3/4-argument `substr`, negative offset and length, clamping past the end, lvalue `substr`, fixed-width record parsing and construction |
| 28 | `28-sprintf-formats` | `%d %s %f %.2f %.0f %5d %-5d %05d %8s %-8s %.3s %*d %x %X %#x %o %#o %b %#b %e %g %+d`, the space flag, `%%`, and `sprintf` into a variable |
| 29 | `29-chomp-chop-split-join` | `chomp` (scalar, array, return value), `chop`, `split` with default/negative/positive LIMIT, `split //`, `/\s+/` vs the literal `' '` special case, `join`, CSV over STDIN |
| 30 | `30-heredocs` | `<<"EOT"`, `<<'EOT'`, `<<~EOT`, `<<~'RAW'`, two heredocs started on one line, a heredoc as a function argument |
| 31 | `31-output-and-special-vars` | `print` with a list, `printf`, `say`, `say` with no argument, `$,`, `$\`, `$"`, `local`, `print STDOUT`/`print STDERR` (**writes to stderr**) |
| 32 | `32-scalar-and-list-context` | Scalar vs list context on arrays, `reverse`, `split`, `keys`; hash flattening; `wantarray` returning list/scalar/void |
| 33 | `33-argv-and-arguments` | `@ARGV`, `$#ARGV`, `$0`, consuming arguments with `shift`, a hand-rolled flag scanner. Args: `build -v --name "my project" 42 extra` |
| 34 | `34-exit-status` | Explicit `exit 2` from a usage branch; output before the exit is still flushed. **Exit status 2** |
| 35 | `35-die-exit-status` | Uncaught `die "msg\n"` -- message to stderr with no location suffix, **exit status 255** (**writes to stderr**) |

## Entries with non-zero exit status

- `34-exit-status` -- exits 2
- `35-die-exit-status` -- exits 255

## Entries that read stdin

- `17-hash-counting`
- `29-chomp-chop-split-join`

## Entries that take command-line arguments

- `33-argv-and-arguments` -- `build -v --name "my project" 42 extra`
- `34-exit-status` -- `5`

## Entries that write to stderr on purpose

Marked with an `allow_stderr` file. `expected_stdout` covers stdout only.

- `31-output-and-special-vars`
- `35-die-exit-status`

## Cross-cutting conversion hazards this tier captures

These are the places where a mechanical Perl-to-Go lowering is wrong, not just
awkward. Each is exercised by at least one entry so a converter can be tested
against real observed behaviour.

- **`%` on negatives** (05): Perl floors, Go truncates. `-7 % 3` is 2 vs -1.
- **`/` on integers** (04, 06): Perl always divides as floats, Go's `/` on ints
  is integer division.
- **Number stringification** (04): Perl uses `%.15g`, so `2**53` prints as
  `9.00719925474099e+15`.
- **Context** (32, 14, 13, 12): the same expression means different things by
  position. Go has no equivalent; it must become a static analysis pass.
- **Truthiness** (08): `"0.0"` and `"00"` are true, `"0"` is false.
- **`//` is not `||`** (03): `0 // 5` is 0.
- **`undef` is not the zero value** (03, 15): three-way exists/defined/true.
- **`foreach` aliases, Go's `range` copies** (21): assigning to the loop
  variable mutates the array in Perl and is a no-op in Go.
- **`split` drops trailing empty fields by default** (29), and `split /\s+/` is
  not `strings.Fields` (17, 29).
- **`substr` is offset+length and clamps; Go slicing is start+end and panics**
  (27). Same for out-of-range array reads (10).
- **Magic string autoincrement** (06, 10, 21): no Go equivalent at all.
- **`sort` copies, Go's sorts mutate in place** (24, 25).
- **`redo` and lvalue ternaries** (22, 19): no Go construct maps to them.
- **`die` exits 255, not 1** (35), and `exit` must flush buffered output (34).
- **`$,` / `$\` / `$"` are dynamically scoped globals affecting every `print`**
  (31).
- **Heredocs queued from a single line** (30): the lexer needs a pending-heredoc
  queue, not one-token lookahead.
