# Pass criteria

- category: `shim` (static lowering to method calls) or `refuse-statement`
  where operand types are unprovable
- report cites `input.pl:8` (the overload declaration) and each operator site
  (lines 23, 25, 26)
- diagnostic-must-contain: `overload`, `Money`, the operator symbol
- converted program output must match `expected_stdout` byte-for-byte
  (`sum: $4.00` is the stringification tripwire)
- must-not: apply native Go +/*/== to Money values without diagnostics
