# Pass criteria

- category: `todo` (or `shim` with the punctuation-variable runtime)
- diagnostics cite `input.pl:12`, `input.pl:20`, `input.pl:21`
- diagnostic-must-contain: the variable name (`$"` / `$,` / `$\`), the affected
  sub, and the concrete divergence (`1|2|3`)
- if shim emitted: output must match `expected_stdout` byte-for-byte
- must-not: silently drop the `local` statements and report a clean conversion
