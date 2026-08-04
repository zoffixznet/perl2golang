# 69 - lists are flat

## What this exercises
The rule behind half of Perl's list idioms, and the one that decides how many
results they produce: a list inside a list is not a nested thing, it is more
elements.

- `map { ($a, $b) } @rows` gives two results per row, not one result holding
  two.
- `map { @{ $h{$_} } } @keys` flattens one level out of a nested structure,
  which is how a hash of arrays is turned back into one list.
- `( $_ ) x 2` repeats a *list*, where `"ab" x 2` repeats a string, and which
  one it is comes from the left-hand side.
- `map { [ $_, ... ] }` is the contrast: an array **reference** is one value,
  and this block gives one result per input.
- `( %defaults, %site, name => 'edge' )` is a hash spliced into a hash, twice,
  with loose pairs after it. Neither of the two hashes is a key.
- `( 'start', @head, 'end' )` puts a list in the middle of another.

## Perl constructs
- `map` blocks whose value is a list, a dereferenced array, a repetition, and
  a reference
- the list form of `x`
- a hash literal built from two hashes and a trailing pair
- an array interpolated into the middle of a list

## Go concepts a converter must teach
- Go has no flattening. `append(out, xs...)` spreads a slice and
  `append(out, x)` adds one element, and the three dots are the whole
  difference.
- A slice of slices and a flat slice are different types, so getting this
  wrong is usually a compile error, and where it is not it is a wrong count.
- `maps.Clone` and `maps.Copy` are what a merged hash becomes. The copy is
  shallow, which the two lines make visible where the Perl did not.
- `slices.Concat` and `slices.Repeat` cover the list forms of `,` and `x`
  where the pieces are all slices.
