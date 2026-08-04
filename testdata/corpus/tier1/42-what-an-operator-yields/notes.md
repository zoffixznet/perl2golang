# 42 - what an operator yields

## What this exercises
Five constructs whose value is not what a first reading suggests, gathered
because getting any of them wrong is silent: the program runs, prints
something plausible, and is wrong.

- `&&` and `||` answer with an **operand**, not with true or false. `$a || $b
  || 'fallback'` is a defaulting chain precisely because of that, and a
  conversion that yields a bool turns "ada" into 1.
- `push` and `unshift` answer with the array's new **length**, not with what
  went in.
- `chop` removes the last character, whatever it is, and yields that
  character; `chomp` removes a trailing newline and yields how many characters
  it removed, which is 0 when there was no newline. The two are one letter
  apart and answer with different kinds of thing.
- `chomp( my $line = <...> )` is a declaration and a chomp in one, which is
  how nearly every line-reading loop is written.
- `s///` answers with the number of replacements, and `s///r` answers with the
  edited copy and leaves the original alone. The same operator, one letter
  apart, and two different kinds of value again.

## Perl constructs
- `&&` and `||` used for their value, including a three-term defaulting chain
- `my $n = push @a, ...` and `my $n = unshift @a, ...`
- `chop`, `chomp`, and `chomp` wrapping a declaration
- `s///g`, `s///` and `s///gr` read for their values, including a failed match

## Go concepts a converter must teach
- Go's `&&` and `||` are strictly boolean and yield nothing else, so a
  defaulting chain becomes a variable and an `if`, which is also where the
  short-circuit becomes visible.
- `append` answers with the new slice and `len` says how long it is: two
  separate things, where Perl's `push` conflated them.
- A Go string cannot be changed in place, so every replacement returns a new
  one and the `/r` question never arises.
- Multiple return values are how one call hands back both the edited text and
  the character it removed, which is what `chop` needed a global for.
