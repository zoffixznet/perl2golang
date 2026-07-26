# 09-boolean-logic

## What this exercises
`&&`/`||`/`!` versus the low-precedence word forms `and`/`or`/`not`, the fact
that `&&` and `||` return one of their *operands* rather than a boolean,
short-circuit evaluation proven with a counter, `xor`, and the classic
precedence trap `my $r = 0 or note(...)` which parses as
`(my $r = 0) or note(...)`.

## Perl constructs
- `&& || !` (high precedence, tighter than `=`)
- `and or not xor` (lower precedence than `=`)
- `EXPR and EXPR` / `EXPR or EXPR` used as flow control statements
- `... if not COND` statement modifier
- short-circuit with side-effecting sub calls
- `$_[0]` to read a sub argument

## Go concepts a converter must teach
- `&&` / `||` in Go are strictly `bool`-typed and *return* `bool`. Perl's
  `"" || "default"` returning the string `"default"` has no direct Go form;
  it becomes an if-statement or a generic helper.
- Short circuiting works the same way in both languages, so side-effect order
  is preserved by a direct lowering -- but only once the operands are booleans.
- `and`/`or` versus `&&`/`||` is *purely* a precedence difference in Perl. When
  they appear in the flow-control idiom (`$x > 3 and print ...`) they lower to
  a plain `if`. When they appear inside an assignment they change what gets
  assigned, and a converter that treats them as synonyms produces wrong code --
  this entry has one of each so the difference is visible in the output.
- `not` is `!` with different binding; `xor` is Go's `!=` on two bools.
- Go has no `unless`; `if not COND` becomes `if !cond`.
