# Pass criteria

- category: `approximate` (prototype-honouring conversion, report entries for
  lines 8, 15, 23) OR `refuse-file` (prototype-blind converter, diagnostic
  containing `prototype` and the sub name)
- if converted: output must match `expected_stdout` exactly - `one=3` is the
  tripwire (a prototype-blind conversion yields `one=7` or a parse error)
- must-not: convert while ignoring prototypes; must-not flatten
  `zipfirst(@x, @y)` into four scalar arguments
