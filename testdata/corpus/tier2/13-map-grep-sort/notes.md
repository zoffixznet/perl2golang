# 13 - map/grep/sort with real blocks

## What this exercises
List pipelines where the blocks contain more than one statement: `map`
returning hashrefs, `grep` in list and scalar context, multi-key sorts, a
Schwartzian transform, and a named sort comparator.

## Perl constructs
- `map { my @f = split /,/; ($f[0], $f[1]) }` - a map block returning *two*
  elements per input, so the output list is longer than the input
- `map { ...; { name => ..., ... }; }` returning an anonymous hashref (the
  braces-as-hashref-vs-block ambiguity is why the block ends with `;`)
- `grep { my $p = $_; ...; $p->{year} >= 1900 && ... }` multi-statement grep
- `my $professors = grep {...} @people;` - **grep in scalar context returns a
  count**, not a list
- multi-key sort: `sort { $b->{year} <=> $a->{year} || $a->{last} cmp $b->{last} }`
- Schwartzian transform: `map { $_->[1] } sort {...} map { [key, val] } @list`
- `sort by_role_then_year @people` - a named comparator using the package
  globals `$a` and `$b`
- `push @{ $roles{...} }, ...` inside a statement-modifier `for`
- `reverse sort { lc($a) cmp lc($b) } map { uc ... }` - a three-stage pipeline
- `int($year / 100) + 1` truncating division

## Go concepts a converter must teach
- `map` is not Go's `map`. It is a transform: `for _, x := range in { out =
  append(out, f(x)) }`. When the block returns several elements, the append is
  variadic - the converter must detect the arity of the block's result.
- `grep` in scalar context returning a count is a context trap: the same source
  construct becomes either a filtered slice or an `int`. Resolving that
  statically is required.
- Sort: `sort.SliceStable` with a comparator; `||` chains become sequential
  `if` returns. Perl's `sort` is stable in practice for recent perls but not
  guaranteed - the converter should pick `SliceStable` to match observed output.
- `$a`/`$b` are package globals in Perl, so a named comparator has no
  parameters. Go needs real parameters.
- The Schwartzian transform exists to avoid recomputing the key; in Go the
  natural equivalent is `sort.Slice` with a precomputed key slice, or
  `slices.SortFunc` - a converter should recognise the idiom rather than
  translating the three stages literally.
- `lc`/`uc` are `strings.ToLower`/`ToUpper`; `length` on a string is bytes in
  Go but characters in Perl once `use utf8` is in play.
