# Pass criteria

- category: `convert-verify`
- converted program output must match `expected_stdout` byte-for-byte;
  tripwires: `bare in list:  0 elements` vs `undef in list: 1 elements`,
  and `keys:  30,name` (the shifted hash)
- report-must-contain: `empty list`, `return`
- must-not: conflate `return;` with `return undef;` in list context;
  must-not build the line-17 map with keys name/age
