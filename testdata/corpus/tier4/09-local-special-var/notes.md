# 09-local-special-var: `local` on punctuation variables changing distant behaviour

Group: **A - genuinely impossible without an interpreter** (without a runtime
that models the punctuation globals)

## Construct
`local $" = "|"` (line 12) changes how LIST INTERPOLATION works inside
`render()` - a sub that never mentions `$"`. `local $, = "-"` and
`local $\ = "<END>\n"` (lines 20-21) change what `print` inside `show()` emits.
The affected subs are textually untouched; only the dynamic scope of the caller
changes their output.

## Why it resists conversion to Go
`$"`, `$,`, `$\` are implicit parameters to EVERY interpolation and every
`print` in the program. Go's `fmt` has no ambient separator state; faithfully
converting requires the generated code for every string interpolation and every
print to consult a runtime-state struct - a decision that affects all generated
code, not just this file. A converter that translates `"@_"` as
`strings.Join(args, " ")` with a hard-coded space is silently wrong the moment
anyone locals `$"`.

## What the converter should do
- Category: **todo**.
- Minimum honest behaviour: convert `render`/`show` with the DEFAULT separator
  semantics, but flag every `local` of a punctuation variable with a diagnostic
  and insert a `// TODO:` comment at the local site AND at each affected
  builtin use, stating that the converted code hard-codes the defaults.
- Better behaviour (optional): a runtime shim struct
  (`perlrt.ListSep`, `perlrt.OFS`, `perlrt.ORS`) consulted by generated
  interpolation/print helpers, with save/restore via `defer`. If the shim is
  emitted, output must match `expected_stdout` exactly.
- Forbidden: dropping the `local` lines and reporting success.

## Ideal diagnostic (word for word)
> input.pl:12: warning P2G-W301: 'local $"' changes list-interpolation
> behaviour for all code called from this block (here: render(), input.pl:8).
> The generated Go hard-codes the default separator " " and will print
> 'local: 1 2 3' where Perl prints 'local: 1|2|3'.
> TODO added: thread a separator parameter through render(), or enable the
> perlrt punctuation-variable shim.

Analogous diagnostics for lines 20 and 21 (`$,`, `$\` affecting show()).

## What a human should do instead
Pass the separator explicitly: `render_with(sep, list)` /
`strings.Join(parts, sep)`; replace `$\` tricks with an explicit suffix
argument. Ambient state becomes a parameter.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0). Load-bearing lines: `local: 1|2|3` (vs `plain:` and
`reset:` with spaces) and `a-b-c<END>` between two `abc` lines.
