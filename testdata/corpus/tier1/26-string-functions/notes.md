# 26-string-functions

## What this exercises
`length`, `uc`, `lc`, `ucfirst`, `lcfirst`, `index` and `rindex` (with and
without a position argument, and the -1 miss result), `index($s, "")` returning
0, `ord`/`chr` (including `ord("")` returning 0), `x` repetition, scalar
`reverse`, `split //` into characters, manual left/right padding, and
case-insensitive comparison via `lc`.

## Perl constructs
- `index STR, SUBSTR [, POSITION]` -- POSITION is a *minimum* start
- `rindex STR, SUBSTR [, POSITION]` -- POSITION is a *maximum* start
- `ucfirst` / `lcfirst` touch only the first character
- `split //` with an empty pattern to explode into characters

## Go concepts a converter must teach
- `index` is `strings.Index`, which also returns -1 on a miss -- a rare clean
  mapping. But `index($s, $sub, $pos)` needs
  `off := strings.Index(s[pos:], sub); if off >= 0 { off += pos }`, and the
  `pos` offset must be clamped.
- `rindex` with a position argument is `strings.LastIndex(s[:pos+len(sub)], sub)`,
  which is easy to get off by one.
- **Byte offsets vs character offsets.** Perl's `index`/`length`/`substr` count
  characters when the string has the UTF-8 flag on, bytes otherwise. Go's
  `strings.Index` and `len` always count bytes. For pure-ASCII data they agree;
  the moment non-ASCII appears the converter needs a rune-index helper.
- `uc`/`lc` are `strings.ToUpper`/`strings.ToLower`. `ucfirst` has no stdlib
  equivalent: it is a rune-aware helper, not `strings.Title` (deprecated and
  wrong -- it titlecases every word).
- `ord("")` is 0 in Perl; indexing an empty Go string panics.
- `split //` is `strings.Split(s, "")` which splits on UTF-8 boundaries -- close
  to Perl's character semantics, unlike `[]byte(s)`.
