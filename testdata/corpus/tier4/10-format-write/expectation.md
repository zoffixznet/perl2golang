# Pass criteria

- category: `refuse-statement` (or `approximate` under the simple-fields
  narrowing)
- diagnostics cite `input.pl:9`, `input.pl:14` (formats) and `input.pl:21`
  (write)
- diagnostic-must-contain: `format`, `write`, `picture`
- if lowered: output must match `expected_stdout` byte-for-byte including the
  header lines and column alignment
- must-not: emit Go that treats the picture lines as printable text; must-not
  omit the STDOUT_TOP header
