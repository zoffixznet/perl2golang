# 20-regex-code-recursion: recursive subpatterns and embedded code blocks

Group: **B - convertible only with an approximation that changes semantics**
(the recursion is; the `(?{...})` code blocks are effectively Group A)

## Construct
- `(?&grp)` named recursion (line 7): matches balanced parentheses - a
  non-regular language.
- `(?{ $count++ })` (line 13): Perl code EXECUTES as a side effect of the
  match engine's progress.
- The `(?{...})(*FAIL)` idiom (line 17): harvest every match via side effects
  while forcing the overall match to fail - the regex as a fold.

## Why naive Go conversion changes semantics
Balanced-paren matching is beyond any regular engine including RE2 - and
beyond `regexp2` for `(?&name)`-style recursion in general. The code blocks
are worse: they interleave the host language with backtracking, so their run
COUNT depends on engine internals (observed here: the code block ran exactly
once for `/a(?{...})a/` on "aaa"; the `(*FAIL)` loop summed 10+20+30=60 with
`\b` anchoring preventing partial-number backtracking). No Go regex engine
can run Go code mid-match.

## What the converter should do
- Recursion `(?&grp)`: **refuse-statement** with a targeted suggestion - a
  hand-written recursive-descent matcher or a depth-counter loop. (A converter
  MAY special-case the balanced-delimiter idiom into a generated
  counter-based matcher; if so, the report must show the generated algorithm
  and the outputs must match.)
- Code blocks `(?{...})`: **refuse-statement**, always. There is no
  approximation that runs the embedded Perl at the right moments; anything
  else changes how many times side effects fire.
- Forbidden: stripping `(?{...})` from the pattern and keeping the match
  (changes `$count`/`$sum` to 0), or converting `(*FAIL)` as a literal.

## Ideal diagnostic (word for word)
> input.pl:7: error P2G-E301: pattern uses recursive subpattern '(?&grp)'
> to match nested parentheses -- not a regular language; no Go regex engine
> can express it. Replaced with a panicking stub. Hand-write a
> depth-counting matcher for balanced delimiters.

> input.pl:13: error P2G-E302: '(?{ $count++ })' embeds Perl code that runs
> DURING regex matching; its execution count depends on engine internals.
> Replaced with a panicking stub. Move the side effect out of the pattern:
> match first, then act on the result.

## What a human should do instead
Balanced delimiters: a 10-line depth-counter scan. The `(*FAIL)` harvest:
an ordinary `while (/\b(\d+)\b/g) { $sum += $1 }` loop - which the converter
CAN translate - computing the same 60.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `(a(b)c): balanced`, `(a(b)c: NOT balanced`,
`((())): balanced`, `code block ran 1 time(s)`, `sum harvested by regex: 60`.
The last two lines pin down the side-effect counts a faithful replacement
must reproduce.
