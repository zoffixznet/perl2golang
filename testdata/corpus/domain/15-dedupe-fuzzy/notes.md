# 15-dedupe-fuzzy

**Domain:** text/data munging. Finds likely duplicate contacts: exact
email match short-circuits; otherwise same zip + normalised-name edit
distance <= 2, with a nickname-folding table (Bob->Robert, Wm->William).
Union-find with path compression keeps clusters transitive; each cluster
elects a survivor (longest email, then lowest id). All contact data is
placeholder data.

## Constructs exercised
- Hand-rolled **Levenshtein** with the two-row DP optimisation -- tight
  loop over `split //` character arrays, manual min-of-three.
- **Union-find** over string ids: `%parent` hash, recursive `find` with
  path compression (`$parent{$x} = find($parent{$x})`), deterministic
  root selection (`($rx,$ry) = ($ry,$rx) if $ry lt $rx`).
- O(n^2) pairwise loop with `@contacts[$i,$j]` array slice destructuring.
- Reference-identity comparison `$m == $survivor` on hashrefs to mark the
  survivor row.
- `my ($survivor) = sort {...} @members` -- taking the head of a sorted
  list via list-assignment truncation.
- Scalar-context `grep` (`my $singles = grep {...} keys %cluster`) for
  counting without collecting.
- Normalisation chain: `lc`, punctuation strip, whitespace collapse,
  first-token nickname lookup.

## Conversion challenges
- `$m == $survivor` compares references numerically (addresses); Go can
  compare pointers only if contacts are `*Contact` -- a converter storing
  values in slices silently breaks survivor marking. This entry is a
  direct probe of value-vs-pointer modelling.
- The Contact record (`id/name/email/zip/norm`) is a named-struct
  candidate used across four subs.
- `for my $i (1 .. @s)` -- an array evaluated in range context (count);
  `$prev[-1]` negative index; `@prev = @cur` array copy each row: all
  small semantic traps in the DP loop.
- Recursive `find` mutating the global `%parent` during reads (path
  compression has store-inside-lookup); in Go this is natural with a map
  but the recursion must not be "simplified" into a read-only loop.
- Cluster keys come from union-find roots; determinism is engineered via
  the lexicographic root rule -- notes for the converter on *why* output
  is stable despite hash storage.
- Scalar-vs-list `grep` context decides count vs. list -- context
  awareness required at two adjacent lines.
