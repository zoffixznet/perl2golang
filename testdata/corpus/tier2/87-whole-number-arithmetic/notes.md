# 87 - dividing bytes into pages, with and without `use integer`

## What this exercises
Perl's `/` is always floating point, so a script that wants whole-number
arithmetic reaches for the `use integer` pragma. Inside its scope two operators
change meaning at once:

- `/` truncates towards zero instead of producing a fraction, so
  `( $n + $page - 1 ) / $page` is the round-up idiom and reads correctly;
- `%` takes its sign from the *left* operand, which is C's rule, instead of
  from the right, which is Perl's. `-7 % 3` is 2 outside the pragma and -1
  inside it.

Both of those are exactly what Go's operators on `int` values already do, so
inside the pragma the arithmetic is written plainly and the float conversions
around it disappear. That makes this one of the few places where the Go comes
out *shorter* than the Perl.

The entry also puts the same sums outside the pragma beside them, so that the
lexical scoping is visible: the block ends and `/` is floating point again on
the next line.

## Perl constructs
- `use integer` inside a bare block, over a loop and over a nested block
- `/` and `%` on the same operands inside and outside the pragma
- the round-up idiom `( $n + $d - 1 ) / $d`
- `int($n / $d)` with a negative numerator, which truncates towards zero either
  way

## Go concepts a converter must teach
- Go has no floating-point division of two ints and no integer division of two
  floats: the operator does what the operand types say, which is why the
  conversions are where they are.
- `%` on ints in Go takes the sign of the dividend, and there is no built-in
  that takes it from the divisor. Perl's rule needs a helper; C's is free.
- A constant with a fractional part cannot be converted to `int` in Go at all,
  which is what `int(-7 / 2)` becomes once Perl's `/` has made the division
  floating point. Working the constant out is the only line the compiler
  accepts, and it is also the clearer one.
