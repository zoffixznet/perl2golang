# 21-string-increment: magic `++` on strings and magic ranges

Group: **B - convertible only with an approximation that changes semantics**
(fully specifiable, so the "approximation" can in fact be exact - via a shim)

## Construct
`$id++` on `"aa"` (line 8) does alphabetic carrying, not numeric addition:
`az → ba`, `zz → aaa`, `Zz → AAa`, `a9 → b0`, `z9 → aa0` (lines 11-14). The
range operator on strings (`"aa" .. "ac"`, line 17) iterates with the same
magic. The magic applies only while the scalar has been used purely as a
string - concatenation preserves it (lines 21-22), numeric use would kill it.

## Why naive Go conversion changes semantics
Go's `++` exists only for numbers. The reflexive conversion - coerce to number,
increment - turns `"aa"++` into `1` (Perl numifies "aa" to 0, then a numeric ++
gives 1), silently destroying ID generators, spreadsheet-column walkers and
hostname enumerators, which is where this idiom lives in real scripts.

## What the converter should do
- Category: **shim**. The magic-increment algorithm is small and fully
  specified (perlop "Auto-increment"): rightmost char steps within its class
  (a-z, A-Z, 0-9); overflow carries left; carry off the leftmost end prepends
  `a`/`A`/`1` matching the leftmost character's class. Emit
  `perlrt.StrInc(s) string` and translate `$v++` on string-typed scalars to it.
- The hard part the report must cover: deciding WHICH `++` is magic. It is a
  runtime property (POK-only scalar matching `/^[a-zA-Z]*[0-9]*$/`). Where the
  converter can type the scalar statically (all sites in this file are literal
  strings), pick statically; where it cannot, it must emit a runtime dispatch
  (numeric ++ if the value is numeric, StrInc if string-magic-eligible) and say
  so - NOT silently pick numeric.
- String ranges lower to a loop driven by the same helper with the documented
  termination rule (stop when the increment result exceeds the endpoint in
  length or passes it).

## Ideal diagnostic (word for word)
> input.pl:8: warning P2G-W310: '++' on the string "aa" is Perl's magic string
> increment (alphabetic carrying), not numeric addition. Converted via
> perlrt.StrInc. A numeric conversion here would have produced 1 instead of
> "ab".

## What a human should do instead
Nothing, if the shim exists - this one is mechanically translatable. Otherwise:
generate IDs numerically and format with a base-26 encoder written explicitly.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0) is the shim's conformance table: `id=ad`,
`az -> ba`, `zz -> aaa`, `Az -> Ba`, `Zz -> AAa`, `a9 -> b0`, `z9 -> aa0`,
`codes=aa ab ac`, `after concat: b0`.
