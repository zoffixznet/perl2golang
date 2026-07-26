# 26-sort-stability: sort stability and comparator conventions

Group: **C — convertible, but the naive conversion is subtly wrong**

## Construct
`sort { $a->[1] <=> $b->[1] } @records` (line 14) sorts by ONE key with ties.
Perl's sort is a stable mergesort in practice (guaranteed stable under
`use sort 'stable'`; the default has been stable since 5.8): equal-aged
records keep input order — carol before dave before erin. Line 18 sorts
descending by swapping `$a`/`$b`; ties again keep input order.

## Why the naive conversion is subtly wrong
The reflexive Go translation is `sort.Slice`, which is documented UNSTABLE:
equal elements may land in any order. The converted program still "works",
still sorts by age — and reorders carol/dave/erin nondeterministically. Any
downstream diff, pagination, or "first match wins" logic silently changes.
Also: Perl comparators return negative/zero/positive from the spaceship
operator; a converter translating to `sort.Slice`'s `less func` must map
`<=>`/`cmp` to `<` correctly (and `$b ... $a` swaps to reversed comparison),
not just transplant the expression.

## What the converter should do
- Category: **convert-verify**: use `sort.SliceStable` (or `slices.SortStableFunc`)
  for EVERY converted Perl `sort` — unconditionally, since proving tie-freedom
  is rarely possible — and note the choice once in the report.
- `<=>` maps to numeric comparison, `cmp` to string comparison; the converter
  must classify which one, because sorting numbers as strings is another
  silent wrong order (not exercised here; keep the classification anyway).
- Forbidden: `sort.Slice` for a comparator with possible ties, with no
  diagnostic.

## Ideal diagnostic (word for word)
> input.pl:14: note P2G-W402: Perl's sort keeps equal elements in input order;
> converted with sort.SliceStable to preserve tie order (carol/dave/erin in
> this data). Using unstable sort here would reorder ties run-to-run.

## What a human should do instead
Make the tiebreak explicit — add a secondary key to the comparator — so the
order no longer depends on any engine's stability promise.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0):
`alice:25 bob:25 carol:30 dave:30 erin:30` and `carol dave erin alice bob`.
The tie groups appear in input order in BOTH directions; that exact order is
the pass bar for the converted program.
