# 23-url-normalize

**Domain:** CDN/web operations. Reads a column of raw URLs, parses each
with a hand-rolled `TinyURL` class in a sibling `.pm`, applies the RFC
3986 normalisations that matter for cache keys (scheme/host case,
default-port removal, dot-segment resolution, percent-triplet folding,
query-parameter sorting, fragment removal), prints the canonical form per
line, then groups lines that collapse to the same cache key. Garbage
lines are rejected with a reason rather than being fatal; exit 1 when
anything was rejected.

## Constructs exercised
- **Two-file program**: `use FindBin; use lib $FindBin::Bin;` to load
  `TinyURL.pm` sitting next to the script, plus a `package`/`1;` module
  with a `parse` constructor that `bless`es or `die`s.
- `eval { TinyURL->parse($raw) }` as an expression, with `$@` chomped
  and stored -- die-as-exception, caught per line.
- Hand-written one-line accessors (`sub scheme { $_[0]{scheme} }`),
  method chaining (`$url->normalize->canon`) made possible by
  `return $self`.
- `split /#/, $rest, 2` then `split /\?/, $rest, 2` -- limited splits in
  a specific order, because a `?` after a `#` belongs to the fragment.
- Aliasing through a list of scalar refs: `for my $part (\$self->{path},
  \$self->{query}) { $$part =~ s/.../ }` mutates hash slots in place.
- `s///ge` twice over the same string: `chr hex $1` to decode unreserved
  triplets, then `'%' . uc $1` to case-fold the survivors.
- `split m{/}, $path, -1` with a negative limit to keep the trailing
  empty segment, and a `pop`-based dot-segment stack.
- Sorting `[key, value]` pairs with a two-level comparator that treats a
  missing value as `''` via `//`.
- `push @{ $by_canon{$canon} }, $n` (autovivification) and
  `sort grep { @{ $by_canon{$_} } > 1 } keys %by_canon` for the
  duplicate report.
- `printf "  line %2d: %s\n", @$_ for @rejects;` -- an arrayref flattened
  straight into printf's argument list by a statement-modifier `for`.

## Conversion challenges
- The chained-mutator style (`normalize` returning `$self` after three
  private in-place steps) wants either pointer receivers returning the
  receiver, or a redesign into pure functions; picking one changes every
  call site.
- Scalar refs used as an alias list has no direct Go form. The natural
  port is a slice of `*string`, which works, but only if the converter
  recognises that `$$part =~ s///` writes back through the reference.
- `//` on `$self->{port}`, `$self->{userinfo}` and `$_->[1]` is
  three-state (absent / present-but-empty / present): a Go port needs
  pointers or an explicit "set" flag, not the empty string as a sentinel.
- The percent-decode regex encodes an ASCII range as hex character
  classes (`[46][1-9A-Fa-f]`, `[57][0-9Aa]`); it is far clearer in Go as
  a byte predicate, but rewriting it is a semantic decision the converter
  must document rather than perform silently.
- `die` inside a constructor caught by `eval` at the call site is Go's
  `(*TinyURL, error)`; the message text is part of the observable output
  (it feeds the rejected-lines report), so error strings must be
  preserved verbatim, trailing newline included.
- The duplicate report interpolates an array inside a string
  (`"lines: @{ $by_canon{$canon} }"`), which uses `$"` (a space) as the
  separator -- easy to lose in translation.
