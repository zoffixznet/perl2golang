# 43 - case folding in a replacement, and tr's squeeze, neither of which converts yet

## What this exercises
The neighbour of entry 42: two things a replacement can ask for that Go's
regexp has no template for.

- `\u`, `\l`, `\U`, `\L` inside a replacement fold the case of what follows.
  Go's `ReplaceAllString` template understands `$1` and `${name}` and nothing
  else, so any transformation of a capture has to happen in a function.
  `ReplaceAllStringFunc` is the shape, with the wrinkle that it is handed the
  whole match rather than the groups.
- `tr/a-c//s` collapses runs of the listed characters without replacing them,
  and `tr/ \t/ /s` collapses whitespace runs into single spaces. The `s`
  modifier changes what the operator does rather than how it is written, and
  no call in the strings package covers it.

The counting form of `tr` is in here as the contrast, because that one does
convert: `$s =~ tr/a//` counts without changing anything.

## What goes wrong today
All three case-folding lines come out with the original case, and both squeeze
lines come out unsqueezed. Nothing is reported about either, which is the part
that makes them worth recording: a silent wrong answer is the outcome the
whole project is meant to avoid.

## Go concepts a converter must teach
- A replacement template is data, not code. Anything that transforms a capture
  needs `ReplaceAllStringFunc` and a function, which is more code and entirely
  explicit.
- `strings.ToUpper` and friends work on whole strings, and folding just the
  first character is `strings.ToUpper(s[:1]) + s[1:]` with a rune-safe caveat.
- Collapsing runs is a loop over the string comparing each character to the
  last one kept, which is three lines and no package call.
