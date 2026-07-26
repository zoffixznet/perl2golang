# 23 - a STDIN line filter

## What this exercises
The grep/awk shape: read STDIN, apply a pattern that came from the command
line, print the surviving lines with their original numbers, then summarise.

**cmd:** `ERROR` &nbsp;&nbsp; **stdin:** a 7-line log excerpt

## Perl constructs
- `my $pattern = shift @ARGV;` - `shift` with an explicit array
- `(shift(@ARGV) || '') eq '-v'` - defensive shift of a possibly-absent arg
- **`qr/$pattern/` built from user input at runtime**, wrapped in `eval` so a
  malformed pattern is caught rather than fatal
- `while (my $line = <STDIN>)` and `length $line` before `chomp`
- `$line =~ $re` - matching against a precompiled pattern held in a variable
- `next if $invert ? $hit : !$hit;` - a ternary inside a loop guard
- `push @matched, [ $seen, $line ];` an array of arrayrefs
- `sort { $a <=> $b } map { length ... }` for min/max
- `my @f = split ' ', $rec->[1]; $fieldcount{ scalar @f }++;` - note the
  deliberate use of a named array; `scalar(() = split ...)` does **not** count
  fields (split short-circuits in scalar context), which is a genuine Perl trap
- `printf ... for sort { $a <=> $b } keys %fieldcount;`

## Go concepts a converter must teach
- STDIN is `bufio.NewScanner(os.Stdin)`; the byte count Perl gets from
  `length $line` (before chomp) needs `len(line)+1` in Go, because the scanner
  has already dropped the newline. Off-by-one per line.
- **Runtime-compiled patterns** are `regexp.Compile` (not `MustCompile`), and
  the `eval { qr// }` guard becomes ordinary error handling.
- Perl regex syntax is not RE2 syntax, so a user-supplied pattern may compile
  in Perl and fail in Go, or vice versa. A converter cannot fix this - it
  should surface it.
- `shift @ARGV` is `os.Args[1:]` consumed by index; there is no mutation-based
  shift idiom in Go.
- The `scalar(() = split ...)` quirk is a reminder that the converter must
  model Perl's context rules rather than pattern-matching on syntax.
