# Pass criteria

- category: `todo` (explicit-destruction transformation with report entry)
- diagnostic cites `input.pl:10` (DESTROY) and both death points
- diagnostic-must-contain: `DESTROY`, `refcount` (or `reference count`),
  `explicit`
- if converted: output must match `expected_stdout` byte-for-byte — ordering of
  the two `release` lines is the pass/fail line
- must-not: use runtime.SetFinalizer; must-not drop the DESTROY behaviour
