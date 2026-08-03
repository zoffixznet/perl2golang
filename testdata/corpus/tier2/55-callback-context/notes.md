# 55 - the context a callback's caller cannot ask for

## What this exercises
The half of the callback problem that does not go away by picking a signature:
Perl's sub returns a list or one value depending on where it was called, and
the same sub called both ways gives two different answers. A Go function has
one return type, so the callback has to pick, and the caller cannot change its
mind.

This is the recorded target. The entry translates and compiles, and the list
half comes out wrong.

## What still goes wrong, and why it is here
- **`my @words = $readers{words}->($line)` gets one element.** The callback
  returns one value of no fixed type holding a list, and assigning it to an
  array does not spread it.
- **`scalar` on the same call gives the list rather than its length** in one
  of the two spellings, because only the written-out `scalar(...)` reaches the
  helper that decides at run time.
- **A callback returning a list of references** cannot be walked without an
  assertion, and the generated one asserts a nil.

## Go concepts a converter must teach
- A function's return type is part of its type, so a caller cannot ask for a
  different one. Where Perl used context to mean "how many", Go uses two
  functions, or one that always returns the slice and lets the caller take
  `len`.
- Returning `(T, bool)` or `(T, error)` is the Go way to say "one value, and
  whether there was one", which is what scalar context was often standing in
  for.
- A callback that must serve both shapes is a sign the table wants an
  interface with two methods rather than one function type.
