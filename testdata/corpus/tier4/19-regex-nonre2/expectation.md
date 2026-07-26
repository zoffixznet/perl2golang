# Pass criteria

- category: `shim` (alternate engine or proven-equivalent rewrite) or
  `refuse-statement` per pattern
- report entries cite `input.pl:8`, `input.pl:12`, `input.pl:15`,
  `input.pl:19`, `input.pl:22`, naming the RE2-incompatible feature in each
  (`backreference`, `lookahead`, `lookbehind`, `atomic`)
- if converted: output must match `expected_stdout` byte-for-byte
- must-not: emit an RE2 pattern with assertions dropped or backreferences
  mangled; a Go compile failure is acceptable only if the report predicted it
