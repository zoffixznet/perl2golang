# 74 - the same stretching, with an index the converter cannot see

## What this exercises
The neighbour of entry 73. There every index was written out as a number, so
the converter could compare it against what it knew about the array's length
and say, at conversion time, whether the write was extending it. Here the
indices are computed, and neither question has an answer before the program
runs:

- `$bucket[$where] += $n` may be writing inside the array or well past its
  end, so the growth has to happen unconditionally;
- and because the converter cannot prove the write opens a gap, the element
  type stays a plain number and the gap holds 0 rather than undef;
- `$three[$i]` with a computed `$i` past the end is undef in Perl and a panic
  in Go, and the plain index expression is kept because wrapping every
  `xs[i]` in a tolerant read would cost more than it buys.

## Perl constructs
- `+=` at an index worked out from a modulus
- `defined` over an array with a computed hole in it
- reading at a computed index past the end, in a loop

## What goes wrong today
Two things. The gap at index 3 prints as 0 and reports as defined, where Perl
prints nothing and reports undef. And the read at a computed index past the
end panics, so the program stops before its last two lines.

## Go concepts a converter must teach
- What is knowable at compile time decides what the generated code can be. The
  same Perl line becomes a plain index expression, a tolerant read, or a
  growth, depending on what the converter can prove, and saying which and why
  is more useful than any one of the three.
- A sparse table indexed by a computed number is usually a map in Go, keyed by
  the index. That shape has no growth, no gaps and no panic, and the two-result
  read answers exactly the question `defined` was asking.
