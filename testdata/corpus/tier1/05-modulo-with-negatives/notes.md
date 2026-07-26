# 05-modulo-with-negatives

## What this exercises
Perl's `%` on negative operands. This is a genuine semantic divergence from Go
and deserves its own entry.

Perl's `%` (without `use integer`) takes the **sign of the right operand** and
is defined so that `$n % $d` has the same sign as `$d`, i.e. it is a floored
modulus. Go's `%` takes the sign of the **left** operand (truncated
remainder, same as C).

    Perl:  -7 % 3  ==  2        Go:  -7 % 3  == -1
    Perl:   7 % -3 == -2        Go:   7 % -3 ==  1
    Perl:  -7 % -3 == -1        Go:  -7 % -3 == -1

The block using `use integer` shows Perl switching to C/Go semantics inside a
lexical scope, which is exactly the behaviour a naive Go lowering produces.

## Perl constructs
- `%` with mixed-sign operands
- `int($n / $d)` -- truncating division, which does *not* pair with Perl's `%`
- the lexically scoped `use integer` pragma
- `while (@nums) { shift; shift }` to walk a flat list pairwise
- `split /:/` for a two-field parse

## Go concepts a converter must teach
- A direct `a % b` lowering is **wrong** whenever either operand can be
  negative. The correct helper is:

      func pmod(a, b int) int { m := a % b; if m != 0 && (m < 0) != (b < 0) { m += b }; return m }

- Likewise the quotient that pairs with Perl's `%` is a floored division, not
  Go's truncating `/`. `int($n/$d)` in Perl truncates, so Perl's own `int(/)`
  and `%` are *not* consistent with each other either -- the entry prints both
  so a converter can see they disagree for negatives.
- `use integer` is lexically scoped and changes `%`, `/`, and `*` to native
  integer ops in that block only. A converter must track it as a scope-level
  flag, not a file-level one.
