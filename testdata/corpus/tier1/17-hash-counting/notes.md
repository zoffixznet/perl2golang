# 17-hash-counting

## What this exercises
The canonical Perl word-frequency program, reading STDIN. Covers
`while (my $line = <STDIN>)`, `chomp`, `split /\s+/`, `lc`, autovivifying
counter increment `$count{$word}++` on a key that does not yet exist, a
multi-key sort (frequency descending then name ascending), and a
conditional-push filter loop.

Note `split /\s+/` on a line with leading whitespace produces an initial empty
field -- the `next if $word eq ''` guard is there for exactly that.

## Perl constructs
- `<STDIN>` in a while condition (the implicit `defined()` wrap)
- `chomp`
- `split /\s+/, lc $line`
- `$h{$k}++` on a missing key: starts from undef, treated as 0, no warning
- `sort { $count{$b} <=> $count{$a} or $a cmp $b } keys %count`
- `@arr ? ... : ...` array in boolean context

## Go concepts a converter must teach
- `while (my $line = <STDIN>)` is `bufio.Scanner` (or `bufio.Reader.ReadString`
  if the final unterminated line matters). Note Perl's `<STDIN>` **keeps** the
  newline and `chomp` removes it, whereas `Scanner.Text()` has already stripped
  it -- so the converter should drop the `chomp` when it lowers to a Scanner,
  and must not drop it when the line came from `ReadString('\n')`.
- The loop condition is `defined($line)`, not truthiness. A line consisting of
  just `"0"` would end the loop if lowered as truthiness -- but Perl special
  cases `while (<FH>)` to add the `defined`. The converter must reproduce that
  special case.
- `$count{$word}++` on a missing key relies on undef numifying to 0 silently.
  Go's `m[k]++` on a missing key does exactly the same thing for an int-valued
  map, so this one lowers cleanly.
- `split /\s+/` is `regexp.MustCompile(`\s+`).Split(s, -1)`, **not**
  `strings.Fields` -- Fields drops the leading empty field that Perl keeps.
  (Perl's `split ' '` -- a literal single-space string -- is the one that
  behaves like `strings.Fields`.)
- Multi-key sort becomes a `sort.SliceStable` with a chained comparison; Perl's
  `or` between two `<=>` results is the same short-circuit pattern.
