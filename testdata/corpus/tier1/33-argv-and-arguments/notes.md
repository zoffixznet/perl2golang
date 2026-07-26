# 33-argv-and-arguments

## What this exercises
`@ARGV`, `$#ARGV`, `scalar(@ARGV)`, `$0`, consuming arguments with `shift` off
a copy, and a hand-rolled flag scanner of the kind that appears in every small
Perl script. The command line includes a quoted argument containing a space so
the harness's argument splitting is exercised too.

The `cmd` file for this entry is: `build -v --name "my project" 42 extra`

## Perl constructs
- `@ARGV` (note: it does **not** include the program name)
- `$0`
- `shift @scan` inside a `while (@scan)` loop
- `shift(@scan) // ""` to tolerate a missing flag value
- `grep { /^[0-9]+$/ } @ARGV` -- the one regex in the tier-1 corpus, used only
  to pick a numeric argument

## Go concepts a converter must teach
- `@ARGV` is `os.Args[1:]`, **not** `os.Args`. `$0` is `os.Args[0]`. Off-by-one
  here silently shifts every argument.
- Bare `shift` inside a sub means `shift @_`, but at file scope it means
  `shift @ARGV`. A converter has to resolve that from the enclosing scope.
- `$ARGV[$i]` reads must be bounds-guarded, since Perl returns undef past the
  end while Go panics.
- The flag scanner lowers cleanly to a Go loop over a slice, but idiomatic Go
  would use `flag`. A faithful converter should keep the loop, because Perl's
  ad-hoc parsers usually have behaviour (`--name` at the end consuming nothing)
  that `flag` would reject with an error.
- `$0` in Perl is the path as invoked and is writable (assigning to it changes
  the process title); `os.Args[0]` is read-only.
