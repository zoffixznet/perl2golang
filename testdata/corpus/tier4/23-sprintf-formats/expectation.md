# Pass criteria

- category: `shim` (perlrt sprintf/number-stringification helpers), with
  `refuse-statement` acceptable for the `%vd` line
- report entries cite `input.pl:7` (%vd), `input.pl:8` (%s of float),
  `input.pl:10` (%#b), `input.pl:15` (%d overflow), `input.pl:18` (ref
  stringification)
- diagnostic-must-contain: `%.15g` (or `stringification`), `%vd`
- if converted: output must match `expected_stdout` byte-for-byte; the
  tripwires are `float via %s: 0.3` and `big to %d: -1`
- must-not: map printf verbatim to fmt.Sprintf without diagnostics
