# Pass criteria

- category: `convert-verify`
- converted program output must match `expected_stdout` byte-for-byte; the
  divergent rows (`-7 % 3 = 2`, `7 % -3 = -2`, `ring index: 4`) are the
  tripwires
- report-must-contain: `sign`, `modul` (modulus/modulo) for the sites at
  `input.pl:10` and `input.pl:15`
- must-not: translate `%` token-for-token where an operand can be negative,
  without a diagnostic
