# 08-csv-report

Hand-written character-level CSV parser (no Text::CSV) handling quoted
fields, `""` escapes, embedded commas AND embedded newlines, plus a
grouped report and a re-emit round trip.

## Constructs exercised
- explicit state-machine parse loop over `substr($text, $i, 1)` with an
  `$in_quotes` flag and one-character lookahead for doubled quotes
- `$field_row[-1] .= $c` -- appending to the last element via negative index
- building records with a hash slice from parallel lists:
  `@rec{@$header} = @$row`
- copy-then-modify idiom `( my $flat = $r->{product} ) =~ s/\n/ /g;`
- `push @{ $cat{...} }, $r` autovivified hash-of-arrayrefs grouping
- nested sorts (category asc, id numeric asc, price desc with list slice)
- minimal-quoting CSV emitter with `s/"/""/g` and `qq{"$q"}`
- deep list equality via `join` with `\x1f`/`\x1e` separator sentinels
- die-based validation (field-count mismatch, stray quote, unterminated
  field), fixture whose 4th record spans two physical lines

## Conversion challenges
- the embedded-newline case means the unit of parsing is the whole file,
  not lines -- converters that translate `while (<$fh>)` per-line habits
  break here
- `$field_row[-1] .= $c` negative-index append: Go needs
  `row[len(row)-1] += string(c)` (and byte-vs-rune decisions)
- hash slice `@rec{@$header} = @$row` -> loop building a map
- Perl strings are mutable accumulators; Go should teach strings.Builder or
  []byte to avoid O(n^2) concatenation
- `%8.2f` on the string field `"449.99"` (numeric coercion of a string)
- the `(sort ...)[0]` list-slice-of-sort idiom

## Go teaching opportunities
- a real tokenizer loop with explicit state; encoding/csv comparison point
  (the notes explain why the hand parser exists); table-driven grouping
