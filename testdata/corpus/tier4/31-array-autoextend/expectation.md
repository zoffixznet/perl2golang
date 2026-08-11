# Pass criteria

- category: `convert-verify`
- converted program output must match `expected_stdout` byte-for-byte and
  must not panic; tripwires: `joined: [1,2,3,,,,,8]` (undef gaps stringify
  empty) and `re-extended len: 5, last defined? no` (no value resurrection)
- report-must-contain: `extend` — for input.pl lines 9, 17 and 20
- report-must-contain: `undef`
- must-not: translate `$a[7] = 8` as append; must-not make gap elements
  defined zero values
