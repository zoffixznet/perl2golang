# Pass criteria

- category: `refuse-statement` with poisoning: diagnostics for `input.pl:14`
  AND for every subsequent use of `$x` (lines 16, 17, 20)
- diagnostic-must-contain: `tie`, `FETCH`, `STORE`, `$x`
- if a full accessor shim is emitted instead: output must match
  `expected_stdout` byte-for-byte (both `(FETCH ran)` lines, correct ordering,
  `QUIET` upcasing)
- must-not: stub only the tie line while converting later `$x` uses as plain
  variable reads
