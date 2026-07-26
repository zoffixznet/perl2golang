# 30-negative-index: negative indices, negative substr length, tolerant OOB

Group: **C — convertible, but the naive conversion is subtly wrong**

## Construct
`$a[-1]` reads and `$a[-1] = 99` WRITES the last element (lines 9-11).
`substr($s, -3)` starts three from the end; `substr($s, -3, 2)` takes two
from there; `substr($s, 1, -1)` has a negative LENGTH meaning "leave one char
off the end" (lines 15-17). `$a[10]` past the end is undef, not a panic
(line 19).

## Why the naive conversion is subtly wrong
Go slices/strings panic on negative indices and out-of-range access, and Go
has no negative-length substring convention. The naive `a[i]` / `s[i:j]`
translation compiles and then either panics at runtime (crash — at least
loud) or, worse, a converter "fixes" negative constants by clamping to 0,
silently reading the FIRST element instead of the LAST. The undef-on-OOB
read must also not become a panic, because the Perl program's logic (the
`//` default on line 19) depends on it.

## What the converter should do
- Category: **convert-verify**: index and substr operations translate through
  helpers implementing Perl's rules:
  - `perlrt.Idx(a, i)`: i < 0 means len(a)+i; out of range yields the zero
    value plus an "existed" flag where the program tests definedness.
  - `perlrt.IdxSet(a, i, v)`: negative resolves from the end; past-the-end
    extends (see entry 31).
  - `perlrt.Substr(s, off, len...)`: negative offset from the end; negative
    length means stop -len from the end; out-of-range clipping per perldoc.
  - Static special case: a provably in-range non-negative index MAY use
    native indexing (report notes which).
- Forbidden: emitting native `a[i]` for an index that can be negative or out
  of range, without a diagnostic.

## Ideal diagnostic (word for word)
> input.pl:9: note P2G-W406: '$a[-1]' indexes from the end of the array;
> converted via perlrt.Idx (Go would panic on a negative index). Out-of-range
> reads yield undef as in Perl (line 19 depends on this).

## What a human should do instead
Use explicit `len(a)-1` arithmetic and bounds checks in Go; for substr,
compute offsets explicitly. Each helper call in generated code marks a spot
a human porter would rewrite idiomatically.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `last: 40`, `second-last: 30`,
`after write: 10 20 30 99`, `substr -3:   llo`, `substr -3,2: ll`,
`neg length:  ell`, `oob read: undef (no panic)`. The `neg length` row and
the final row are the two most commonly mis-converted.
