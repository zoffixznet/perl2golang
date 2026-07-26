# Pass criteria

- category: `shim` (perlrt.StrInc or equivalent)
- report entries cite `input.pl:8`, `input.pl:13`, `input.pl:17`, `input.pl:22`
- diagnostic-must-contain: `string increment`, `carry` (or `carrying`)
- converted program output must match `expected_stdout` byte-for-byte; the
  seven mappings (ba, aaa, Ba, AAa, b0, aa0, and id=ad) are the conformance
  table
- must-not: numify the operand and apply numeric ++ without a diagnostic
