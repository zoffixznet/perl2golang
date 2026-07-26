# 29-str2num-coercion: string-to-number coercion rules

Group: **C — convertible, but the naive conversion is subtly wrong**

## Construct
Eleven strings coerced with `$s + 0` (line 10). Perl parses the longest
numeric PREFIX and never errors: `"3abc"` is 3, `"0x10"` is 0 (hex syntax is
NOT recognized in numeric context), `"010"` is 10 (no octal either),
`" 12 "` is 12 (leading whitespace skipped, trailing junk ignored), `"1e3"`
is 1000, `"1_000"` is 1 (underscores only work in literals), `"inf"`/`"nan"`
ARE recognized (Inf/NaN), `""` is 0. `hex()` and `oct()` (lines 13-14) are
the explicit converters — `oct()` handles 0x/0/0b prefixes.

## Why the naive conversion is subtly wrong
`strconv.Atoi`/`ParseFloat` ERROR on almost all of these; a converter that
uses them and ignores the error gets 0 for `"3abc"` (Perl: 3) and 0 for
`" 12 "` (Perl: 12 — ParseFloat rejects the trailing space). One that uses
`ParseInt(s, 0, 64)` gets 16 for `"0x10"` (Perl: 0!) and octal 8 for `"010"`
(Perl: 10). Every easy Go choice disagrees with Perl on some row; scripts
that parse loosely formatted data (log scraping, config reading) hit these
constantly.

## What the converter should do
- Category: **convert-verify**: every implicit numeric coercion of a string
  must go through a shim (`perlrt.NumifyF`/`NumifyI`) implementing Perl's
  prefix-parse rule: skip leading whitespace, optional sign, digits with
  optional decimal point and exponent, `Inf`/`Infinity`/`NaN`
  case-insensitive; stop at the first invalid character; empty/invalid
  prefix yields 0; NEVER an error.
- The shim must NOT accept `0x`, `0b`, octal, or underscores. `hex()`/`oct()`
  convert to dedicated helpers.
- A diagnostic per file (not per site) is enough when the shim is used; a
  converter WITHOUT the shim must flag every coercion of a
  non-provably-numeric string as a divergence.
- Forbidden: strconv with discarded errors, or ParseInt with base 0.

## Ideal diagnostic (word for word)
> input.pl:10: note P2G-W405: implicit string-to-number coercion uses Perl's
> longest-numeric-prefix rule ("3abc" -> 3, "0x10" -> 0, "" -> 0, never an
> error). Converted via perlrt.Numify. Go's strconv would reject or
> reinterpret several of these values (e.g. ParseInt(s,0,64) reads "0x10" as
> 16).

## What a human should do instead
Validate inputs explicitly at the boundary (`/^\s*[-+]?\d+/` extraction, or
strconv with handled errors) so the program's tolerance for junk is a
decision, not an accident.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0) is the shim's conformance table. Highlight rows:
`'3abc' -> 3`, `'0x10' -> 0`, `'010' -> 10`, `' 12 ' -> 12`, `'1_000' -> 1`,
`'inf' -> Inf`, `'nan' -> NaN`, `'' -> 0`, and `oct()` yielding `16 8 5`.
Note the capitalization Perl prints for Inf/NaN — the shim's stringification
must match it (see also entry 23).
