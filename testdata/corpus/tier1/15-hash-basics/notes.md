# 15-hash-basics

## What this exercises
Hash literals with the fat comma, keys that need quoting, `$h{k}` access,
adding keys by assignment, sorted iteration over `keys`, summing `values`,
the three-way distinction between `exists` / `defined` / truth, `delete`
(which returns the removed value), the fact that an rvalue lookup of a missing
key does not create it, and clearing with `%h = ()`.

## Perl constructs
- `%hash`, `$hash{key}`, `keys`, `values`, `delete`, `exists`
- `=>` auto-quoting the bareword on its left
- `for my $k (sort keys %h)` -- the mandatory idiom for deterministic output
- a key whose value is `undef` (exists but not defined)

## Go concepts a converter must teach
- `map[string]int` is the obvious target, but Perl hash values are scalars that
  can be undef, so `map[string]*int` or a sentinel is needed if the program
  distinguishes "absent" from "present but undef" -- as this entry does.
- `exists $h{k}` is `_, ok := m[k]`. `defined $h{k}` is a *separate* question.
  `$h{k}` being true is a third. Collapsing all three into `if m[k] != 0`
  is the single most common conversion bug for hashes.
- **Iteration order.** Perl's `keys` order is randomised per process; Go's
  `range` order is randomised per iteration. Neither is stable, so any
  conversion must preserve the `sort` the Perl author wrote -- and any Perl
  program that *didn't* sort was already nondeterministic.
- `delete` returns the value; Go's `delete(m, k)` returns nothing, so the
  converter must read first when the value is used.
- `%h = ()` is `m = map[string]int{}` (or `clear(m)` in Go 1.21+), and the
  distinction matters if the map is shared.
