# 19-kmer-count

**Domain:** bioinformatics. Canonical k-mer spectrum: for every k-window
the lexically smaller of the k-mer and its reverse complement is
counted, windows containing non-ACGT are skipped, and the report shows
totals, a multiplicity histogram (count-of-counts), and the top-N
k-mers. The archetypal Perl tight loop over `substr`.

## Constructs exercised
- Sliding window via `substr $s, $i, $k` in a `0 .. $last` range loop --
  the hot path a converter's output will be judged on for performance.
- `tr/ACGT//c` (complement count) in boolean position as a "contains
  anything else?" test.
- `reverse` + `tr/ACGT/TGCA/` reverse complement; `lt` string comparison
  to pick the canonical form.
- Deferred-flush parser: sequence lines accumulate until the next `>`
  header, with the classic explicit `process($seq) if length $seq` after
  the loop for the final record (called out in a comment).
- Count-of-counts second-order histogram (`$mult{$_}++ for values
  %count`).
- `keys %count` in scalar context for the distinct count; `splice
  @ranked, $top_n` truncation; two-key sort (count desc, k-mer asc).
- Subs mutating file-scoped `my` accumulators (`%count`, `$bases`,
  `$skipped`) rather than taking/returning state.

## Conversion challenges
- `substr` in a loop is O(k) string copying in both languages, but the
  idiomatic Go port should slice (`s[i:i+k]`) with zero copies -- a
  measurable performance teaching point; likewise the `tr///c` test
  should become a byte scan, not a regex.
- The file-scoped-accumulator style (subs closing over `my` variables at
  file scope) translates to either package-level vars, closures, or a
  counter struct with methods -- an architectural choice a converter
  must make deliberately.
- `keys %count` scalar context vs list context in the same program.
- `$k` arrives as a string from `@ARGV`, is regex-validated, then used
  in arithmetic and range bounds; `||=` defaulting would misbehave if a
  caller passed `0` -- worth preserving faithfully (it is `||=`, not
  `//=`, and that difference is observable).
- Hash iteration over `%count`/`%mult` is always sorted before output;
  the histogram's numeric sort on string keys is another string/number
  duality point.
