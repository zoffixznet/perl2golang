# 37-list-slice-in-scalar-context

## What this exercises
The list slice, `(LIST)[i]`, which picks an element out of a list without
giving the list a name first. With one index it is a single value and behaves
like any other scalar: it can be added to, multiplied, measured and
concatenated. With several indices the same syntax yields a list, and the
entry puts both forms next to each other so the difference is visible.

The three sources are the ones this idiom is actually used on: a literal list,
the return of `split`, and the return of `sort`. Negative indices count from
the end, which is how `(split /:/, $line)[-1]` reads the last field.

## Perl constructs
- `(10, 20, 30)[1]` and `(10, 20, 30)[-1]`
- `(split /:/, $line)[2]`, `[-1]` and `[4]`
- `(sort { $a <=> $b } @scores)[0]` and `[-1]`
- The picked value used in arithmetic, in `length`, and in concatenation
- `(sort ...)[0, -1]`, where two indices make it a list again

## Go concepts a converter must teach
- A list slice with one index is a scalar, so the Go should be one element:
  `fields[2]`, not a slice holding `fields[2]`. Keeping it a slice compiles,
  and then arithmetic on it silently produces the wrong number, which is
  exactly the failure this entry exists to catch.
- Go indexes what it has, so the list has to exist as a value first. Where the
  list came from a call, that means a variable for the result and then an index
  on it, which is what a Go developer writes anyway.
- A negative index is not an index in Go: `-1` has to become `len(xs)-1`, and
  the conversion has to prove `xs` is not empty or accept a panic.
- With more than one index the expression really is a list, and the Go is a
  slice literal gathering the elements.
