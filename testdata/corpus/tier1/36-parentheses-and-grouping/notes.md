# 36-parentheses-and-grouping

## What this exercises
The two jobs Perl gives one pair of parentheses. Around a single expression in
scalar context they group, and nothing more: `($n - $m) / $d` divides a number.
In list context, or with commas inside, they build a list. The surrounding
context decides, so the same three characters mean different things two lines
apart. The entry also covers what an operator does to an array on either side
of it: an operator imposes scalar context, so `@items + 1` is a count plus one
and `"there are " . @items` is a count in text.

## Perl constructs
- Grouping in arithmetic, exponentiation, unary minus, `%` and comparison
- Precedence with and without the parentheses, side by side
- The floored-division identity `($a - $a % $b) / $b`
- An array as an operand of `+`, `*`, `==` and `.`
- `$#items` beside `scalar(@items)`
- The comma operator in scalar context, yielding its last value after
  evaluating everything before it
- Parentheses that really are a list: `my @three = (1, 2, 3)`, `my @one = (9)`,
  `my ($only) = @items`

## Go concepts a converter must teach
- **Go has no context**, so the converter has to decide for itself which of the
  two meanings each pair of parentheses had. Reading a grouped expression as a
  one-element list produces Go that compiles and computes something else, which
  is the worst kind of wrong answer.
- An operator puts both of its sides in scalar context. An array there becomes
  `len(items)`, which is the same claim Perl is making and reads more plainly
  in Go than it does in Perl.
- Go's `%` takes the sign of its left operand and Perl's takes the sign of its
  right, so the floored-division identity only survives translation through a
  helper. The entry checks the identity itself rather than the operator, which
  is what a reader would do to convince themselves.
- The comma operator has no Go form: the discarded operands become statements
  before the expression, which is what Go would have made you write anyway.
- Go's precedence differs from Perl's for `**` and unary minus, so grouping
  that was redundant in Perl may not be redundant in the Go.
