# 13-deep-autoviv: autovivification through several levels

Group: **B — convertible only with an approximation that changes semantics**

## Construct
Reading `$h{a}{b}{c}` in a boolean test (line 8) CREATES `$h{a}` and
`$h{a}{b}` (but not the leaf `c`). Writing `$cfg{db}{primary}{port} = 5432`
(line 14) builds three levels in one statement. Passing `$seen{x}{y}` to a sub
(line 19) vivifies `$seen{x}`, because sub arguments are lvalues.

## Why naive Go conversion changes semantics
A Go `map[string]map[string]map[string]V` read does NOT create intermediate
maps; a naive translation of line 8 either panics (indexing a nil inner map is
fine for reads in Go, but a 3-level read yields zero values, creating nothing)
or — the common bug — the converter adds vivification only on WRITES. Then
`exists $h{a}` after the read reports false where Perl reports true. The
observable difference is exactly what this file prints.

## What the converter should do
- Category: **shim**. Emit a nested-container helper
  (e.g. `perlrt.Hash` with `Get(path...)` non-vivifying only where Perl does not
  vivify, and `GetLV(path...)`/`SetPath(path..., v)` vivifying) and translate:
  - rvalue-but-vivifying contexts (multi-level read, sub-call argument) to the
    vivifying accessor;
  - genuine non-vivifying contexts (`exists`, the FINAL key of a read) to the
    non-vivifying one.
- Report entry required per vivification site, because the emitted code differs
  from the "obvious" map indexing a reviewer expects.
- Acceptable alternative for a converter without a runtime: convert reads
  non-vivifying and emit a warning per site admitting the divergence — but then
  this entry's run comparison MUST be reported as failed, not passed.

## Ideal diagnostic (word for word)
> input.pl:8: warning P2G-W303: reading '$h{a}{b}{c}' autovivifies $h{a} and
> $h{a}{b} in Perl. Converted using the vivifying accessor perlrt shim so later
> 'exists' checks agree with Perl. Without the shim, 'exists $h{a}' on line 9
> would return false where Perl returns true.

## What a human should do instead
Decide whether vivification was ever intended. Usually it was not: guard deep
reads with `exists` at each level (Perl) or comma-ok map reads (Go), and make
writes create levels explicitly.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0). Load-bearing lines: `after read: h{a} EXISTS`,
`inner:      h{a}{b} EXISTS`, `leaf:       leaf absent` (the leaf is NOT
created), and `seen{x}: EXISTS (viv'd by a sub call)`.
