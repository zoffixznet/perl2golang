# 28 - word frequency with deterministic ordering

## What this exercises
The classic word-count script, written so that the output is fully
deterministic: ties are broken alphabetically, and every hash walk is sorted.
Also covers stopword filtering, histograms and order-preserving dedupe.

**stdin:** five lines of prose

## Perl constructs
- `my %stop = map { $_ => 1 } qw(...);` set construction from a `qw` list
- `while (my $line = <STDIN>)` with `lc` normalisation
- `for my $w ($line =~ /([a-z][a-z'\-]*)/g)` - **a `//g` match in list context
  driving a foreach loop**
- `$freq{$w}++` and `$first_line{$w} = $lines unless exists $first_line{$w};`
- **`sort { $freq{$b} <=> $freq{$a} || $a cmp $b } keys %freq`** - count
  descending, then alphabetical, so ties are stable
- `grep { !$stop{$_} } @ranked` filtering
- a bounded array slice `@content[0 .. ($#content < 9 ? $#content : 9)]` -
  Perl's manual clamp, since a slice past the end yields `undef`s
- `'#' x $freq{$w}` histogram bars
- `push @{ $by_len{ length $_ } }, $_` autovivified hash of arrays
- `my %seen; grep { !$seen{$_}++ }` dedupe preserving a sort order
- `$#array` last-index, `scalar @array`
- percentage formatting with `%.1f%%` (the doubled `%%`)

## Go concepts a converter must teach
- `map[string]int` plus a `[]string` of keys sorted with `sort.Slice` - the
  two-key comparator (count desc, then name asc) is essential to reproduce the
  byte-exact output.
- **Both languages randomise map iteration order**, so this entry is a good
  test that the converter never emits a bare `for k := range m` where the Perl
  had `sort keys`.
- `//g` in list context feeding a loop is `re.FindAllString(s, -1)`.
- The `[0 .. min(n, $#a)]` clamp maps to Go's slice bounds - but Go panics on
  an out-of-range high bound where Perl silently pads with `undef`, so the
  clamp must be preserved, not optimised away.
- `'#' x n` is `strings.Repeat`.
- `$freq{$w}++` on a missing key works identically in Go (zero value), which is
  the one autovivification case that needs no special handling.
- `lc` is `strings.ToLower`; character classes in the tokeniser must be
  translated carefully because Go's `\w` and Perl's differ on Unicode.
