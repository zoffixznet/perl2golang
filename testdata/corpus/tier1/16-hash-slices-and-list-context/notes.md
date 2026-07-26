# 16-hash-slices-and-list-context

## What this exercises
Hash slices `@h{LIST}` for reading, writing and deleting; a hash flattening to
a key/value list and being rebuilt from one; `reverse %h` to invert a hash;
`@color{@names} = @codes` to zip two arrays into a hash; merging hashes so
later keys win; and the "hash as a set" idiom.

## Perl constructs
- `@hash{...}` slice -- `@` sigil, `{}` subscript
- slice on the left of `=`, and `delete @h{...}`
- `my @flat = %h` and `my %h2 = @flat`
- `reverse %h` (works because flattening yields an even-length list)
- `%merged = (%a, %b)` -- rightmost duplicate key wins
- `$seen{$_} = 1 for LIST`

## Go concepts a converter must teach
- Slices of a map become loops. There is no `m[k1, k2]` in Go.
- Flattening a map to a list and back is order-dependent in Perl but this entry
  is written so the *result* is order-independent. A converter that emits a
  `range` over a Go map reproduces that property; if the Perl program had
  depended on the order it would already have been broken.
- `reverse %h` as inversion works only because the flattened list has an even
  length and reversing swaps every key with its value. In Go this is an
  explicit `for k, v := range m { inv[v] = k }`, and note the value type becomes
  the key type -- Go requires the value type to be comparable for this to
  compile at all, which Perl never checks.
- `@color{@names} = @codes` is a zip loop with a length mismatch policy (Perl
  pads with undef).
- `%merged = (%a, %b)` is two `range` copy loops in order.
- The set idiom becomes `map[string]struct{}` or `map[string]bool`.
