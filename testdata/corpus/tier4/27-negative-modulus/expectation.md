# Pass criteria

- category: `convert-verify`
- converted program output must match `expected_stdout` byte-for-byte; the
  divergent rows (`-7 % 3 = 2`, `7 % -3 = -2`, `ring index: 4`) are the
  tripwires
- report-must-contain: `sign` — for the sites at input.pl lines 10 and 15
- report-must-contain: `modul` `negative operand` — modulus or modulo by
  name, or the operand discussion that identifies the same construct
- must-not: translate `%` token-for-token where an operand can be negative,
  without a diagnostic
