# 31-array-autoextend: assignment past the end; `$#a` truncation/extension

Group: **C — convertible, but the naive conversion is subtly wrong**

## Construct
`$a[7] = 8` on a three-element array (line 9) silently grows it to length 8,
filling indices 3-6 with undef. Assigning to `$#a` truncates (`$#a = 1`,
line 17) or re-extends with undefs (`$#a = 4`, line 20). The gap elements are
UNDEF — distinct from `""` and from 0 (they stringify empty under
`no warnings`, line 14's join shows `1,2,3,,,,,8`).

## Why the naive conversion is subtly wrong
Go's `a[7] = 8` on a len-3 slice PANICS. The obvious fix — `append` — appends
at index 3, not 7: silently different layout. A converter using
`make`+`copy` growth but filling with zero values conflates undef with 0/""
(observable through `defined`, line 11, and join output). `$#a = 1`
translating to `a = a[:2]` is correct for truncation but the RE-extension
must bring back UNDEFS, not whatever the underlying array still holds from
before the truncation (a classic slice-aliasing leak: `a[:5]` would resurrect
old values 3 and undef... whatever memory retained).

## What the converter should do
- Category: **convert-verify**: array-element assignment lowers to a helper
  (`perlrt.AvSet(&a, i, v)`) that extends with explicit undef-representing
  values when i >= len. `$#a = n` lowers to `perlrt.AvSetLen(&a, n+1)` which
  truncates via reslicing BUT clears/reallocates on re-extension so old data
  cannot resurface. The undef representation must be distinguishable from 0
  and "" (whatever scalar model the converter uses must support `defined`).
- Static special case: provably in-range assignments may use native
  indexing; the report notes which sites got the helper.
- Forbidden: append-based growth for indexed assignment, or zero-value gaps
  presented as defined.

## Ideal diagnostic (word for word)
> input.pl:9: note P2G-W407: '$a[7] = 8' extends the 3-element array to
> length 8 with undef gaps; converted via perlrt.AvSet (native Go indexing
> would panic). 'defined $a[5]' on line 11 distinguishes those gaps from
> zero values — the scalar model preserves undef.

## What a human should do instead
Size the slice up front (`make([]T, 8)`) or restructure to append-only
growth; replace `$#a` manipulation with explicit reslicing plus a clearing
loop where re-extension matters.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `len: 8`, `gap defined? no`,
`joined: [1,2,3,,,,,8]`, `truncated: 1 2`,
`re-extended len: 5, last defined? no`. The final line is the resurrection
test: after truncate-then-extend, the tail must be undef again.
