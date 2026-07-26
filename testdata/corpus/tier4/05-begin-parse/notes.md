# 05-begin-parse: BEGIN block that changes how later code parses

Group: **A — genuinely impossible without an interpreter**

## Construct
The `BEGIN` block (lines 8-16) runs DURING compilation and — based on
`$ENV{TAG_GREEDY}` — defines `tag` either with a `($)` prototype or without one.
The call `my @out = ( tag "a", "b" )` on line 20 then parses as either
`(tag("a"), "b")` or `(tag("a", "b"))`. One source text, two parse trees, chosen
by the environment at compile time.

## Why it resists conversion to Go
A converter must parse the file, but this file cannot be parsed without
EXECUTING the BEGIN block first, and the BEGIN block's outcome depends on runtime
state (the environment). There is no single correct AST to convert. This is the
fundamental "only perl can parse Perl" case.

## What the converter should do
- Category: **refuse-file**.
- Any `BEGIN` block containing conditional sub definition, `eval`, glob
  assignment, or `@INC`/import manipulation makes subsequent parse results
  unreliable; the converter must detect that and refuse the whole file with a
  diagnostic naming the BEGIN block and the first construct whose parse depends
  on it.
- Safe exception it MAY implement: BEGIN blocks that only assign constants or
  contain `use`-style boilerplate with no conditional definition. This file does
  not qualify (the definition is under `if`).
- It must NOT parse the file with its own fixed guess about `tag`'s prototype
  and silently emit one of the two meanings.

## Ideal diagnostic (word for word)
> input.pl:8: error P2G-E201: this BEGIN block conditionally defines 'tag'
> (with and without a prototype) while the file is being compiled, so the parse
> of line 20 depends on the environment at compile time. The file cannot be
> converted ahead of time. Remove the conditional definition or make both
> branches define 'tag' with the same signature.

## What a human should do instead
Define `tag` once, unconditionally, with an explicit argument list, and put the
behavioural switch INSIDE the sub body (`if ($ENV{TAG_GREEDY}) { ... }`); then
the file has one parse and converts normally.

## Observed with perl 5.42.2 (x86_64-linux)
Default (TAG_GREEDY unset): `[a]|b` — captured in `expected_stdout` (exit 0).
With `TAG_GREEDY=1` the SAME file prints `[a,b]`. Those two outputs are the
proof that no single translation is faithful.
