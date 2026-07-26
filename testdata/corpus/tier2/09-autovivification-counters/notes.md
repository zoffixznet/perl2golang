# 09 - autovivification as a counting idiom

## What this exercises
**Autovivification is the point of this entry.** Four accumulators are built
purely by touching keys that do not exist yet, at one, two and three levels of
nesting, and the entry also demonstrates the surprising *read* side of
autovivification.

## Perl constructs
- `$tally{$status}++` on a key that has never been assigned (undef -> 0 -> 1,
  with no "uninitialized" warning because `++` is special-cased)
- `$by_client{$ip}{$method}++` - the intermediate hashref springs into being
- `$matrix{$ip}{$path}{$status}++` - two intermediate levels created at once
- `push @{ $paths_seen{$path} }, $ip;` - an arrayref autovivified inside a hash
- `split ' ', $line` (the magic single-space split)
- `grep { !$uniq{$_}++ }` - the canonical dedupe-preserving-order idiom, which
  is itself autovivification
- `exists $matrix{'10.0.0.9'}` before and after a *read* of
  `$matrix{'10.0.0.9'}{'/nope'}`, showing that reading through an intermediate
  level creates that level but not the leaf
- `sort keys` at every level so output is deterministic

## Go concepts a converter must teach
- **Go has no autovivification.** `m[a][b]++` panics on a nil inner map. Every
  nested increment must become a get-or-create sequence. A converter that
  emits the literal expression produces code that compiles and then panics at
  runtime, which is the worst possible failure mode.
- Recommended targets: (a) a helper `func ensure(m map[string]map[string]int,
  k string) map[string]int`, (b) a flattened `map[key3]int` with a comparable
  struct key, or (c) generated accessor methods. The corpus entry is a good
  test of which strategy the converter picks.
- One-level `m[k]++` *does* work in Go (missing key reads as 0), so the
  converter must distinguish depth-1 from depth-2+.
- `push @{ $h{$k} }, $v` works in Go via `append` on a nil slice - another
  depth-dependent difference.
- The read-creates-intermediate behaviour has no Go analogue at all and is a
  genuine semantic difference: after the probe, Perl reports four client keys,
  Go would report three. A converter must decide whether to reproduce it (it
  usually should not) and say so.
- `grep { !$seen{$_}++ }` becomes a `map[string]struct{}` plus an append loop.
