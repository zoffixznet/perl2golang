# 02 - named arguments via a hash

## What this exercises
The "named parameters" convention: pass `key => value` pairs, slurp them into
a `%args` hash, and merge them over a defaults hash.

## Perl constructs
- `my %args = @_;` turning the flat argument list into a hash
- defaults by hash merge: `my %user = (%defaults, %args);` (later keys win)
- `die "...\n" unless defined $user{name};` required-argument validation
- returning a hash (`return %user`) - which really returns a flattened list
- returning a hashref (`return \%user`) and using `->{}` on it
- passing a hash into a sub and re-slurping it (`my (%u) = @_`)
- `sort keys %$href` for deterministic ordering
- `grep`/`map` over `sort keys` to report which options were supplied
- `50_000` numeric literal with underscores
- `sprintf`/`printf` with `%-8s`, `%5d` column formatting
- an array of arrayrefs (`@requests`) flattened back into arguments via `@$req`

## Go concepts a converter must teach
- `key => value` argument lists have no Go equivalent. The idiomatic target is
  an options struct plus a constructor that fills defaults, or the functional
  options pattern. A converter has to *infer the struct* from the keys the sub
  reads.
- `(%defaults, %args)` merge order is a right-wins map merge; in Go this is a
  loop over the override map, or per-field `if opt.X == zero { opt.X = def }`.
  The zero-value trap is real: Perl can distinguish "not supplied" from
  "supplied as 0"; Go cannot without pointers.
- `return %user` (a hash) versus `return \%user` (a reference) are different
  things in Perl but both become `map[string]any` or a struct in Go. The
  copy-vs-alias semantics differ and matter if the caller mutates.
- Hash key iteration order is undefined in Perl and randomised per process, so
  every hash walk in the corpus is wrapped in `sort`. Go maps are also
  randomised, so this discipline carries over directly.
- `die` here is a fatal error, not an exception to catch: map to
  `log.Fatalf`/`os.Exit` or a returned `error`, depending on the caller.
