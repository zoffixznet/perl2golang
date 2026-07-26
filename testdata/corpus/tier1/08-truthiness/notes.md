# 08-truthiness

## What this exercises
Exactly which values Perl considers false. The complete list is: `undef`,
the empty string `""`, the string `"0"`, and the number `0`. Everything else
is true -- including `"0.0"`, `"00"`, `"0E0"`, `" "` (a single space), and the
famous `"0 but true"`.

Also shows what boolean operators actually *return*: `1` for true and the empty
string `""` for false, so `length((1==2))` is 0 and `(1==2) + 1` is 1.

## Perl constructs
- boolean context in a ternary
- `tr///` used only to make an embedded newline visible in the output
- array in boolean context (true iff non-empty)
- hash in boolean context (true iff it has at least one key)
- the dualvar-ish return value of comparison operators

## Go concepts a converter must teach
- Go has no truthiness at all: `if x` requires `x` to be a `bool`. Every Perl
  conditional needs an explicit predicate. The general helper is:

      func truthy(v any) bool  // false for nil, "", "0", 0, 0.0

- Note that `"0.0"` and `"00"` are **true** in Perl. A converter that lowers
  "is this string truthy" to "is this string non-empty and not equal to zero
  after parsing" gets these wrong. The rule is purely lexical: only the exact
  one-character string `"0"` is false.
- `if (@arr)` becomes `if len(arr) > 0`; `if (%h)` becomes `if len(m) > 0`.
- Perl's false is `""`, not `0`. Code that does arithmetic on a comparison
  result relies on `"" + 1 == 1`, so lowering booleans to Go `bool` requires
  inserting a `b2i` conversion wherever the value escapes a condition.
