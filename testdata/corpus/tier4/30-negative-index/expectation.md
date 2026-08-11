# Pass criteria

- category: `convert-verify`
- converted program output must match `expected_stdout` byte-for-byte and the
  program must NOT panic; tripwires: `neg length:  ell` and
  `oob read: undef (no panic)`
- report-must-contain: `negative` — including a note for the negative substr
  LENGTH at input.pl line 17
- report-must-contain: `index`
- must-not: emit native Go indexing for possibly-negative indices without a
  diagnostic; must-not clamp negative indices to 0
