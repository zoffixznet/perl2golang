# Pass criteria

- category: `refuse-statement` (or `approximate` via whole-file context
  specialization, documented per call site in the report)
- diagnostic-must-contain: `wantarray`, `context`, the sub name(s)
- diagnostics reference `input.pl:9` and `input.pl:16`
- if specialized: output must match `expected_stdout` exactly - the
  `wrapped: n=4 w=x y z w` line proves context propagated through wrapper()
- must-not: convert `pieces` with a single fixed return shape and no diagnostic
