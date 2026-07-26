# 23-statement-modifiers

## What this exercises
Every statement-modifier form: `EXPR if COND`, `EXPR unless COND`,
`EXPR while COND`, `EXPR until COND`, `EXPR for LIST`, `EXPR foreach LIST`.
Includes the modifier-`for` aliasing `$_`, a modifier-`while` with a
side-effecting condition (`$i-- > 0`), and modifiers used to build arrays,
hashes and strings.

## Perl constructs
- all six statement modifiers
- `$_` bound by a modifier `for`
- a modifier `while` whose condition mutates (`$i-- > 0`) so the loop and the
  final value of `$i` are both observable
- `push @a, EXPR for LIST` as a one-line map

## Go concepts a converter must teach
- Statement modifiers are just an inverted syntax for the corresponding block
  form; the lowering is a straight `if`/`for` with the body on the inside.
  A converter's parser has to handle them because they are extremely common in
  real Perl, but the IR should normalise them away immediately.
- `EXPR for LIST` still binds `$_`, so the expression may reference a variable
  that never appears in the source line. That is a scoping surprise worth
  handling explicitly.
- `print "tick $i\n" while $i-- > 0;` shows a condition with a side effect
  running one extra time than the body -- the final `$i` is -1, not 0. Naive
  conversions to `for i > 0` lose the extra decrement.
- Perl statement modifiers cannot be stacked (`print if $a for @b` is a syntax
  error), which simplifies the grammar.
