# 24-sort-basics

## What this exercises
`sort` with no block (always a **string** sort, even for numbers), the numeric
block `{ $a <=> $b }`, descending via `{ $b <=> $a }` and via `reverse sort`,
case-insensitive sorting, sorting by a computed key, chained comparators with
`or`, and the fact that `sort` returns a new list without touching the original.

The default-sort-on-numbers line is the point of the entry: `sort (10,9,100,2)`
gives `10 100 2 9`.

## Perl constructs
- `sort LIST`, `sort BLOCK LIST`
- the package globals `$a` and `$b` (exempt from `strict vars`)
- `<=>` for numbers, `cmp` for strings
- `COMPARE_1 or COMPARE_2` to chain tie-breakers
- `reverse sort` vs `sort { $b cmp $a }`

## Go concepts a converter must teach
- `sort` returns a copy; Go's `sort.Slice` / `slices.Sort` sort **in place**.
  Every `my @s = sort @a;` needs a `slices.Clone` first, or `@a` is silently
  reordered too.
- `$a` and `$b` are globals set by the sort machinery, not parameters. In Go
  the comparator receives indices (`func(i, j int) bool`) or values
  (`slices.SortFunc`'s `func(a, b T) int`). `slices.SortFunc` is the closest
  match since it takes a three-way result exactly like `<=>`/`cmp`.
- Perl's comparator returns -1/0/1; Go's `sort.Slice` wants a `less` bool.
  Mechanically: `less(i,j) := cmp(a[i], a[j]) < 0`. Chained `or` comparators
  translate to Go's `cmp.Or(...)` or a sequence of early returns.
- Default `sort` is `slices.Sort` on strings -- but if the converter typed the
  slice as `[]int`, `slices.Sort` gives numeric order and the output changes.
  The converter must preserve "string sort on numeric data" faithfully by
  formatting to strings first.
- Perl's `sort` is not guaranteed stable (it has been mergesort since 5.8, but
  the docs do not promise it). `sort.SliceStable` is the safe target.
