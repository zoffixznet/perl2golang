# 19 - line-by-line reading

## What this exercises
The bread-and-butter file loop: three-argument `open` with a lexical
filehandle, `while (my $line = <$fh>)`, `chomp`, and the `$.` line counter.

## Perl constructs
- `open my $fh, '<', $path or die "$0: cannot open $path: $!\n";`
- `$!` (errno as a string) and `$0` (the program name) in the error message
- `while (my $line = <$fh>)` - the readline-returns-undef-at-EOF loop
- `chomp $line;` removing the record separator
- skip patterns: `next if $line =~ /^\s*#/;` and `/^\s*$/`
- `my ($ip, @names) = split ' ', $line;` scalar-plus-slurpy destructuring
- storing an arrayref inside a hashref record
- `my @all = <$fh>;` - **readline in list context slurps every line**
- `chomp @all;` chomping a whole array in one call
- `(sort { $b <=> $a } map { length } @all)[0]` - a list slice on a sorted list
- `while (<$fh>)` with the implicit `$_`, and `$.` for the current line number
- `my $removed = chomp $sample;` - chomp's return value is the count of
  characters removed
- `close $fh or die` on every handle

## Go concepts a converter must teach
- `bufio.Scanner` is the natural target, but it has a 64KB default line limit
  that Perl does not; a converter should either set `scanner.Buffer` or use
  `bufio.Reader.ReadString('\n')`.
- **`<$fh>` keeps the newline, `chomp` removes it. `Scanner.Text()` has already
  removed it.** A converter must not emit a redundant TrimRight, and must
  handle the `chomp`-return-value case (count of removed chars) separately.
- List-context `my @all = <$fh>` is "read all lines into a slice" - a different
  function entirely from the loop form, decided by context.
- `$.` is an implicit counter the converter must materialise as a local `int`.
- `$!` is `err.Error()`; `$0` is `os.Args[0]`.
- Perl's `open ... or die` is a two-value pattern: Go's `f, err := os.Open(p)`
  plus `if err != nil`. Mechanical, but every single call site needs it.
- `close $fh or die` matters for write handles (buffered data); for read
  handles Go's `defer f.Close()` is enough.
- `sort { $b <=> $a } map { length }` on a slice needs an intermediate `[]int`.
