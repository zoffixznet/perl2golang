# 40 - copying, context, and the value of a step

## What this exercises
Four places where a mechanical translation quietly changes the answer rather
than failing. Each of them compiles either way, which is what makes them worth
an entry of their own.

## Perl constructs
- `my @copy = @original` and `my %adjusted = %prices`, both of which copy
- `reverse` read as a list, as one value, and on a single string in each of
  those two ways
- `$i--` used for its value in a `while` modifier, and `$n++` inside a `push`
- `--$m` in the same statement that reads `$m`
- `delete $stock{gadget}` used for the value it removed, and `delete` of a key
  that is not there
- `%pair` where a list was wanted, and `( %pair, c => 3 )` as a merged hash

## Go concepts a converter must teach
- Assigning one slice to another copies the header alone: both names then
  share a backing array and a write through either is visible through the
  other. `slices.Clone` is the difference, and a map is the same story with
  `maps.Clone`. This is the single most surprising thing about slices coming
  from Perl.
- `reverse` is two operations wearing one name. Go has `slices.Reverse`, which
  works in place, and nothing at all for reversing text, because reversing
  text is only meaningful character by character.
- `++` and `--` are statements in Go, not expressions. A step used for its
  value has to be written out, and the old value has to be named first,
  because that is what the post-step form yields.
- `delete` on a Go map yields nothing, so the value has to be read out before
  the key goes.
- A hash written inside another hash contributes its pairs. Go has no splicing
  into a composite literal: the base is cloned and the extras are set
  afterwards, which also makes it plain that the copy is shallow.
