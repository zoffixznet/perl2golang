# 12-array-splice

## What this exercises
`splice` in all of its shapes: remove N at offset, insert without removing
(length 0), replace with a different number of elements, negative offset,
negative length (meaning "leave that many at the end"), truncate (offset only),
splice everything (no arguments beyond the array), and splice in scalar context
returning the last element removed rather than a list.

## Perl constructs
- `splice(@a, OFFSET)` / `(@a, OFFSET, LENGTH)` / `(@a, OFFSET, LENGTH, LIST)`
- negative OFFSET counts from the end
- negative LENGTH means "stop that many from the end"
- scalar vs list context for the return value

## Go concepts a converter must teach
- There is no `splice` in Go; each shape becomes explicit slice surgery:

      removed := append([]T(nil), a[off:off+n]...)     // must copy!
      a = append(a[:off], a[off+n:]...)                // remove
      a = append(a[:off], append(append([]T{}, ins...), a[off:]...)...)  // insert

  The inner copy matters: `append(a[:off], a[off+n:]...)` overwrites the region
  the "removed" slice would otherwise point at, so the removed elements have to
  be cloned first. This is a real bug generator in naive conversions.
- Negative offsets and negative lengths must be normalised before use, and Perl
  clamps out-of-range offsets rather than panicking.
- Context sensitivity of the return value means the converter must look at how
  the result is used: `my @x = splice(...)` vs `my $x = splice(...)` produce
  different Go expressions from the same Perl call.
