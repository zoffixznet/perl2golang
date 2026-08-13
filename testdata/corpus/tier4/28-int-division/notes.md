# 28-int-division: `/` is float division; `use integer` flips the rules

Group: **C - convertible, but the naive conversion is subtly wrong**

## Construct
`7 / 2` is 3.5 - Perl's `/` is ALWAYS floating point (lines 7-8). Truncation
is explicit via `int()` (line 9, toward zero). Inside `use integer`
(lines 11-14) the SAME operators switch to C semantics lexically: `/`
truncates, and `%` changes to C behaviour too (`-7 % 2` is `-1` inside,
`1` outside - line 16). The average idiom on line 18 is where real scripts
get burned.

## Why the naive conversion is subtly wrong
Two ints dividing in Go is integer division: a converter that maps `$a / $b`
onto Go `a / b` for int-typed operands turns 3.5 into 3 silently - averages,
percentages and rates all shift. The inverse trap: inside `use integer`,
float division would be the wrong translation. The pragma is LEXICAL, so
correctness requires scope tracking, not file-level flags.

## What the converter should do
- Category: **convert-verify**:
  - Outside `use integer`: every `/` produces float64 (convert operands),
    regardless of operand types. `int(expr)` maps to explicit truncation
    toward zero (`int64(expr)` conversion in Go truncates toward zero for
    positive and negative - matching).
  - Inside `use integer`: `/` maps to native Go integer division and `%` to
    native Go `%` (C semantics - see entry 27 for the outside-`use integer`
    helper).
  - The report must note each `use integer` scope and the operator semantics
    switch.
- Forbidden: int/int division emitted for plain Perl `/` because both
  operands "look like" integers.

## Ideal diagnostic (word for word)
> input.pl:18: note P2G-W404: '/' is floating-point division in Perl even for
> integer operands; '(3 + 4) / 2' converted as float64 yielding 3.5. Integer
> division in Go would yield 3. Sites inside 'use integer' scopes (lines
> 11-14) use native integer division as Perl does there.

## What a human should do instead
When porting by hand, decide per division site whether the mathematical value
or the truncated value was intended, and write `float64(a)/float64(b)` or
`a/b` explicitly. Treat every `use integer` block as a marker that the
original author wanted C arithmetic.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `7/2 = 3.5`, `-7/2 = -3.5`, `int(-7/2) = -3`,
`integer 7/2 = 3`, `integer -7/2 = -3`, `integer -7%2 = -1`,
`plain -7%2 = 1`, `avg = 3.5`. The `integer -7%2` vs `plain -7%2` pair pins
the pragma's effect on `%`.
