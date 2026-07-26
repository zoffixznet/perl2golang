# 27-negative-modulus: `%` with negative operands

Group: **C — convertible, but the naive conversion is subtly wrong**

## Construct
The four sign combinations of `%` (line 10) and the ring-buffer idiom
`-1 % 5` (line 15). Perl's `%` (outside `use integer`) takes the sign of the
RIGHT operand: `-7 % 3 == 2`, `7 % -3 == -2`. Go's `%` takes the sign of the
LEFT operand (truncated division): `-7 % 3 == -1`, `7 % -3 == 1`. Two of the
four combinations differ; `-1 % 5` is `4` in Perl and `-1` in Go.

## Why the naive conversion is subtly wrong
`a % b` translates to the SAME TOKEN in Go and compiles clean. The difference
only appears when a negative operand reaches it — typically an index
decrement, a clock/day-of-week calculation, or a ring buffer, where Go's `-1`
result then indexes out of bounds or wraps the wrong way. This is the single
most mechanical, most silent arithmetic divergence in the whole corpus.

## What the converter should do
- Category: **convert-verify**: translate every Perl `%` to a helper with
  Perl semantics, e.g.
  `func pmod(a, b int64) int64 { r := a % b; if r != 0 && (r < 0) != (b < 0) { r += b }; return r }`
  Only when BOTH operands are provably non-negative may it emit the native
  Go `%`, and the report should note which form each site got.
- `use integer` scopes (see entry 28) switch Perl to C semantics — inside
  them the native Go `%` is the CORRECT translation. The converter must track
  that lexical pragma.
- Forbidden: token-for-token `%` translation with no analysis and no
  diagnostic.

## Ideal diagnostic (word for word)
> input.pl:10: note P2G-W403: '%' has Perl semantics (result takes the sign
> of the right operand), which differ from Go's for negative operands
> (-7 % 3 is 2 in Perl, -1 in Go). Converted via perlrt.Mod. Sites with
> provably non-negative operands use native '%' (see report).

## What a human should do instead
Nothing if the helper is used. When porting by hand: audit every `%` whose
left operand can go negative, and use `((a % b) + b) % b` for the Perl/Python
behaviour.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0):
`  7 %   3 =   1` / ` -7 %   3 =   2` / `  7 %  -3 =  -2` / ` -7 %  -3 =  -1`
and `ring index: 4`. A native-`%` conversion produces `-1`, `1`, and
`ring index: -1` on the divergent rows.
