# 11-tsv-join

**Domain:** text/data munging. A relational left/inner join over two TSV
files keyed by a named column: right side indexed as key -> list of rows,
duplicate right keys fan out like SQL, left rows with empty keys never
match, and `--empty` supplies the null marker for unmatched left joins.
A trailing `#` comment line carries join statistics.

## Constructs exercised
- Hash-of-arrays index built with `push @{ $right{...} }, $_ for @$rrows`
  -- autovivification inside a postfix loop.
- Array slices of arrayrefs (`@{$rhdr}[@rkeep]`, `@{$rrow}[@rkeep]`) where
  `@rkeep` is a `grep` over an index range excluding the join key.
- `split /\t/, $line, -1` to preserve trailing empty columns; ragged-row
  padding with `push @f, '' while @f < @hdr`.
- Multiple return values as `($hdr_ref, $rows_ref)` pairs.
- `map { $opt{empty} } @rkeep` to synthesize the null columns.
- Empty-string join key treated as "never matches" (`defined $k and $k ne
  ''`) -- the E007 row exercises it.

## Conversion challenges
- Index slices (`@{$rhdr}[@rkeep]`) have no direct Go syntax; the
  converter must emit an explicit gather loop or helper -- a good test of
  whether it recognises the slice-of-indices idiom rather than
  translating token-by-token.
- The row representation is `[]string` (positional), while headers map
  names to indices -- Go code that jumps to `map[string]string` rows
  changes memory shape and column order semantics.
- `read_tsv` returning two refs: multiple return values are natural in
  Go, but the `$#$rhdr` arithmetic (`0 .. $#$rhdr`) needs care with
  `len()-1`.
- Fan-out on duplicate right keys plus the stats line ordering means the
  right-index must preserve *insertion order per key* (slices do), and
  `scalar(keys %right)` maps to `len(map)` -- one of the few places an
  unsorted map is safe, worth teaching why (only the count reaches
  output).
- `--empty -` on the command line: a Getopt value that looks like a flag;
  Go's `flag` package would eat `-` differently -- argument-parsing
  fidelity matters.
