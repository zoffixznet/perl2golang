# 25-hash-order: hash iteration order dependence

Group: **C — convertible, but the naive conversion is subtly wrong**

## Construct
`keys %h` (lines 11, 12, 14) with the order escaping into program OUTPUT.
Perl randomizes hash order PER PROCESS (hash-seed randomization): two runs of
the same script differ, but within one process repeated `keys` calls on an
unmodified hash return the SAME order (lines 11 and 12 always agree).

## Why the naive conversion is subtly wrong
A Go `map` is also "random", but differently: Go randomizes PER ITERATION, so
the converted lines 11 and 12 would usually DISAGREE with each other —
observably different from Perl, where they always agree. Converting to an
ordered structure instead (sorted keys, insertion order) is deterministic and
therefore differently wrong: it hides an order-dependence bug that the Perl
original actually has.

## What the converter should do
- Category: **convert-verify** with a MANDATORY warning: whenever hash
  iteration order can reach observable output (print, join, string building,
  writing to files), the report must flag the site as order-dependent.
- Recommended lowering: iterate Go maps via an explicit key-slice helper, and
  within one program run REUSE the captured order for repeated iterations of
  an unmodified hash (matching Perl's within-process stability). Sorting keys
  is acceptable ONLY if the report says output order was changed from
  "random stable" to "sorted" — that is a semantic change the user must see.
- Forbidden: plain `for k := range m { ... }` into output with no diagnostic.

## Ideal diagnostic (word for word)
> input.pl:11: warning P2G-W401: hash iteration order reaches program output.
> Perl's order is randomized per process but stable within a run; Go's map
> order changes on every iteration. Converted using a captured key list so
> repeated iterations agree within a run, as in Perl. If downstream consumers
> depend on any FIXED order, that is a pre-existing bug in the Perl script.

## What a human should do instead
Sort the keys explicitly (both languages), or use an ordered container.
Consumers of the CSV line must never have depended on hash order.

## Observed with perl 5.42.2 (x86_64-linux)
No `expected_stdout` is recorded — the output is nondeterministic BY DESIGN,
which is itself the specification. Two sample runs:
- run 1: `keys:  epsilon,beta,gamma,delta,alpha` (and `again:` identical)
- run 2: `keys:  epsilon,gamma,delta,alpha,beta` (and `again:` identical)
The invariants a conversion must preserve: all five keys exactly once; the
`keys:` and `again:` lines IDENTICAL within a run; `csv:` in the same order as
`keys:`. A harness should check those invariants rather than a byte diff.
