# What this entry exercises

Accumulators, which is the shape most loops in a real script end in. A total
fed from text fields, a fractional total fed from the same split, a counter,
an average built with `/=`, and every other compound arithmetic operator
once each, ending with text accumulation so the string case stands beside
the numeric ones.

The point is that Perl's `+`, `-`, `*`, `/`, `%` and `**` are numeric
operators and nothing else. `$bytes += $size` reads a number out of `$size`
whatever `$size` was spelled as, so the total is a number and never text,
and a converter that records "this variable was seen holding a string"
because a string appeared on the right of a `+=` ends up declaring the most
common variable in Perl as holding anything at all.

What it costs to convert:

- `+=` over text becomes a parse on the right and a numeric variable on the
  left, with the parse visible where Perl did it silently
- `/=` produces a fraction from two whole numbers, so the variable it
  assigns to is floating-point even when everything feeding it is not
- `%=` and `**=` on whole numbers stay whole, and the result has to be
  converted back when the variable is floating-point for another reason
- `.=` is the one that really does make the variable text, and it is the
  reason the numeric cases cannot simply be assumed

## Go concepts to teach

- `explicit-conversions-no-coercion` - the parse that appears on every `+=`
- `static-types-and-zero-values` - why the accumulator has to pick one type
- `strconv-parsing` - what the parse is standing in for
