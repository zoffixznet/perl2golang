# 20-while-until-dowhile

## What this exercises
`while`, `until`, and `do {} while` including the `while (0)` case that proves
the body runs at least once. Then draining a queue with
`while (defined(my $x = shift @q))`, and the bug that appears when you drop the
`defined` and a `0` shows up in the data.

## Perl constructs
- `while COND BLOCK`, `until COND BLOCK`
- `do BLOCK while COND` (statement-modifier form; `do BLOCK until COND` is the
  same shape with the condition inverted)
- `my` declaration inside the loop condition

## Go concepts a converter must teach
- Go has exactly one loop keyword. `while (c)` is `for c`; `until (c)` is
  `for !c`; `while (1)` would be `for {}`.
- **`do {} while` is not a loop in Perl** -- it is a `do` block with a statement
  modifier, which is why `last`/`next` do *not* work inside it. Go's equivalent
  is `for { body; if !cond { break } }`, and there `break` *does* work, so a
  converter must not assume the two are interchangeable in the presence of loop
  control.
- A `my` declaration inside a condition scopes to the loop body in both
  languages, but Go requires the `for x := ...; cond;` form or a restructured
  loop, since `for x := f(); x != nil` is not valid Go.
- The `defined` vs truthiness distinction on `shift` is preserved by lowering
  to `for len(q) > 0 { item := q[0]; q = q[1:]; ... }`. The entry deliberately
  contains both spellings so the converter's output can be checked against
  Perl's differing results.
