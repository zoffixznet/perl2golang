# 41 - undef in a typed container, and reading past the end

## What this exercises
The one place Perl's single scalar type and Go's many part company for good.
Perl's `undef` is a value every scalar can hold and is distinct from `0` and
from `''`; a Go `int` has no such value, and the zero it does have is a real
number. This is the recorded target, and every difference in it is the same
difference wearing a different hat.

## What still goes wrong, and why it is here
- **`defined $age{frank}` says yes.** The hash holds ints, `undef` was stored
  as `0`, and `defined` on an int can only ask whether it is zero.
- **`defined $zero` says no**, for the same reason in the other direction.
- **`$zero // 'fallback'` takes the fallback**, where `//` is defined-or and
  should not.
- **The queue drain stops at the zero**, dropping two elements, because the
  `defined` around the shift cannot tell an absent element from a zero one.
- **Reading past the end panics** where Perl gives `undef`. That one is a
  deliberate choice rather than an oversight: a loud failure beats a silent
  wrong answer, and it is reported as such.

## Go concepts a converter must teach
- The Go answers to `undef` are all explicit, and which one to choose is a
  design decision the type declaration records: a pointer (`*int`), a
  `(value, ok)` pair, a `sql.Null`-style struct, or a sentinel the domain
  agrees on.
- The comma-ok form of a map read, `v, ok := m[k]`, is the exact counterpart
  of `exists`, and it is the only one of these that Go gives for free.
- A queue drained with `for len(q) > 0` does not need to ask whether an
  element is present, which is why the Go shape of that loop never had the
  problem in the first place.
