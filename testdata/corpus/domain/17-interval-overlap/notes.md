# 17-interval-overlap

**Domain:** bioinformatics. Annotates BED peaks with overlapping GFF
genes: loads genes into a chrom -> strand -> sorted-interval-list
structure, converts BED's 0-based half-open coordinates to 1-based
closed on load (the conversion is commented at the site because it is
the classic pipeline bug), computes per-overlap bp, reports orphan
peaks, and accumulates per-gene coverage.

## Constructs exercised
- **Three-level nested structure**: `%genes{chrom}{strand}` -> arrayref
  of `[start, end, name]` triples, built by push-autovivification, then
  sorted in place via `@{...} = sort {...} @{...}`.
- In-place sort of a nested arrayref -- aliasing through the deref.
- Early-exit scan over sorted intervals (`last if $g->[0] > $pend`)
  nested inside a sorted-strand loop.
- `%{ $genes{$chrom} || {} }` -- guarded deref so unknown chromosomes
  (chr4 in the fixture) iterate zero times instead of autovivifying.
- Regex capture into list with fallback: `my ($name) = $attrs =~
  /Name=([^;]+)/;` then `$name //= sprintf ...`.
- `split /\t/` with `undef` placeholders in the receiving list to skip
  columns.
- Max/min via chained ternaries; overlap arithmetic in closed
  coordinates (`+1`).

## Conversion challenges
- The guarded deref `|| {}` prevents autovivification that a plain
  `$genes{$chrom}{$strand}` loop would cause -- converters must
  understand *why* the guard exists, since Go maps have no
  autovivification and the naive translation accidentally becomes
  correct; the teaching note is the asymmetry itself.
- Gene tuples are positional arrayrefs (`[start,end,name]`) while peak
  overlaps are hashrefs (`{name,strand,bp}`) -- mixed representations in
  one program; good Go would unify both into named structs (`Gene`,
  `Overlap`).
- In-place sorting of the nested list through a dereferenced alias:
  the Go translation sorts a slice held in a nested map -- fine, but
  only if the converter keeps slices (not copies) in the map.
- `undef` list placeholders in `split` destructuring have no Go
  equivalent -- index-based access or `_`-style discards.
- Coordinate conversion is a one-line `+1` with a comment carrying the
  domain knowledge; regression here is invisible without the fixture.
