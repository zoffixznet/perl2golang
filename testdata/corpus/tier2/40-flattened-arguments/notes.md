# What this entry exercises

Argument lists. Perl flattens every array in a call into one `@_`, so
`total($n, @batch)` and `total($n, 10, 20, 30)` are the same call and the sub
cannot tell them apart. Each line of the entry is a different mixture: one
array alone, single values alone, a value in front of an array, two arrays
in a row, arrays and values interleaved, a fixed parameter followed by a
variadic tail, and an empty array, which contributes nothing.

Go spreads exactly one slice, with `...`, and will not mix that with any
other argument in the same call, so anything past the simplest case has to
build the list first and spread it whole.

What it costs to convert:

- a single array becomes `f(xs...)` and nothing else changes
- anything mixed becomes a list built on the lines above the call, which is
  the flattening written out
- an empty array spreads to nothing, exactly as in Perl, so the count the
  sub sees is the same
- the tail slice is passed rather than copied, so a sub that writes to
  `$_[0]` writes into the caller's array, which is Perl's aliasing surviving
  by accident rather than by design

## Go concepts to teach

- `variadic-and-no-defaults` - the one flexibility Go's argument list has
- `slices-not-arrays` - why the spread is a slice and not a list
- `collections-hold-one-type` - why the built list needs an element type
