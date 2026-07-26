# 10-ini-config

INI-style configuration parser: DEFAULT fallback, `extends` inheritance
with cycle detection, `${var}` interpolation to fixpoint, typed getters.

## Constructs exercised
- line-oriented parsing with `$.` (current line number) in error messages
- `s/^\s+|\s+$//g` trim, comment/blank skipping, `[\w.]+` section names
- hash-merge layering via list-flattening: `%merged = ( %merged, %$raw )`
  (later keys win) -- pure Perl idiom with no Go one-liner
- recursion with a `$seen` hash for cycle detection (`$seen->{$name}++` as
  test-and-set)
- interpolation loop: `while ($v =~ /\$\{(\w+)\}/)` with `\Q...\E` quoting
  in the substitution and a runaway guard
- `//=` default init, `delete` on a hash key, `exists` for duplicate-key
  detection, `die` carrying file:line context
- typed getter validating `/^-?\d+$/` then numifying with `+ 0`
- probe table of `[label, coderef]` pairs run under `eval`

## Intentional semantic trap (do not "fix")
`database.dev` shows `dsn = pg://db.internal:5432/ordersvc_prod`: the parent
section is *fully resolved and interpolated before* the child's overrides
merge, so `${host}` in the parent's `dsn` binds to the parent's `host`.
A converter that naively merges raw keys first and interpolates once at the
end produces `pg://localhost:5432/ordersvc_dev` -- observably wrong here.
Evaluation ORDER is part of the spec.

## Conversion challenges
- hash-spread merging -> explicit map-copy loops in Go, preserving
  later-wins ordering
- `\Q$ref\E` regex-metachar quoting -> regexp.QuoteMeta, or avoid regex
- `$.` line-number variable must be tracked manually
- eager-vs-lazy interpolation ordering (see trap above)

## Go teaching opportunities
- bufio.Scanner with line counting; map layering; error wrapping with
  fmt.Errorf("%s:%d: ...") mirroring Perl's die strings
