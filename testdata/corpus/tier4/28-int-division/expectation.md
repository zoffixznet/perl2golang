# Pass criteria

- category: `convert-verify`
- converted program output must match `expected_stdout` byte-for-byte;
  tripwires: `avg = 3.5` (not 3) and the `-7%2` pair (`-1` inside
  use integer, `1` outside)
- report-must-contain: `float` `fraction` - either word, for what plain /
  produces
- report-must-contain: `use integer` - noting the lexical semantics switch
  for lines 11-14
- must-not: emit Go integer division for plain Perl `/`; must-not apply one
  set of arithmetic rules to the whole file
