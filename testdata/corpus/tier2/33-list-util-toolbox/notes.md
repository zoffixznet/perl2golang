# 33 - List::Util

## What this exercises
The List::Util functions that appear in almost every non-trivial Perl script,
applied to a small load-average table.

## Perl constructs
- `sum`, `sum0` - and the fact that **`sum` returns `undef` on an empty list**
  while `sum0` returns 0
- `max`, `min` (numeric) and `maxstr`, `minstr` (string comparison)
- `first { BLOCK } @list` - short-circuiting search returning `undef` when
  nothing matches
- `reduce { $a OP $b }` with the implicit `$a` accumulator and `$b` item,
  used three ways: numeric product, "longest string", and string chaining
- **`reduce` building a hashref**: `reduce { $a->{$b} = ...; $a } {}, @hosts` -
  the accumulator is a reference and the block must return it
- `any`, `all`, `none` as boolean predicates over a block
- `uniq` (string equality, order preserving) versus `uniqnum` (numeric
  equality) on the same list, giving different results for `1` / `'1.0'`
- `pairs` turning a flat `k => v` list into arrayrefs
- prototypes: these all take a block as the first argument, which is why they
  read like builtins
- composing them: `sort { max(@{$samples{$b}}) <=> max(@{$samples{$a}}) }`

## Go concepts a converter must teach
- None of these exist in Go's standard library before generics; with Go 1.21+
  there are `slices.Max`, `slices.Min`, `slices.Contains`, `slices.IndexFunc`,
  and `maps.Keys`. A converter should emit a small generic helper package
  rather than inlining a loop at every call site.
- **`sum` returning `undef` vs `sum0` returning 0** is exactly the
  nil-vs-zero-value distinction Go forces you to make explicit: `(int, bool)`
  or a pointer.
- `max`/`min` on an empty slice panics in `slices.Max`; Perl returns `undef`.
- `first` is `slices.IndexFunc` plus an index check, or a loop with `break` -
  and the "not found" case must not be conflated with "found at index 0".
- `reduce` is a plain loop; the reference-accumulator variant shows why the
  block must return the accumulator, which in Go is just mutating a map.
- `$a`/`$b` in `reduce` are package globals, not parameters - the converter
  must rename them.
- `uniq` vs `uniqnum`: Perl compares stringified values by default, so `1` and
  `'1.0'` are distinct under `uniq` but equal under `uniqnum`. In Go the
  element type decides, so the converter must pick `[]string` or `[]float64`
  deliberately - and that choice changes the answer.
- `any`/`all`/`none` short-circuit; a naive `for` conversion should keep the
  early `return`.
