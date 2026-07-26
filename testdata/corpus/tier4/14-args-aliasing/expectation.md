# Pass criteria

- category: `approximate` (mutation-analysis conversion) or `refuse-statement`
  for subs it cannot analyze
- report entries for subs `bump`, `blank_all`, `chompy` citing @_ mutation
- diagnostic-must-contain: `@_`, `alias`, `caller`
- converted program output must match `expected_stdout` byte-for-byte
  (`n=6 s=hi!` is the primary tripwire)
- must-not: emit value-parameter conversions of these subs without a diagnostic
