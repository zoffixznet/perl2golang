# Pass criteria

- category: `convert-verify`
- converted program output must match `expected_stdout` byte-for-byte;
  tripwires: `'00' -> true`, `'0.0' -> true`, `'0E0' -> true` alongside
  `'0' -> false`, and `defined: no` with `level+1: 1`
- report-must-contain: `undef`
- report-must-contain: `truth` `boolean` — either word
- must-not: model truthiness as `!= 0` or `!= ""`; must-not panic on the
  missing hash key
