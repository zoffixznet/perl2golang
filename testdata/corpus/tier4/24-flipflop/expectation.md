# Pass criteria

- category: `shim` (per-site generated flip-flop state)
- report entries cite `input.pl:17` and `input.pl:23` as two INDEPENDENT
  state sites
- diagnostic-must-contain: `flip-flop`, `state`
- converted program output must match `expected_stdout` byte-for-byte -
  including `state=4E0 END` and `state=3E0 END`
- must-not: treat `..` as a range or as `&&`; must-not share state between
  the two sites
