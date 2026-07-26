# 26 - Getopt::Long

## What this exercises
The standard option parser with a realistic mix of option types: a negatable
boolean, an integer, a string, a repeatable list, a `key=value` hash option, an
incrementable counter, aliases and bundling.

**cmd:** `--no-header -t 40 --format=csv -i cpu -i mem --rename web1=frontend1 --rename db1=primary -vv files/metrics.tsv`

## Perl constructs
- `use Getopt::Long qw(GetOptionsFromArray);`
- `Getopt::Long::Configure('bundling', 'no_ignore_case');`
- option specifications, one per type:
  - `'header!'` - negatable, enables `--header` **and** `--no-header`
  - `'threshold|t=i'` - integer with a short alias
  - `'format|f=s'` - string
  - `'include|i=s@'` - repeatable, accumulating into an arrayref
  - `'rename=s%'` - `key=value` pairs accumulating into a hashref
  - `'verbose|v+'` - incrementing counter, so `-vv` gives 2
  - `'quiet|q'` - plain boolean
- destinations given as `\$scalar`, a pre-existing arrayref and a pre-existing
  hashref
- `GetOptions` returning false on a parse error
- `@ARGV` left holding only the non-option arguments
- `GetOptionsFromArray(\@other, ...)` parsing a second list without touching
  `@ARGV`, and leaving its leftovers in that array
- `my %want = map { $_ => 1 } @{ $opt{include} };` building a set
- `next if %want && !$want{$metric};` - a hash in boolean context is
  true when non-empty
- `split /\t/` on a TSV file

## Go concepts a converter must teach
- Go's `flag` package covers `=s`, `=i` and plain booleans, but **not**:
  negatable `--no-x`, repeatable `=s@`, `key=value` `=s%`, `+` counters, or
  bundling. Each needs a custom `flag.Value` implementation, or a third-party
  library (`spf13/pflag`, `urfave/cli`) - a converter must choose a policy and
  apply it consistently.
- Go's `flag` stops parsing at the first non-flag argument; Getopt::Long by
  default permutes, so options *after* a positional argument still parse. This
  changes behaviour on real command lines.
- `flag.Args()` is the `@ARGV` remainder.
- `GetOptionsFromArray` is `flag.NewFlagSet(...).Parse(slice)` - the cleanest
  correspondence in this entry.
- A hash in boolean context (`%want && ...`) is `len(want) > 0`.
- `map { $_ => 1 }` is a `map[string]struct{}` set built in a loop.
- Failure handling: `GetOptions` returns false and warns; Go's `flag` calls
  `os.Exit(2)` by default unless `ContinueOnError` is set.
