# 03-undef-and-defined-or

## What this exercises
`undef` as a distinct third state alongside "false" and "empty". `defined`,
the defined-or operator `//`, the assign forms `//=` and `||=`, the difference
between `//` and `||` when the left side is `0` or `""`, missing hash keys
returning undef without autovivifying, `undef $x` to reset, and skipping undef
elements while summing.

## Perl constructs
- `my $x;` leaves the scalar undefined
- `defined EXPR`
- `//`, `//=`, `||=`
- `$conf{user}` on a nonexistent key yields undef (rvalue lookup does not
  create the key)
- `undef $x`

## Go concepts a converter must teach
- Go's zero value is not undef. `var s string` is `""`, which Perl would call
  defined-but-false. Representing undef needs either a pointer (`*string`),
  a `sql.Null`-style struct, or a sentinel.
- `//` is *not* Go's `||`. `x // y` means "x unless x is undef"; `0 // 5` is 0.
  Lowering to `if x == nil { x = y }` is right; lowering to `if x == 0` is wrong.
- `||` returns the *operand*, not a bool: `"" || "dflt"` is the string "dflt".
  Go's `||` is bool-only, so this becomes an if-expression or a helper generic
  `func or[T comparable](a, b T) T`.
- Missing map key in Go returns the zero value plus an `ok` flag -- the `ok`
  flag is Perl's `exists`, and the "is it undef" question is separate.
