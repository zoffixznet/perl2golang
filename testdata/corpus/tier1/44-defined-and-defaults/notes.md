# 44 - defined, and the default that keeps a false value

## What this exercises
The two questions Perl keeps apart and Go collapses: is this true, and does
this have a value at all.

`//` exists because `||` answers the wrong one. `$port || 8080` replaces a
port of 0; `$port // 8080` does not. A conversion that lowers both the same
way turns the second into the first, and a configuration file with a zero in
it starts behaving as though the key were missing.

The entry covers the three shapes the answer comes in:

- A **scalar with a value**, where the answer is always yes: `my $count = 3 *
  7` cannot be undef, so a `defined` test on it has one answer and `//` can
  never reach its right-hand side. The zero-value test that would otherwise
  stand in says a variable holding 0 is undefined, which is a wrong answer
  rather than an approximate one.
- A **hash key**, where the answer is real and Go can give it: the two-result
  form of the index expression asks exactly "is this key there", which is what
  `//` on a hash element means.
- A key that is **present and false**, which is the case that separates the
  two operators, run for `0` and for the empty string.

## Perl constructs
- `//` and `||` side by side over 0, the empty string and a set value
- `defined` on a scalar initialised with arithmetic and with an interpolation
- `exists`, `//` and `||` over a hash with a missing key, a zero value and an
  empty string

## Go concepts a converter must teach
- Go has no undefined state. A declared variable holds its type's zero value,
  and that value is indistinguishable from one stored on purpose.
- The comma-ok form of a map read is the one place the distinction survives,
  and it is a different question from testing the value.
- Where a variable genuinely can be absent, the Go answer is a pointer type,
  and `nil` then really does mean absent. That is a change to the declaration
  rather than to the test.
