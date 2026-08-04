# 82 - an array's length, in the three places Perl never mentions it

## What this exercises
A Levenshtein table, which is the smallest realistic program that needs all
three of the answers a converter has to have about array length.

- **A table built by writing into it.** `$d[$i][0] = $i` creates the outer
  array up to `$i`, puts an array reference there, and creates that one up to
  0. Perl calls this autovivification and says nothing about it. Go starts
  nil at both levels, and both a write past the end and a write into a nil
  inner slice are panics, so the growth has to be written out at every level
  and written back, because growing may reallocate.
- **A loop counting over the indices an array has.** Inside
  `for my $i (0 .. $#a)`, `$a[$i]` can only name an element the array has, so
  it is a plain Go index expression. `for my $i (1 .. @a)` with `$a[$i-1]` is
  the same statement written the other way round, and has to be recognised as
  such or every read grows a call around it.
- **A read that may be past the end.** The last loop counts over `@b`, which
  is longer than `@a`, so `$a[$i]` is undef on the last step. That is not an
  error in Perl and is a panic in Go, so the read goes through the tolerant
  helper and `defined` has a real answer.

`0 .. @a` also puts an array in numeric context, where it is the element
count. That is `len(a)` and nothing else, and reading it as a number the way
a scalar would be read gives zero and a loop that never runs.

## Perl constructs
- `$d[$i][$j] = ...` building a two-dimensional table with no declaration
- `0 .. @a`, `1 .. @a` and `0 .. $#a` as loop ranges
- `$a[$i - 1]` inside a loop counting from one
- a read at an index the array may not have, guarded with `defined`
- `scalar @{ $d[0] }` on a nested array

## Go concepts a converter must teach
- What the converter can prove decides what the generated code is allowed to
  be. The same `$a[$i]` becomes `a[i]` or `at(a, i)` depending on whether the
  loop that produced `$i` was counting over `a`, and saying which and why is
  the lesson.
- A slice of slices is nil at every level until something makes it. There is
  no autovivification, and `grow` has to be assigned back because append may
  return a different backing array.
- `len` is the only way to ask how long a slice is, and it is what an array in
  Perl's numeric context always meant.
