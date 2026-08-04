# 66 - reading and writing an array outside its range

## What this exercises
The neighbour of the tolerant-read work: the other place a converted program
still stops dead.

Perl's array has no bounds. Reading past the end gives undef and changes
nothing; writing past the end grows the array and fills the gap with undef; a
negative index counts back from the end, on both sides of an assignment; and
`$#a = 2` truncates. Go's slice has bounds and panics on every one of those
except the read of an in-range element.

## Perl constructs
- `$a[99]` on a four-element array, and `$empty[0]`
- `$fields[1]` where `split` returned one field
- `$a[6] = 'fig'` on a four-element array, and the undef gap it leaves at 5
- `$a[-1]` and `$a[-2]` read, `$a[-1] = ...` written, `$lines[-1] .= ...`
- `$#a = 2` as a place rather than as a value

## What goes wrong today
The file does not compile: `invalid argument: index -1 (constant of type int)
must not be negative`. A negative literal index is written out as
`len(a)-1` when it is read and left as `-1` when it is written, which Go
rejects outright. The write past the end compiles and panics.

The read of `$a[99]` is a separate decision. Go's panic there is arguably the
kinder behaviour and the generated code says so in a note, so the question is
not "how do we stop it panicking" but "where is the line between a difference
worth keeping and one worth hiding".

## Go concepts a converter must teach
- A slice has a length, and `append` is the only thing that changes it.
  Growing to reach an index is a loop or a `make` plus a copy, and writing one
  out is where the difference from Perl becomes obvious.
- There is no negative index. `a[len(a)-1]` is the last element, and it panics
  on an empty slice, which is why the empty case has to be handled and not
  assumed.
- Truncating is `a = a[:n]`, which keeps the backing array, so the elements
  beyond `n` are still reachable through any other slice that shares it.
