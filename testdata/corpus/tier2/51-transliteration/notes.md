# 51 - tr/// and its four modifiers

## What this exercises
Character-for-character replacement, which Perl gives its own operator and Go
gives nothing at all. The four modifiers are the interesting part: they change
what the operator means rather than how it is written, and no single call in
the `strings` package covers them.

## Perl constructs
- plain `tr/ACGT/TGCA/`, which is `strings.NewReplacer`
- `tr/ACGT//` counting rather than replacing, and `tr/ACGT//c` counting the
  complement
- `tr/ACGT//cd`, deleting everything outside the list
- `tr/ \t/ /s`, collapsing runs, with `\t` as an escape inside the list
- `tr/acgt/ACGT/r`, which leaves the original alone and yields the result
- `tr/A-Za-z/N-ZA-Mn-za-m/`, ranges on both sides
- `( my $copy = $original ) =~ tr/.../.../`, the copy-then-edit idiom

## Go concepts a converter must teach
- `strings.NewReplacer` is the plain case and is worth hoisting out of a loop:
  it builds its matcher once and reuses it.
- `strings.Map` with a function returning `-1` deletes a character, which is
  the `d` modifier on its own.
- Counting characters from a set has no call at all: `strings.Count` counts
  occurrences of one substring and `strings.ContainsAny` only says whether any
  appear.
- `\t` inside a character list is a tab, not the letter `t`, and a Go string
  literal spells it the same way.
- The `r` modifier is the only form that is an expression rather than a
  change, and it is the one that reads most like Go: a function of a string
  returning a string, with the original untouched.
