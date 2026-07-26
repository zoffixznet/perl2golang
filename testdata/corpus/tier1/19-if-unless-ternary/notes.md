# 19-if-unless-ternary

## What this exercises
`if`/`elsif`/`else` chains, `unless` including `unless`/`else`, `unless (@list)`
on an array, the ternary conditional including a chained (nested) ternary
formatted as a decision table, and the ternary used as an **lvalue** so that
`($cond ? $a : $b) += $i` picks which variable to accumulate into.

## Perl constructs
- `if` / `elsif` / `else` (note the spelling: `elsif`, one `e`)
- `unless COND BLOCK` and `unless ... else`
- `?:` and chained `?:`
- ternary in lvalue position (`($c ? $x : $y) = ...`)
- braces are mandatory on every block; there is no brace-less `if`

## Go concepts a converter must teach
- `if`/`else if`/`else` maps directly; `elsif` is just spelled differently.
- **Go has no ternary operator.** Every `?:` becomes either an if-statement
  (requiring the expression to be hoisted into a temporary before its use site)
  or a small generic helper `func ifElse[T any](c bool, a, b T) T` -- which
  loses short-circuiting, so it is only safe when both arms are side-effect
  free. This entry has a chained ternary specifically to make the hoisting
  problem concrete.
- **Go has no lvalue conditional.** `($i % 2 ? $odds : $evens) += $i` must
  become an if-statement with two separate `+=` lines, or the two variables
  must be reworked as a 2-element array indexed by `i % 2`.
- `unless` is `if !cond`. `unless ... else` inverts to `if cond { else-block }
  else { then-block }`; converters that mechanically negate and keep the block
  order produce backwards code.
- `unless (@list)` is `if len(list) == 0`.
