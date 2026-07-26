# Pass criteria

- category: `approximate` (declared numeric model) or `shim`
  (checked-arithmetic helpers)
- report entries cite `input.pl:10` and `input.pl:14` naming the promotion
  behaviour and the chosen Go representation
- diagnostic-must-contain: `overflow`, `int64`, `float` (or `double`)
- if shim-grade: output must match `expected_stdout` byte-for-byte including
  the `%.15g` stringifications
- must-not: emit wrapping int64 arithmetic for these lines with no diagnostic
  (a converted program printing a NEGATIVE IVmax+1 with a clean report is the
  canonical fail)
