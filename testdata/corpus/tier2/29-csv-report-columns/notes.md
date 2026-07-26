# 29 - CSV-ish parsing and a column report

## What this exercises
A header-driven CSV reader, two-level grouping, aligned `sprintf` output with
totals and percentages, and - at the end - a proper quoted-field parser for the
one line that plain `split` cannot handle.

## Perl constructs
- reading the header line separately and building `my %idx = map { $cols[$_] =>
  $_ } 0 .. $#cols;` - a name-to-position index
- `my @f = split /,/, $line, scalar @cols;` using the column count as the limit
- `my %row = map { $_ => $f[ $idx{$_} ] } @cols;` building a record by name
- a derived field computed at load time (`$row{total} = $row{qty} *
  $row{unit_price}`) - string-to-number coercion happening implicitly
- two-level grouping with autovivification:
  `$by_region{$region}{$status}{count}++` and `{value} +=`
- an `ALL` pseudo-key accumulated alongside the real ones
- `sort { $a->{order_id} <=> $b->{order_id} }` numeric sort on a string field
- `sort { $cust{$b} <=> $cust{$a} || $a cmp $b }` value-descending sort
- `'*' x int(20 * $v / $total + 0.5)` bar chart with explicit rounding
- `printf` with `%-6s %-10s %9.2f`, `'-' x 72` rules, and `\n\n` spacing
- a `\G`-anchored quoted-CSV parser: `/\G(?:"((?:[^"]|"")*)"|([^,]*))(?:,|$)/gc`
  with `""` unescaping and a `pos()` termination check

## Go concepts a converter must teach
- The header-index map is exactly `encoding/csv`'s job; a converter should
  recognise the idiom and consider emitting `csv.Reader` - but only if the Perl
  used a real CSV parser, because plain `split /,/` and `csv.Reader` differ on
  quoted fields. Here the script has *both*, which is the interesting case.
- **Implicit numeric coercion** (`$row{qty} * $row{unit_price}` on strings from
  `split`) needs `strconv.ParseFloat` plus error handling at load time, and a
  decision about what to do with unparseable values (Perl warns and uses 0).
- Float formatting: Perl's `%.2f` and Go's `%.2f` round the same way here, but
  Perl uses the C library and Go has its own formatter - values exactly on a
  .005 boundary can differ. Worth testing.
- Two-level grouping is the nil-inner-map problem again.
- The bar chart's `int(x + 0.5)` is round-half-up; Go's `math.Round` matches,
  but `int()` alone truncates in both languages.
- The `\G` CSV parser has no RE2 equivalent (see entry 15) - the right answer
  in Go is `encoding/csv`, which is a case where the converter should replace
  the algorithm rather than translate it.
