# 13-array-slices-and-reverse

## What this exercises
Array slices `@a[LIST]` for reading, for writing, and for the swap idiom
`@w[1,0] = @w[0,1]`. Slice index lists can be literals, ranges, negatives, or
another array. Also `reverse` and `sort` returning new lists rather than
mutating, and the context-sensitivity of `reverse`: in list context it reverses
the list, in scalar context it concatenates its arguments and reverses the
*characters*.

## Perl constructs
- `@a[...]` slice (note the `@` sigil with the `[]` subscript)
- slice on the left of an assignment
- `@a[1 .. $#a]` for "all but the first"
- `reverse` in list vs scalar context; `scalar(reverse $s)`
- `my ($first, @rest) = @list;`

## Go concepts a converter must teach
- Go slicing `a[1:4]` is a *contiguous range and it aliases*. Perl's slice is
  a gather-by-index-list and always copies. Only `@a[i .. j]` maps to Go's
  `a[i:j+1]`, and even then a `slices.Clone` is needed if the result is
  mutated.
- Non-contiguous slices become an explicit gather loop.
- Slice assignment `@w[0,1] = ("A","B")` becomes a scatter loop. The swap form
  works in Perl because the right-hand side is fully evaluated first; a naive
  Go scatter loop would clobber. The converter must materialise the RHS.
- `reverse` in list context is a manual loop or `slices.Reverse` (which mutates
  in place -- Perl's does not, so clone first).
- `scalar reverse $s` is a rune-reversal helper, not `slices.Reverse` on bytes,
  or multibyte characters break.
- `my ($first, @rest) = @w` is `first, rest := w[0], w[1:]` with an emptiness
  guard.
