# 22-int-overflow-float: silent IV → UV → NV promotion

Group: **B - convertible only with an approximation that changes semantics**

## Construct
Perl numbers escalate representation silently: `IVmax + 1` (line 10) promotes
to UV and stays EXACT (`9223372036854775808`); `UVmax + 1` (line 14) becomes a
DOUBLE (`1.84467440737096e+19`) which then absorbs `+ 1000` without changing
(line 17). Separately, `**` always returns an NV, so `2**53` prints in
e-notation and `2**53 + 1 == 2**53` is true.

## Why naive Go conversion changes semantics
Go `int64` WRAPS: `IVmax + 1` is `-9223372036854775808`, not the positive
value Perl prints. Go `uint64` fixes that case but wraps at the next line.
Go `float64` for everything reproduces the precision loss but breaks exact
64-bit integer arithmetic elsewhere. There is no single Go numeric type with
Perl's behaviour; every choice is an approximation.

## What the converter should do
- Category: **approximate**, with a declared numeric model. The recommended
  model: `int64` by default; at every arithmetic op the converter cannot prove
  overflow-free, either (a) insert an overflow-checked helper that promotes to
  float64 exactly like Perl (shim-grade fidelity), or (b) keep raw int64 and
  emit a diagnostic at each such site admitting wrap-on-overflow.
- `**` must convert to `math.Pow` (float) even for integer literals, or the
  report must state the divergence in stringification (`9.00719925474099e+15`
  vs `9007199254740992`).
- Forbidden: converting to int64 with NO overflow diagnostics - the wrap is
  silent data corruption in accounting/ID scripts.

## Ideal diagnostic (word for word)
> input.pl:10: warning P2G-W311: '$big += 1' can exceed int64 (Perl would
> promote to unsigned, then to float with precision loss; Go int64 wraps to
> negative). Converted with the checked-arithmetic helper perlrt.AddInt, which
> reproduces Perl's promotion. Sites proven overflow-free use native int64.

## What a human should do instead
Decide what the numbers MEAN. Counters/IDs: use uint64/big.Int deliberately.
Money: integers of the smallest unit. Scientific values: float64 and accept
what that implies. Perl's auto-promotion was hiding this decision.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `IVmax+1: 9223372036854775808` (exact, positive -
int64 wrap gives a NEGATIVE number here), `UVmax+1: 1.84467440737096e+19`,
`absorbs +1000? YES`, `2**53: 9.00719925474099e+15`, `2**53+1 == 2**53? YES`.
Note the exact stringifications: Perl prints integral UVs in full digits but
NVs via %.15g.
