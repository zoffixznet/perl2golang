# 80 - the scan anchor where it cannot be an anchor, which does not convert

## What this exercises
The neighbour of entry 79. There `\G` was at the start of a pattern being
walked with `/g`, which is the one shape where it has a Go equivalent: the
walk hands the engine the text from the cursor, and `^` against that text is
the same anchor.

Neither shape here is that.

`(?:\G|,)\s*(\w+)` puts the anchor inside an alternation, which is the
standard way to write "at the cursor, or after a comma". There is no way to
say "at the cursor" in the middle of a Go pattern at all: the anchor would
have to refer to a position the engine does not know about.

`$stamp =~ /\G(\d{4})/` has no walk at all. In Perl it means "at pos, which is
0 because nothing has matched yet", so it behaves as `^` here and would not if
anything had matched first. Reading it as `^` unconditionally would be right
today and wrong after an edit somewhere else in the file.

The last section is the fact the whole idiom rests on, and is worth recording
whether or not it converts: `/c` leaves the position alone when a match fails
and plain `/g` resets it to undef.

## Perl constructs
- `\G` inside an alternation
- `\G` in a pattern used without `/g`
- `pos()` after a failed match, with and without `/c`

## What goes wrong today
Both `\G` patterns are refused, by name and with the reason. The refusals are
honest and the program still runs; the sections that depend on them print
nothing useful, which is the correct outcome for a construct with no
translation rather than a plausible wrong answer.

## Go concepts a converter must teach
- An anchor is a claim about a position, and Go's regexp knows only two
  positions: the start and the end of the text it was handed. Everything else
  has to be arranged by choosing what text to hand it.
- The rewrite that does work is to make the cursor explicit: keep an index,
  slice from it, and let each pattern anchor with `^`. That is more code and
  it is code whose behaviour is visible, which is usually the better trade.
