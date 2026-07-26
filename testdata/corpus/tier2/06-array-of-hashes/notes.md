# 06 - array of hashes, sorted report

## What this exercises
The most common Perl data shape: a list of records, each a hashref, read from
a delimited file, sorted on several keys and printed as a fixed-width table.

## Perl constructs
- three-argument `open my $fh, '<', $file or die "...: $!\n";`
- `while (my $line = <$fh>)` with `chomp`
- `split /\|/, $line` (the pipe needs escaping in the pattern)
- `push @servers, { name => ..., ... };` building an array of anonymous hashes
- multi-key sort with `||` chaining `cmp` and `<=>`, including one descending
  key (`$b->{cpu} <=> $a->{cpu}`)
- `printf` with `%-8s`, `%4d`, `%8d` and `'-' x 46`
- accumulation loop over the records
- `grep { $_->{cpu} >= 8 } @servers` and `map { $_->{name} } ...`
- `my ($first_down) = grep {...}` - list assignment in scalar-ish position to
  take the first match
- `close $fh or die`
- integer/float mix: `$total_mem / 1024` printed with `%.1f`

## Go concepts a converter must teach
- Records become a `[]Record` of a real struct; the converter has to infer
  field names and types from the hash keys and how they are used (`cpu` is used
  with `<=>` and `%d`, so it is numeric even though `split` produced a string).
- Perl strings are silently numeric on demand. `split` yields strings;
  `$s->{cpu} >= 8` coerces. Go needs `strconv.Atoi` at parse time plus error
  handling that Perl does not have.
- Multi-key sort with `||` becomes `sort.Slice` with a chain of
  `if a != b { return ... }` comparisons; `cmp` is `strings.Compare`, `<=>` is
  numeric comparison, and descending means swapping the operands.
- `my ($first) = grep {...}` is "first match or nil" - a loop with `break`, not
  a full filter.
- `%-8s` etc. map onto `fmt.Printf` almost verbatim, which is one of the few
  places the translation is trivial.
- `close $fh or die` - Go's `defer f.Close()` discards the error; a faithful
  conversion needs a named return or an explicit check.
