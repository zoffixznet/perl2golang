# 07 - hash of arrays (grouping)

## What this exercises
Grouping records into buckets: the hash-of-arrays shape, built by pushing onto
an arrayref that does not exist yet, then inverted into a second index.

## Perl constructs
- `push @{ $by_level{$level} }, $msg;` - autovivifies the arrayref on first use
- `push @order, $level unless exists $by_level{$level};` remembering
  first-seen order separately, because hashes have none
- a rank table (`%rank`) used as the sort key: `sort { $rank{$a} <=> $rank{$b} }`
- `scalar @$msgs` and `@{ $by_level{$_} }` derefs in different positions
- nested iteration to invert the structure into `%keyword`
- `my ($word) = $msg =~ /^(\w+)/;` list-context match to capture
- `$errors[-1]` negative indexing
- `printf "  - %s\n", $_ for @$msgs;` statement-modifier loop with printf
- `next unless $line =~ /^(\w+)\s+(.*)$/;` guard plus capture in one step

## Go concepts a converter must teach
- `map[string][]string` is the direct analogue, and Go's zero value for a
  missing key is a `nil` slice which `append` accepts - so
  `push @{ $h{$k} }, $v` maps cleanly to `h[k] = append(h[k], v)`. This is one
  of the few autovivification cases Go handles natively, and worth contrasting
  with the map-of-map case where it does not.
- Insertion order must be tracked explicitly in both languages; the `@order`
  array plus `exists` guard maps to a `[]string` plus an `_, ok :=` check.
- `sort { $rank{$a} <=> $rank{$b} }` needs the rank map captured by the closure
  passed to `sort.Slice`.
- Negative indexing (`$errors[-1]`) has no Go form: `s[len(s)-1]`, with a
  length check Perl does not require.
- `my ($x) = $s =~ /re/;` - a match in list context returning captures. In Go
  this is `FindStringSubmatch` returning `[]string` (or `nil`), so the
  "no match yields empty list yields undef" path needs an explicit nil check.
