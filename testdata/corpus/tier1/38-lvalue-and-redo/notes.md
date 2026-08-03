# What this entry exercises

Assignment targets Perl allows and Go has no name for. A conditional on the
left, so the choice picks a variable rather than producing a value. `substr`
on the left, in all three shapes: a fixed window, a window running to the end
of the string, and a zero-length window, which appends. A hash slice and an
array slice, both assigned in one statement.

`redo` is the neighbouring case that does not convert. It restarts the
current iteration without re-reading the list or advancing the loop variable,
and Go has `break` and `continue` and nothing that goes back to the top of a
body it is already inside. The shape that would work is an inner `for` around
the body with the outer loop labelled, and until that is built the entry is a
recorded target rather than a passing one.

What it costs to convert:

- the conditional target becomes an `if` around two assignments
- `substr` on the left rebuilds the whole string, since a Go string is
  immutable, and the window rules stay the forgiving ones rather than the
  ones slicing a string would apply
- both slice assignments become one multiple assignment, which matches
  Perl's rule that every value on the right is worked out before any of
  them is stored

## Go concepts to teach

- `statements-vs-expressions` - why the conditional cannot be a target
- `strings-are-bytes` - why the string is rebuilt rather than edited
- `range-is-not-foreach` - the loop vocabulary `redo` is missing from
