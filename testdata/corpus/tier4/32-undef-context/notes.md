# 32-undef-context: undef in numeric, string, and boolean context

Group: **C — convertible, but the naive conversion is subtly wrong**

## Construct
A missing hash key yields undef, not a panic (line 10). undef is 0 in
numeric context (line 11), "" in string context (line 12), false in boolean
context — but `defined` distinguishes it from all of them (line 13). The
truthiness table (lines 16-19): `0`, `"0"`, `""` are FALSE; `"00"`, `"0.0"`,
`"0E0"` are TRUE — Perl's boolean test is "empty string or the exact string
'0'", not "numerically zero".

## Why the naive conversion is subtly wrong
Go has neither undef nor truthiness. A converter modelling Perl scalars as
plain `string`/`int64` conflates undef with ""/0, changing `defined` checks
and warnings behaviour; one translating `if ($v)` as `v != 0` (numeric) makes
`"00"` false (Perl: true) and one translating as `v != ""` makes `"0"` true
(Perl: false). Both mistranslations pass every casual test that uses 1/"" as
the flag values.

## What the converter should do
- Category: **convert-verify**: the scalar model must have a distinct undef
  state (pointer, option type, or flag), and boolean conversion must be a
  helper implementing Perl's exact rule:
  undef -> false; string -> false iff `""` or `"0"`; number -> false iff
  zero. Map reads lower to comma-ok with undef on miss.
- `no warnings 'uninitialized'` (line 7) tells the converter warnings need
  not be reproduced; WITHOUT that pragma a faithful conversion would also
  emit "Use of uninitialized value" on stderr — the report should state
  whether warning emission is modelled at all, once per file.
- Forbidden: truthiness as `!= 0` or `!= ""`, or a scalar model where
  `defined` cannot be answered.

## Ideal diagnostic (word for word)
> input.pl:17: note P2G-W408: boolean context uses Perl truth ("" and "0"
> are false; "00", "0.0", "0E0" are true). Converted via perlrt.Bool.
> A numeric or empty-string test would misclassify the quoted-zero forms.

## What a human should do instead
Make the intent explicit: `defined && length` for "has a value",
`!= 0` after explicit numification for "numerically nonzero". The Perl
one-liner `if ($v)` was always ambiguous; porting is the moment to resolve
it.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0). Highlights: `level+1: 1`, `level str: <>`,
`bool: false`; table rows `'00' -> true`, `'0.0' -> true`, `'0E0' -> true`
versus `'0' -> false` (the numeric 0 and string "0" rows both display as
`'0'`). The "0E0" row is beloved by DBI-style APIs ("zero but true").
