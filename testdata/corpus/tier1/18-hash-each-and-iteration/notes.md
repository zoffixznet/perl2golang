# 18-hash-each-and-iteration

## What this exercises
`each` as a two-value iterator (with the results collected and sorted so the
output is deterministic despite Perl's randomised hash order), `keys` in scalar
context giving the pair count, aggregating `values` in an order-independent way
(sum/min/max), iterating in a chosen order via a sort block that consults the
hash, and mutating values while iterating the key list.

The only `map` in the tier-1 corpus appears on the last line, used purely to
format sorted output.

## Perl constructs
- `while (my ($k, $v) = each %h)`
- `my $n = keys %h` (scalar context on `keys`)
- `sort { $stock{$a} <=> $stock{$b} or $a cmp $b } keys %stock`
- statement-modifier `if` inside a loop body
- `map { "$_=$h{$_}" } sort keys %h`

## Go concepts a converter must teach
- `each` maintains a hidden per-hash iterator that is shared with `keys` and
  `values` -- calling `keys %h` mid-iteration resets it. Go has no such thing;
  `for k, v := range m` is the lowering, and any Perl code that relied on the
  shared iterator state (interleaved `each` and `keys`) has no equivalent and
  must be restructured.
- A `while (my ($k,$v) = each %h)` loop that is *not* order-independent cannot
  be converted faithfully, because both languages randomise. The right move is
  to flag it: if the loop body appends to output, the Perl program was already
  nondeterministic.
- `my $n = keys %h` is `len(m)`; note this is scalar context on a function that
  normally returns a list.
- Sorting keys by their hash value is `sort.Slice(keys, func(i,j int) bool
  { ... m[keys[i]] ... })`.
- Mutating map values inside a `range` over the same map is legal in Go for
  existing keys (only *adding* keys during range is unspecified) -- same as
  Perl, where adding keys during `each` is explicitly undefined behaviour.
