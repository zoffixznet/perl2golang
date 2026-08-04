# 62 - the counting hash, at every depth

## What this exercises
`$matrix{$host}{$path}{$status}++` declares nothing. The hash, the two hashes
under it and the number at the bottom all come into being as the line runs,
and a report script builds its whole model this way. Go needs the shape
written down before anything can go in it, so the conversion has to work the
shape out from the operations, and the only operation that says anything about
the leaf is the one at the very bottom.

The entry runs that at one, two and three levels, counting with `++` and
accumulating with `+=`, and then asks the built structure the questions a
report asks: `keys %{ $h{$k} }` for the inner hash, `scalar keys` for its size,
a total over its values, and a sorted walk at every level. Those reads are the
half that breaks loudest when the inner level stayed dynamic: an inner hash
that is `any` has no keys a Go program can enumerate, so a wrong type here does
not merely read badly, it prints nothing.

`push @{ $seen_paths{$host} }, $path` is in there as the contrast: a slice
under a key needs no creation at all, because appending to a nil slice works.

## Perl constructs
- `$h{$k}++`, `$h{$a}{$b}++`, `$h{$a}{$b}{$c}++`
- `$h{$a}{$b} += $n` on a value that carries a fraction
- `keys %{ $h{$k} }`, `scalar keys %{ $h{$k} }`, `values` summed in a
  statement-modifier `for`
- `push @{ $h{$k} }, $v` beside the hash-of-hashes forms
- `map` over sorted inner keys, interpolating a two-level access into a string

## Go concepts a converter must teach
- One map has one value type, and the innermost operation is what decides it:
  `++` means the leaves are numbers, and every level above is a map of
  whatever the level below turned out to be.
- Reading a missing key gives the value type's zero value, so a counter needs
  no initialisation; writing through a nil inner map panics, so the inner map
  does need making.
- `if m[k] == nil` is the guard when the value type is a map, and it is
  shorter and clearer than the two-result index form, which answers a
  different question.
- `maps.Keys` with `slices.Sorted` is how a hash is walked in a fixed order,
  at every level, because Go randomises map iteration on purpose.
