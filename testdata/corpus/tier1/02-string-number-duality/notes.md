# 02-string-number-duality

## What this exercises
Perl scalars carry both a string and a numeric value and convert on demand.
`"42" + 8` is 50; `"42" . 8` is "428". Numeric literal forms: underscores as
digit separators, `0x`, `0`-prefixed octal, `0b` binary, scientific notation.
The `hex`/`oct` functions parse strings. `length` on a number stringifies it
first.

## Perl constructs
- implicit string<->number conversion at the operator, not at the variable
- `+` is always numeric, `.` is always string -- the operator picks the context
- `==` vs `eq` on `"007"`
- `1_000_000`, `0xff`, `0755`, `0b1010`, `1.5e3`
- `hex`, `oct` (oct also understands `0x` and `0b` prefixes)
- `++` on a numeric-looking string

## Go concepts a converter must teach
- Go has no implicit conversion. Every Perl scalar that is used both ways needs
  either a decision (pick `string` or a numeric type and insert
  `strconv.Atoi` / `strconv.FormatInt` at the crossings) or a boxed
  "PerlScalar" value type.
- The operator, not the variable, determines context. A converter can often use
  this to infer a static type: a scalar only ever fed to `+`/`*` is an int or
  float; a scalar only fed to `.` is a string.
- `"007" == 7` is true but `"007" eq "7"` is false -- Go's `==` on strings is
  the `eq` semantics, so numeric comparisons need an explicit parse.
- `0755` in Perl is octal; in Go the literal is `0755` or `0o755` -- same value,
  but `0755` parsed out of a *string* needs `strconv.ParseInt(s, 8, 64)`.
- `hex("ff")` -> `strconv.ParseUint("ff", 16, 64)`; `oct` is a small dispatcher
  on the prefix.
