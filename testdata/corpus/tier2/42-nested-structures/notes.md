# What this entry exercises

Nested data, which is where a converter either works out what a program holds
or gives up and calls everything "anything". A hash of arrays built by
autovivification, a hash of hashes read back two levels down, a running total
per key, an array of records returned from a sub, a comparator taken out of a
hash and handed to `sort`, the copy-then-edit idiom, and a list behind a
reference asked how long it is.

The evidence a converter needs is spread across the file. `push @{ $h{$k} },
$v` says the hash holds lists of whatever `$v` is; `$h{$a}{$b} = $v` says it
holds maps; a sub that pushes records and returns them says what the caller's
array holds. Following each of those back to the variable is what keeps the
whole structure out of `any`.

What it costs to convert:

- a hash of arrays needs nothing at the point of use, because reading a
  missing key gives a nil slice and appending to one works
- a hash of hashes needs the inner map made first, because writing to a nil
  map panics
- a comparator held in a variable reads `$a` and `$b`, so the generated code
  keeps those two variables and fills them in before each call
- `(my $copy = $orig) =~ s///` becomes two statements: the copy, then the
  substitution on the copy

## Go concepts to teach

- `maps-of-slices` - the shape a nested structure takes
- `nil-slices-vs-nil-maps` - why one needs making and the other does not
- `sort-slice` - what a comparator looks like when it takes its arguments
- `collections-hold-one-type` - why the element type has to be decided
