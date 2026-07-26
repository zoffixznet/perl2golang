# 25-sort-comparators

## What this exercises
Named comparator subs used as `sort SUBNAME LIST`, comparators that close over
an outer hash (`$score{$b} <=> $score{$a}`), multi-key sorts, a comparator that
parses each record with `split`, sorting hash keys by value with a key
tie-break.

## Perl constructs
- `sub by_x { $a <=> $b }` then `sort by_x @list` -- note **no** parentheses,
  no comma, and no sigil on the sub name
- comparator subs implicitly returning the last expression
- `$a` / `$b` visible inside a named sub because they are package globals
- comparators referencing an outer lexical hash
- `split` inside a comparator (called O(n log n) times -- the reason the
  Schwartzian transform exists)

## Go concepts a converter must teach
- `sort SUBNAME LIST` is a distinct grammar production. A parser that only
  handles `sort BLOCK LIST` and `sort LIST` will mis-parse `sort numerically @n`
  as sorting the list `(numerically(@n))`.
- `$a`/`$b` are **not** parameters. A named comparator in Go becomes
  `func(x, y T) int` and every `$a`/`$b` in the body must be rewritten to the
  parameter names. If the sub is also called normally elsewhere, that is a
  conflict the converter has to detect.
- Comparators closing over an outer hash become closures: the Go function needs
  the map captured, so it cannot be a package-level `func` with the Perl sub's
  name -- it must become a closure or take the map as a bound parameter.
- Chained `or` between comparisons is `cmp.Or(cmp.Compare(...), strings.Compare(...))`
  in modern Go, or nested if-returns.
- Re-parsing inside the comparator is O(n log n) work; a converter that wants
  idiomatic Go should hoist it into a decorate-sort-undecorate, which changes
  nothing observable but is worth flagging as an optimisation opportunity.
