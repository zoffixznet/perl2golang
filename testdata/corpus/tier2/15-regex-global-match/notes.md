# 15 - //g in both contexts, pos() and \G

## What this exercises
The `/g` flag, which behaves completely differently in list and scalar
context, plus the match-position machinery (`pos`, `\G`, `/c`) that scalar
`/g` depends on.

## Perl constructs
- `my @keys = $text =~ /(\w+)=/g;` - list context, all matches at once
- multiple captures per match flattening into one list, then `my %map = @pairs`
- `while ($text =~ /(\w+)=(\w+)/g)` - scalar context, one match per iteration,
  with `pos($text)` advancing
- `pos()` becoming `undef` when the loop ends
- `my $n = () = $str =~ /re/g;` the count-of idiom
- `pos($text) = index(...)` - **`pos` is an lvalue**, used to restart matching
  part-way through
- `\G` anchoring to the current position, with `/gc` so a failed match does not
  reset `pos` - a hand-written tokeniser over an arithmetic expression
- per-variable match position: three separate strings each keeping their own
  `pos` inside a loop
- explicit demonstration that a failed `/g` resets `pos` while a failed `/gc`
  does not

## Go concepts a converter must teach
- Go has `FindAllString`, `FindAllStringSubmatch` and `FindAllStringIndex`,
  which cover the list-context case directly (`-1` for "all").
- **There is no per-variable match position in Go.** Scalar `while (/.../g)`
  must become an index variable plus `re.FindStringSubmatchIndex(s[pos:])` with
  manual offset arithmetic, or a range over `FindAllStringSubmatchIndex`.
- The flattening of multi-capture `/g` into one list (then `%h = @pairs`) needs
  an explicit nested loop in Go.
- `\G` has no Go equivalent. Anchored tokenising uses `re.Prefix`-style
  patterns anchored with `\A` applied to a shrinking slice, or a hand-written
  scanner. A converter meeting `\G` should probably emit a `bufio.Scanner`-like
  lexer rather than regexps.
- `/c` semantics (keep `pos` on failure) only matter for the tokeniser loop; in
  the Go rewrite it becomes "do not advance the cursor", which is clearer.
- `pos()` as an lvalue is a Perl-ism with no analogue - it is just an integer
  cursor in the Go version.
