# Pass criteria

- category: `refuse-statement` (or `approximate` under the documented
  static-specialization narrowing)
- diagnostic-must-contain: `AUTOLOAD`, the undefined method name(s), a reference
  to `input.pl:11`
- diagnostics cite the call sites of `get_color`, `set_size`, `get_size`,
  `launch_missiles`
- if specialized: output must match `expected_stdout` exactly, INCLUDING
  `error: no such method: launch_missiles`
- must-not: report success with a Go type whose method set is only {new}
