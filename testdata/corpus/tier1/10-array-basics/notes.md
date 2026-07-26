# 10-array-basics

## What this exercises
Array literals, `qw()`, ranges (`1 .. 10` and `'a' .. 'e'`), the list
repetition form of `x`, `scalar(@a)` vs `$#a`, indexing including negative
indices, array interpolation into a string, value-copy assignment semantics,
reading past the end (undef, and does *not* extend the array), writing past
the end (extends and leaves undef holes), shrinking with `$#a = N`, and
clearing with `@a = ()`.

## Perl constructs
- `@array`, `$array[i]`, `$#array`, `scalar(@array)`
- `qw(...)`
- `..` range operator on numbers and on strings (magic string increment)
- `(LIST) x N` list repetition (note: needs the parens, `$x x 3` is string repeat)
- negative indices
- interpolation of a whole array, which joins with `$"` (default a space)

## Go concepts a converter must teach
- Perl arrays are dynamically sized: writing `$a[6]` on a 4-element array grows
  it. In Go this is `for len(a) <= 6 { a = append(a, "") }` then `a[6] = ...`,
  or a helper `setAt`.
- `$#a` is `len(a) - 1`, and assigning to it is a resize (`a = a[:n+1]` when
  shrinking, append-pad when growing).
- Negative indices need translation: `a[len(a)-1]`. Go will panic on a negative
  index rather than wrap.
- **Reading out of range is undef in Perl but panics in Go.** Every array read
  whose index the converter cannot prove in-bounds needs a guarded helper.
- `@copy = @fruit` copies. Go slice assignment *aliases*; the correct lowering
  is `copy := append([]string(nil), fruit...)` or `slices.Clone`.
- `1 .. 10` becomes a loop or a generated slice; `'a' .. 'e'` needs the string
  autoincrement helper from entry 06.
- Array interpolation `"@fruit"` is `strings.Join(fruit, " ")`, honouring `$"`.
