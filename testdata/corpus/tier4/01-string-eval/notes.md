# 01-string-eval: `eval EXPR` of a computed string

Group: **A — genuinely impossible without an interpreter**

## Construct
`eval $code` where `$code` is a string assembled at runtime (lines 11 and 18 of
`input.pl`). The operator compiles and runs Perl source that does not exist until
the program is already running.

## Why it resists conversion to Go
Go has no `eval`. The text being evaluated is a runtime value; no static analysis
can enumerate what it will contain (here it is built from a loop variable and a
`join`, in real scripts it comes from config files or user input). Translating it
would require shipping a Perl interpreter inside the Go binary.

## What the converter should do
- Category: **refuse-statement**.
- Convert the rest of the file normally.
- Replace each `eval EXPR` site with a stub that panics at runtime, carrying the
  original Perl expression text in the panic message, e.g.
  `panic("perl2go: unconverted 'eval EXPR' at input.pl:11: eval $code")`.
- Emit one conversion-report entry per site (lines 11 and 18).
- It must NOT try to pattern-match the string being built and "helpfully" inline
  it: that is guessing, and it breaks the moment the string changes shape.
- Special case it MAY implement: `eval EXPR` where the argument is a compile-time
  constant string literal can be treated as inline code. That case does not occur
  in this file, so no such conversion may appear here.

## Ideal diagnostic (word for word)
> input.pl:11: error P2G-E101: 'eval EXPR' compiles and runs Perl source built at
> run time; this cannot be translated ahead of time. The statement was replaced
> with a stub that panics if reached. Rewrite without string eval (a dispatch
> table keyed by operator would work here), or keep this script in Perl.

A second identical diagnostic must be issued for line 18.

## What a human should do instead
Replace the string-built arithmetic with a dispatch table:
`my %op = ('+' => sub { $_[0] + $_[1] }, ...)` in Perl, or in Go a
`map[string]func(float64, float64) float64`. For the `"2*3*4"` case, compute the
product directly instead of building source text.

## Observed with perl 5.42.2 (x86_64-linux)
See `expected_stdout` (exit 0). Useful for verifying a converted dispatch-table
rewrite, not for the converter itself: the converter must refuse these
statements, not reproduce them.
