# 86 - the same construct over an array, which does not convert

## What this exercises
The neighbour of entry 85. A hash slice as a place became a loop over the
keys; an array slice as a place is the same shape with one thing added, and
that one thing is why it is harder: the indices decide how long the array has
to be.

- `@row[0, 1] = ('A', 'B')` is a fixed set of places and already converts.
- `@row[@at] = ('x', 'y')` is not, and index 3 is past the end, so the loop
  has to grow the array as it goes and grow it to the largest index rather
  than to the number of values.
- `@sparse[0, 2, 4] = ('only')` leaves real holes: indices 1 and 3 were never
  named at all, and index 2 was named but had no value. Perl calls all three
  undef, and the element type needs room for that.
- `@pair[0, 1] = @pair[1, 0]` is a swap, which only works because every value
  on the right is worked out before any of them is stored. Go's multiple
  assignment has the same rule, so the fixed form of this one is already
  right; a loop over computed indices would not be.

## Perl constructs
- an array slice read with literal and with computed indices
- an array slice assigned with literal and with computed indices
- a write past the end through a slice, stretching the array
- a short right-hand side leaving holes at named and unnamed indices
- a swap through overlapping slices

## What goes wrong today
The computed-index writes are refused. The refusal is honest and the rest of
the program runs, but two of the four sections print nothing useful.

The loop is the same three lines the hash slice gets, plus a growth to the
largest index before it starts. The swap is the case that must *not* become a
loop, because a loop stores as it goes and would read a value it had already
overwritten.

## Go concepts a converter must teach
- Go's multiple assignment evaluates every right-hand side before storing any
  of them, which is what makes `a, b = b, a` a swap. That guarantee is worth
  knowing on its own, and it is the reason the fixed-index form is better
  than a loop wherever it applies.
- A slice has a length and a write past it panics, so a slice assignment at
  computed indices has to make room first, and it has to make room for the
  largest index rather than for the number of values.
