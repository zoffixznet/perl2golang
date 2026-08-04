# 73 - an array that stretches to fit whatever is written into it

## What this exercises
A Perl array has no fixed length: writing at index 5 of a three-element array
makes it six long and leaves indices 3 and 4 holding undef. A Go slice has a
length, and a write past it is a panic rather than a growth, so three separate
things have to be said out loud:

- the room is made before the write, which is one extra statement and the
  visible price of the array not managing itself;
- the gap the growth opens holds undef and not zero, so the element type needs
  the pointer shape for `defined` on a gap to answer correctly;
- `$#a = N` sets the length in both directions, which reslicing alone cannot
  do because it can only reach as far as the capacity happens to go.

The negative index is here because it is the one place Go is stricter than it
looks: `a[-1]` is a compile error for a constant and a panic for a variable,
so counting back from the end is arithmetic that has to be written out, on the
left of an assignment as much as on the right.

## Perl constructs
- `$a[k] = v` where k is past the end, and the undef gaps it opens
- reading past the end, which yields undef and does not lengthen the array
- `$a[-1] = v` and `$a[-2] .= v`
- `$#a = N` used to truncate and then to re-extend
- `$t[k]++` into an array nothing ever sized

## Go concepts a converter must teach
- `len` is part of a slice, not a fact about it that can be assigned. Growth
  is `append`, and growth to a particular length is `append` with a `make`.
- A slice of pointers is what an array with holes in it becomes, for the same
  reason a hash with undef values does: nil is the hole.
- Go rejects a negative constant index outright. That is a deliberate design
  choice rather than an oversight, and the arithmetic that replaces it is
  clearer about what is being read.
