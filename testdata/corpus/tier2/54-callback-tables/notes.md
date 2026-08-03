# 54 - callbacks kept in collections

## What this exercises
The three shapes a script puts functions in a slot: the dispatch table keyed
by name, the pipeline held in an array, and the closure that remembers
something. All three are the same problem in Go: a collection has one element
type, and Perl's subs disagree about what they take and return.

## Perl constructs
- a hash of anonymous subs with different arities and different return kinds,
  including one that returns nothing
- calling through the table by a literal key and by a name from a loop
- an array of subs applied one after another as a pipeline
- a closure created inside a loop that closes over a variable of its own,
  stored in a hash and called several times

## Go concepts a converter must teach
- A map has one value type. Two closures that disagree about their signature
  cannot both go in it, so they are given one: everything in through a
  variadic list, one value of no fixed type out. That is the cost of the
  table, and it is why a Go program usually writes a small interface or a
  named function type instead.
- A closure created in a loop closes over the variable declared *inside* the
  loop, so each one gets its own. Go 1.22 made the range variable per
  iteration too, which removed the classic version of this bug.
- Reading an argument that was not passed is a panic in Go, where it was undef
  in Perl, so a variadic callback reads its arguments through something that
  tolerates a short list.
- A function value is an ordinary value: it can be stored, passed and
  returned, and unlike Perl's it carries a signature the compiler checks at
  every call.
