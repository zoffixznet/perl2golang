# 21-fuzzy-match

CLI "did you mean?" engine: Text::Abbrev prefix table, Levenshtein DP,
LCS-ratio similarity, three-key suggestion ranking, stdin-driven.

## Constructs exercised
- `Text::Abbrev::abbrev` building the unambiguous-prefix map (core module
  with no Go analogue; behavior must be reimplemented: every unambiguous
  prefix maps to its command, ambiguous prefixes are absent)
- Levenshtein with a rolling two-row DP (`@prev`/`@cur`, `$prev[-1]`)
- LCS length in a SINGLE row with a `$diag` temporary -- an in-place DP
  that converters must not "simplify" into the naive two-dimensional form
  incorrectly
- `substr( $s, $i - 1, 1 )` character indexing throughout (byte semantics)
- hashref-record pipeline: `map { {...} }` building dicts, three-key sort
  (dist asc, sim desc, name asc), `@scored[0..2]` slice then `grep`
  threshold
- hash slice inside an interpolated sprintf:
  `@{$_}{qw(cmd dist sim)}`
- `grep { $_ eq $try } @commands` membership test in boolean context
- guard `elsif ( my $full = $abbrev{$try} )` -- assignment-in-condition
- float formatting `%.2f` of a ratio; division guarded by `$longer ? ... : 1`

## Conversion challenges
- Text::Abbrev semantics (including that a full command is always its own
  key, and shared prefixes like 'st' vanish) must be replicated exactly --
  the invariant checks at the bottom pin this
- slice-past-end behavior: `@scored[0..2]` on a shorter-than-3 result list
  yields undefs that `grep` must tolerate; Go slicing panics instead --
  needs min() bounds
- DP index arithmetic off-by-ones when translating 1-based loops over
  `length` to Go's 0-based `len`
- anonymous hashref records -> struct slices; the three-key sort ->
  sort.Slice with chained comparisons; float compare direction flips
- byte-wise `substr` equality vs Go range-over-string producing runes

## Go teaching opportunities
- classic DP translations, struct-based ranking pipelines, why
  suggestion engines belong in table-driven tests
