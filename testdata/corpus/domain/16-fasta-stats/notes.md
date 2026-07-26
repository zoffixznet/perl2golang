# 16-fasta-stats

**Domain:** bioinformatics. FASTA assembly QC: per-contig length, GC%
(excluding Ns from the denominator), N and ambiguity counts, longest N
run (gap signature), plus assembly-level totals, min/max/mean, N50/L50,
and overall GC. Handles soft-masked lowercase, blank lines inside
records, and a zero-length record (header with no sequence).

## Constructs exercised
- Streaming FASTA parser with a `$cur` hashref that outlives loop
  iterations; order preserved via a parallel `@order` array beside the
  `%seq` hash (insertion-order-preserving idiom again, deliberately).
- `tr/GC//` in scalar context as a **counting** operator (three of them),
  and the derived ambiguity count by subtraction.
- `while ($s =~ /(N+)/g)` -- a `/g` match loop with `pos()` state used to
  find the longest run.
- N50 computation: descending numeric sort, accumulate, `last` on the
  half-total condition -- and the "exclude empties" policy applied via
  `grep { $seq{$_}{len} }` (truthiness of 0).
- Mixed-type table cell: GC% is a formatted number or the string `'-'`
  printed through `%6s`.
- `die` on duplicate ids and data-before-header (defensive parsing).

## Conversion challenges
- `tr///` as a counter is the signature Perl-ism here; Go needs
  `strings.Count` per character or a byte loop -- a converter emitting a
  regex would be both slow and wrong for counting.
- The `/g` scan loop carries implicit `pos()` state; Go needs
  `FindAllStringIndex` or an explicit scan -- and the "longest run"
  reduction on top.
- `grep { $seq{$_}{len} }` uses 0-is-false to drop empty contigs: in Go
  the filter condition must become `len > 0` explicitly; a truthiness
  mistranslation silently changes N50.
- The per-sequence record accumulates derived fields (`len/gc/at/n/amb/
  gap`) onto the same hashref after parsing -- in Go, either a mutable
  struct or a second type; converters must not re-parse.
- `$lens[-1]` negative indexing for min; division `$total / @lens` where
  an array in numeric context is its length.
- GC denominator excludes N (`$r->{len} - $r->{n}`) with a guard against
  zero -- an easy silent-divide-by-zero in a naive port.
