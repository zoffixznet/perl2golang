# 10-format-write: `format` / `write` report DSL

Group: **A — genuinely impossible without an interpreter**

## Construct
Two compile-time format declarations — `format STDOUT_TOP` (line 9, the
per-page header) and `format STDOUT` (line 14, the row picture) — rendered by
`write` (line 21). Picture lines (`@<<<<<<<<<<`, `@>>`) declare left/right
justification and width; the line after each picture names the package variables
to fill it; `write` also maintains per-filehandle pagination state (`$%`, `$=`,
top-of-form on page break).

## Why it resists conversion to Go
`format` is a separate language embedded in Perl with its own parser, its own
variable-binding rule (the picture binds to package variables by name at
compile time), and hidden per-filehandle state (lines-left-on-page, automatic
header emission). Go has nothing comparable; `text/template` or `fmt` can
approximate the LAYOUT but nothing tracks pagination, `^` continuation fields,
or `~~` repeat lines. Translating the general case means reimplementing the
format engine.

## What the converter should do
- Category: **refuse-statement** for each `format` declaration and each `write`,
  converting the rest of the file (the loop and data are ordinary Perl).
- The stub for `write` must panic with a message naming the format it would
  have used.
- Optional narrowing: formats using ONLY plain `@<`/`@>`/`@|` fields (as here)
  may be lowered to a generated `fmt.Sprintf` with computed widths plus an
  explicit header-once emulation, reported as an approximation. If lowered, the
  output must match `expected_stdout` byte-for-byte, including column spacing.
- Forbidden: dropping the header, or converting the picture line as if it were
  literal text.

## Ideal diagnostic (word for word)
> input.pl:14: error P2G-E107: 'format STDOUT' declares a report template in
> Perl's format sub-language (picture line '@<<<<<<<<<< @>>  @>>>>>'); 'write'
> at input.pl:21 renders it with pagination state Go does not have. Replaced
> 'write' with a panicking stub. Rewrite the report with printf-style column
> widths ('%-11s %3s  %6s'), which this tool can convert.

## What a human should do instead
Rewrite as `printf "%-11s %3d  %6s\n"` rows plus an explicit header print —
mechanical, and it converts. Pagination, if actually used, becomes an explicit
line counter.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): header block then three aligned rows
(`apple         3    1.50` etc.). Exact column spacing is the fidelity bar for
the optional lowering.
