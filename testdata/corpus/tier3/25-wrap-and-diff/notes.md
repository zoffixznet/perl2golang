# 25-wrap-and-diff

Two text algorithms in one tool: greedy word wrap with hyphenated
long-token splitting and hanging indent, and an LCS traceback diff of two
policy files with a similarity score.

## Constructs exercised
- wrap: `split ' '` word tokenization, greedy line building, a `while`
  that repeatedly carves oversized words with two `substr` calls and
  re-assigns the loop variable `$word` (loop-variable mutation!)
- hanging indent applied to continuation lines only, via
  `map { $_ ? $indent . $lines[$_] : $lines[$_] } 0 .. $#lines` --
  index-as-boolean trick (line 0 is falsy)
- self-check with `grep { length > $width }` + die (uses implicit `$_`
  for `length`)
- coderef-held helper: `my $slurp = sub {...}; $slurp->($file)` returning
  a list that's assigned to arrays
- full 2-D LCS DP table as array-of-arrayrefs (`$lcs[$i][$j]`,
  autovivified rows), boundary initialisation loops over `0 .. @a`
  (note: `@a` in numeric range context = its length)
- traceback loop with `--$i` / `--$j` INSIDE index expressions
  (`$a[ --$i ]`) and `unshift` building the edit script front-to-back
- diff output `+/-/space` lines; counts derived while printing
- `$lcs[-1][-1]` negative indexing into the DP table
- Dice-style similarity `2*LCS/(|a|+|b|)` printed with `%%` literal

## Conversion challenges
- `( my @script, $i, $j ) = ();` style list declarations and arrays used
  in numeric context (`0 .. @a`, `@a + @b`) -- silent length coercions
  everywhere
- pre-decrement inside an index (`$a[--$i]`) is an evaluation-order trap
  when mechanically rewritten into Go (index must be computed before use)
- ragged 2-D slices in Go need explicit allocation per row; Perl
  autovivifies
- the tie-break in traceback (`>=` favoring insertion) determines WHICH
  valid diff is printed -- converter must preserve it exactly or output
  changes while remaining "a correct diff"
- unshift-heavy building -> append+reverse for O(n)

## Go teaching opportunities
- strings.Fields, byte-length vs rune-length wrapping decisions, 2-D DP
  allocation patterns, golden-output diff testing
