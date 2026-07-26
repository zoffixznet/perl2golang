# 08-tie: tied variables — reads and writes run methods

Group: **A — genuinely impossible without an interpreter**

## Construct
`tie my $x, 'UpperScalar'` (line 14) attaches a class to a scalar. From then on,
`$x = "quiet"` calls `STORE`, and EVERY read — including string interpolation in
`"value: $x\n"` — calls `FETCH`. The observed output proves the ordering: the
`(FETCH ran)` line prints BEFORE the `value:` line, because FETCH fires while
the argument string is being built.

## Why it resists conversion to Go
After `tie`, the plain syntax `$x` no longer means "read a variable"; every
mention of the variable anywhere in the program, including inside string
interpolation and in code that has no idea the variable is tied, is a method
call. Converting faithfully requires rewriting every access to every possibly-
tied variable into a call through an accessor — and whether a variable is tied
can be decided at runtime (`tie` inside an `if`). Perl's own semantics make this
a whole-program, dynamic property.

## What the converter should do
- Category: **refuse-statement** for the `tie` statement, which in practice
  poisons the variable: every subsequent use of `$x` must ALSO be flagged
  (secondary notes), because converting those uses as plain reads is silently
  wrong (they would not print `(FETCH ran)`, and would not upcase).
- The honest minimum: replace `tie` with a panicking stub AND refuse every later
  use of the tied variable in the file. Merely stubbing the `tie` line while
  converting `print "value: $x\n"` as a plain read is the dishonest outcome this
  entry exists to catch.
- A full shim (an interface with Fetch/Store and rewriting all accesses) is
  acceptable only if the converter proves it rewrote EVERY access; then output
  must match `expected_stdout`.

## Ideal diagnostic (word for word)
> input.pl:14: error P2G-E106: 'tie' attaches class UpperScalar to '$x': every
> later read of $x calls UpperScalar::FETCH and every write calls STORE. Go
> variables cannot intercept access. The tie and all 3 subsequent uses of $x
> (lines 16, 17, 20) were replaced with panicking stubs. Rewrite using an
> explicit accessor object.

## What a human should do instead
Make the interception explicit: a small struct with `Get()`/`Set()` methods (Go)
or a plain object with methods (Perl). Explicit calls convert; invisible ones do
not.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0). Load-bearing details: `(STORE ran)` once,
`(FETCH ran)` twice (once per read), each FETCH line printed BEFORE the line
that consumes the value, and the values upcased to `QUIET`.
