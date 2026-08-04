# 79 - a hand-written lexer, where the position lives on the string

## What this exercises
Three things that only work together.

`\G` anchors a match where the previous one on the same variable stopped. Go
has no such anchor and no position to anchor to, but a scanning loop already
carries the position in a variable and hands the engine the text from there
onwards; against that text `^` means exactly what `\G` meant. That is the one
place the anchor is expressible, and it is the place it is used.

`/c` says a failed match leaves the position alone, which is what lets the
next alternative try from the same place. Without it the first pattern that
fails would rewind the walk to the start.

And the `if`/`elsif` chain has to be lazy, which sounds obvious and is the
thing most easily lost in translation: each alternative *consumes input* when
it matches, so evaluating every test before choosing a branch tokenises the
string four times over and returns nonsense. The last section makes the same
point without regexes, with a branch whose test drains a queue.

## Perl constructs
- `pos($s) = 0` and `pos($s)` as a loop condition
- `\G` at the start of a pattern with `/gc`
- an `if`/`elsif` chain of anchored matches, each with captures
- an `elsif` whose test has a side effect

## Go concepts a converter must teach
- A Go string carries nothing but its bytes. Everything Perl hangs off a
  scalar, including the match position, becomes a variable beside it.
- `^` against a slice of the string from the cursor is the scan anchor, and
  seeing that identity is what makes the whole construct translatable.
- Go's `else if` is lazy in exactly the way the original relied on, but the
  setup a condition needs has to be written inside the `else` to stay that
  way. That is the difference between `if x := f(); x` and computing every
  branch's value in front of the chain.
