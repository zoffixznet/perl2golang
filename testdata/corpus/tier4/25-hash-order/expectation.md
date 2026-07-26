# Pass criteria

- category: `convert-verify` (invariant check, not byte diff — no
  expected_stdout exists for this entry)
- report-must-contain a warning for lines 11/12/14: `iteration order`,
  `output`
- converted program invariants: five keys each exactly once per line; `keys:`
  line == `again:` line within one run; `csv:` order == `keys:` order
- must-not: emit `for k := range m` into output with no order warning;
  must-not silently sort (sorting requires a report entry saying order was
  changed)
