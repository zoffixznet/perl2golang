# 35 - the same grid, one level of indirection away

## What this exercises
The neighbour of tier2 entry 82. There the table was a named array, so the
converter could see the variable being grown and write the growth back into
it. Here the grid is an array reference held in a scalar, which is what it
becomes the moment it has to be passed to a sub or stored in a structure, and
that is the shape most real code is written in.

Everything else is the same program: a rectangular table filled by index, a
sub that sums one row, and a write well past the end of both levels that
leaves a ragged grid with undef in the skipped cells.

## Perl constructs
- `$grid->[$r][$c] = ...` where `$grid` is a scalar holding an array reference
- the same write inside a sub, through a reference passed as an argument
- `@{ $grid->[$r] }`, `scalar @$grid`, `$#{$grid}` and `$grid->[-1]`
- a write past the end of both levels, and `defined` on a cell it skipped

## What goes wrong today
The growth analysis only follows a chain of index steps down to a *named*
array, because that is where it has a variable to assign the grown slice back
into. Through a reference it has nothing solid to write to, so no growth is
emitted and the first write panics on a nil slice.

The fix is to let the chain end at any place that can be assigned to, not only
at a binding: a scalar holding a slice, a struct field, or an element of an
outer container are all places, and the growth is the same three lines in each
case.

## Go concepts a converter must teach
- Perl's `$ref->[$i]` and `$array[$i]` are two spellings that behave alike
  because the reference is transparent. In Go a slice value *is* already a
  reference to its backing array, so there is no extra level to write down,
  and the two spellings collapse into one.
- What a slice value does not carry is its own length: growing it produces a
  new header, and the caller only sees the growth if it is assigned back. That
  is why a Go function that appends takes and returns the slice, and it is the
  whole reason `append` looks the way it does.
