# What this entry exercises

A work queue written the way a script that manages one is written: `splice`
for every list edit, hash slices for bulk assignment, `each` for the walk,
`substr` on the left of an assignment, and a conditional as an assignment
target.

`splice` is the interesting one. It is the only Perl builtin that removes,
inserts, replaces, changes the length and reports what it took, all in one
call, and Go splits those across `slices.Delete`, `slices.Insert` and
`slices.Replace`, none of which reports anything. Every shape is here: take
from the front, insert without removing, replace a run with a different
number of values, truncate, a negative offset, a negative length, and a
splice through a reference held in a hash.

What it costs to convert:

- the whole of `splice` becomes one helper, and it takes a pointer, because
  a Go function handed a slice can change the elements but not the caller's
  length
- a splice through a hash entry goes via a variable, since a map entry has
  no address in Go
- `@h{qw(a b c)} = (...)` becomes one multiple assignment, which evaluates
  every value before storing any of them, exactly as Perl does
- `substr(...) = ...` rebuilds the string, because a Go string cannot be
  edited where it stands
- `($cond ? $a : $b) += 1` becomes an `if` around two assignments, since
  Go's conditional produces a value and can never name a place

## Go concepts to teach

- `slice-surgery` - the three functions splice becomes
- `slice-aliasing-and-copy` - why the removed run is a copy
- `map-iteration-order` - what `each` was doing and why range needs none of it
- `strings-are-bytes` - why the string is rebuilt
