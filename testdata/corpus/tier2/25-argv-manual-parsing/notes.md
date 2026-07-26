# 25 - hand-rolled @ARGV parsing

## What this exercises
Option parsing written by hand, the way small scripts do it before reaching
for a module: long options with and without values, clustered short flags,
`--` to end option processing, and positional arguments.

**cmd:** `--sort=count -rv --limit=3 -- files/data.txt`

## Perl constructs
- `while (@ARGV) { my $arg = shift @ARGV; ... }` - destructive consumption
- `--` handling: `push @positional, @ARGV; @ARGV = (); last;`
- `--name=value` parsed with `/^--(\w[\w-]*)=(.*)$/`
- `--name` as a boolean, validated against `exists $opt{$name}`
- clustered short flags: `-rv` split with `split //` and dispatched per letter
- `$opt{verbose}++` so `-v -v` or `-vv` increments
- a defaults hash defining the entire option surface, doubling as the validator
- a `usage` sub returning a string built with `join "\n", ...`
- **a comparator chosen at runtime and stored in a code ref**, then invoked
  from inside `sort { $cmp->($a, $b) }`
- `index($_->{name}, $opt{prefix}) == 0` prefix filtering
- array slice with a computed range: `@records[0 .. $opt{limit} - 1]`
- `reverse @records`

## Go concepts a converter must teach
- Go's `flag` package parses differently (no clustering, no `--name=value` for
  bools in the same way, stops at the first non-flag). A faithful conversion of
  a hand-rolled parser usually means *keeping it hand-rolled* in Go rather than
  mapping to `flag`, or the observable behaviour changes.
- `shift @ARGV` in a loop is index-based iteration over `os.Args[1:]`.
- The defaults hash is an options struct; the `exists $opt{$name}` validation
  becomes a `map[string]bool` of known names or a switch.
- `$opt{verbose}++` for repeated flags has no `flag`-package equivalent; it
  needs a custom `flag.Value` or manual parsing.
- **A code ref chosen at runtime and used as a sort comparator** is
  `func(a, b Record) bool` stored in a variable and handed to `sort.Slice` -
  clean in Go, but note Perl's `$a`/`$b` are globals while Go's are parameters.
- Array slices with computed bounds need Go's half-open ranges plus a length
  clamp; Perl silently produces `undef` entries past the end, Go panics.
- `index(...) == 0` is `strings.HasPrefix`.
