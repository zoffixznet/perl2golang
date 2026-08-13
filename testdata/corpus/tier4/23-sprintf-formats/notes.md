# 23-sprintf-formats: sprintf/printf behaviours fmt cannot mirror

Group: **B - convertible only with an approximation that changes semantics**

## Construct
Seven formatting behaviours in one file:
- `%vd` on a plain string (line 7): joins each CHARACTER's ordinal with dots -
  observed `49.46.50.50.46.51.51.51` for "1.22.333" (NOT "1.22.333"; that
  output needs a v-string). Doubly treacherous.
- `%s` of a float (line 8): Perl stringifies via `%.15g`, printing `0.3` for
  `0.1 + 0.2`; Go's `%v`/`strconv.FormatFloat(-1)` prints the shortest
  round-trip form `0.30000000000000004` (line 9 shows the full value).
- `%#b` (line 10): `0b1010` - Go's fmt rejects `#` with `b`.
- runtime negative width via `%*s` (line 11): left-justifies.
- `%.2f` of 2.675 (line 12): `2.67` (the double is below 2.675) - Go agrees
  here, but only because both round the same double; the entry pins it.
- `%d` of 1e20 (line 15): silently produces `-1` (UV clamp of the oversized
  value printed through a signed conversion). No error, no warning (suppressed).
- `%s` of an array ref (line 18): `ARRAY(0x...)` - type name plus address.

## Why naive Go conversion changes semantics
Mapping Perl's sprintf verbatim onto `fmt.Sprintf` compiles and runs, and gets
`%s`-of-float wrong on EVERY value (Perl's %.15g vs Go's shortest-round-trip),
`%vd` catastrophically wrong, `%#b` and `%d`-overflow differently wrong. These
are output-format bugs that diff cleanly but pass casual review.

## What the converter should do
- Category: **shim**. Emit `perlrt.Sprintf` implementing Perl semantics for
  the verbs the file actually uses; fall through to `fmt` only for verbs whose
  semantics are proven identical (`%d` in-range, `%x`, plain `%s` of strings).
- Float-to-string ANYWHERE (interpolation, print, %s) must go through a
  `perlrt.NumToStr` (%.15g) helper, not Go defaults. This single rule removes
  the largest class of diffs between converted and original programs.
- `%vd`: refuse the statement unless the argument is a literal v-string; on a
  runtime string its meaning (char ordinals) is almost never what the author
  wanted - the diagnostic should say what it observed.
- Ref stringification `%s` of a reference: refuse or shim with a stable
  placeholder, and warn that addresses are process-specific (this input
  already masks the address for comparability).

## Ideal diagnostic (word for word)
> input.pl:8: warning P2G-W312: '%s' of a floating-point value uses Perl's
> %.15g stringification ("0.3"); Go's default prints "0.30000000000000004".
> Converted via perlrt.NumToStr to preserve Perl output.

> input.pl:7: warning P2G-W313: '%vd' applied to the plain string "1.22.333"
> prints the ordinal of each character ("49.46.50..."), which is rarely
> intended. Converted faithfully via perlrt.Sprintf; confirm this is the
> wanted output.

## What a human should do instead
Stop depending on implicit float stringification: pick a precision and write
`%.6g`/`%.2f` explicitly, which both languages honour identically. Replace
`%vd` with an explicit `join('.', unpack 'C*', $s)` if char ordinals really
were intended.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0) - every line is a conformance fixture; the three
that kill naive conversions: `vector: 49.46.50.50.46.51.51.51`,
`float via %s: 0.3`, `big to %d: -1`.
