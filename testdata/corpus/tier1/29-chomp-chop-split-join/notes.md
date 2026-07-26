# 29-chomp-chop-split-join

## What this exercises
`chomp` (removes a trailing `$/`, returns the number of characters removed,
mutates in place, works on a whole array), `chop` (removes and returns the last
character unconditionally), and the full range of `split` behaviours: trailing
empty fields being **dropped by default** but kept with a negative LIMIT, a
positive LIMIT capping the field count, the empty pattern exploding into
characters, `/\s+/` keeping a leading empty field, and the special-cased
literal `' '` pattern behaving like awk. Finally a CSV-ish round trip over
STDIN.

## Perl constructs
- `chomp(my $x = $y)` -- declare, assign and chomp in one expression
- `chomp @array`
- `split PATTERN, STRING, LIMIT`
- the `split ' '` special case (a literal one-space *string*, not `/ /`)
- `join SEP, LIST`

## Go concepts a converter must teach
- `chomp` is `strings.TrimSuffix(s, "\n")`, **not** `strings.TrimSpace` and not
  `TrimRight(s, "\n")` (which would remove several). It mutates its argument in
  Perl, so `chomp $x` is `x = strings.TrimSuffix(x, "\n")` and `chomp @a` is a
  loop over indices.
- `chop` is a rune-aware "drop last character" helper returning that character.
- **`split` default trailing-empty removal has no Go equivalent.**
  `strings.Split("a,b,,c,,", ",")` keeps all six fields, matching Perl's
  `LIMIT = -1`, not Perl's default. Every bare `split` must be lowered as
  "split then trim trailing empty fields".
- `split /\s+/` -> `regexp.Split(s, -1)` (keeps the leading empty field);
  `split ' '` -> `strings.Fields` (does not). Conflating these is a real bug.
- `split //` -> `strings.Split(s, "")`.
- Positive LIMIT is `strings.SplitN(s, sep, n)` and Go's N has the same meaning.
- `join` is `strings.Join`; joining an empty list gives `""` in both.
