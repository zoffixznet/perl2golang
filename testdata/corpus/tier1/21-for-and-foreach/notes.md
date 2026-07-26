# 21-for-and-foreach

## What this exercises
C-style three-part `for` (including a comma-operator init and step),
`foreach my $x (@list)`, `foreach (@list)` using the implicit `$_`, index-driven
iteration over `0 .. $#a`, **loop variable aliasing** (assigning to the loop
variable modifies the array), nested loops, `reverse` over a range, string
ranges, and the fact that `foreach` localises `$_` so the outer value survives.

## Perl constructs
- `for (init; cond; step)` with `,` sequencing multiple expressions
- `foreach my $x (LIST)` / `foreach (LIST)` / `for my $x (LIST)` -- all the same
- `$_` as the implicit topic
- aliasing: `for my $n (@nums) { $n *= 10 }` mutates `@nums`
- `$_ .= "!" for @words` -- statement modifier that also aliases
- `'aa' .. 'ad'` string range

## Go concepts a converter must teach
- C-style `for` maps one-to-one, except Go's post statement cannot be a comma
  list -- `$i++, $j--` becomes two statements or `i, j = i+1, j-1`.
- **Aliasing is the trap.** Perl's `foreach` variable is an alias to the actual
  element; Go's `for _, v := range s` gives a *copy*, so `v *= 10` is a no-op.
  Any loop whose body assigns to the loop variable must be lowered to
  `for i := range s { s[i] *= 10 }`. Detecting this requires a write analysis on
  the loop variable, and getting it wrong silently drops the mutation.
- `$_` is a dynamically scoped global. Go has no such thing, so the converter
  must introduce an explicit variable and be careful when a nested construct
  also uses `$_`.
- `0 .. $#a` is `for i := 0; i < len(a); i++`; `reverse 1 .. 5` is a downward
  loop or a generated + reversed slice.
- String ranges (`'aa' .. 'ad'`) need the magic-increment helper plus the
  length-based termination rule Perl uses.
