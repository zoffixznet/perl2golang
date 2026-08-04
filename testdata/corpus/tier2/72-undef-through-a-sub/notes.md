# 72 - absence that crosses a boundary, which does not convert yet

## What this exercises
The neighbour of entry 71. There undef stayed inside one hash, and the
container's element type could be widened to hold it. Here it travels:

- `sub lookup { return $price{$item} }` returns undef for a missing key, so
  the function's result type has to carry the absence and not only the value.
  A Go signature is decided once, so this is a question about the sub rather
  than about any one call.
- `{ name => 'washer', qty => undef }` is an anonymous hash inside a list of
  records, so the absence is two levels down: the field of a record inside a
  slice.
- `my @candidates = ( undef, 0, 7 )` walked with `next unless defined $c`
  has to tell the undef from the zero while the loop variable is a copy.

## Perl constructs
- a sub returning a hash element that may not be there
- an anonymous hash with an undef value, inside an array of records
- a foreach over a list containing both undef and 0, filtered by `defined`

## What goes wrong today
The first section is wrong: the sub's result type settles on `int`, so the
undef becomes 0 and `defined $p` answers yes for the missing key and no for
the stored zero, which is the wrong answer in both directions. The record
list and the candidate walk come out right.

## Go concepts a converter must teach
- A function that may have no answer says so in its signature. Go has two
  spellings: `(int, bool)`, which is the comma-ok shape the standard library
  uses, and `*int`, which is one value and reads better when the caller
  passes it on. Picking between them is a design decision, not a mechanical
  one, which is why it deserves saying out loud.
- Absence propagates: once a value can be missing, every type it passes
  through has to have room for that, or the information is lost at the first
  boundary it crosses.
