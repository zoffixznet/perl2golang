# 06-prototypes: prototypes that alter parsing and argument passing

Group: **A — genuinely impossible without an interpreter** (the parse-changing
aspect; the individual calls ARE convertible once the prototype is honoured)

## Construct
Three prototypes, each changing the CALL SITE, not just the sub:
- `sub apply (&@)` (line 8): makes `apply { ... } 1, 2, 3` legal — a bare block
  becomes a code ref.
- `sub zipfirst (\@\@)` (line 15): the caller writes `zipfirst(@x, @y)` but the
  sub receives TWO ARRAY REFS, not four flattened scalars.
- `sub one ($)` (line 23): forces scalar context, so `one(@stuff)` passes the
  COUNT 3, not the first element.

## Why it resists conversion to Go
The meaning of every call site depends on a declaration the converter must have
already processed and honoured. A converter that ignores prototypes will parse
`apply { $_[0] * 2 } 1, 2, 3` as a syntax error or a hash, will flatten
`zipfirst(@x, @y)` into four arguments, and will pass `7` instead of `3` to
`one`. All three are silent wrong-answer bugs, not crashes.

## What the converter should do
- Category: **approximate** (with a hard prerequisite): the converter MUST
  implement prototype-aware parsing before converting any file that declares
  prototypes. If its parser does not honour prototypes, it must **refuse-file**
  any file containing a prototype rather than mis-parse it.
- `(&@)` converts cleanly: `func apply(fn func(...), items ...)`.
- `(\@\@)` converts to slice parameters (Go slices are already references).
- `($)` must insert an explicit scalar-context conversion (`len(stuff)`) at the
  call site.
- Report entry required per prototype, stating the call-site rewriting applied.

## Ideal diagnostic (word for word)
For the refusing converter:
> input.pl:8: error P2G-E202: sub 'apply' declares prototype '(&@)', which
> changes how its call sites parse. This converter does not honour prototypes,
> so it cannot parse this file correctly and will not guess. Remove the
> prototype or convert this file manually.

For the honouring converter, a report note per site, e.g.:
> input.pl:24: note: 'one(@stuff)' passes scalar context due to prototype '($)':
> converted as the element COUNT (3), not the first element.

## What a human should do instead
Drop prototypes in the Perl source (plain `sub apply { my ($fn, @items) = @_ }`
called as `apply(sub { ... }, 1, 2, 3)`); the file then parses the boring way
and converts mechanically.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `doubled=2 4 6`, `zip=10/30`, `one=3`. The `one=3`
line is the tripwire: a prototype-blind conversion prints `one=7`.
