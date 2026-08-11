# Pass criteria

- category: `convert-verify`
- converted program output must match `expected_stdout` byte-for-byte on every
  run (ties in input order, both ascending and descending)
- report-must-contain: `stable` — for the sorts at input.pl lines 14 and 18
- must-not: emit sort.Slice (unstable) for these sorts without a diagnostic
  admitting tie order may change
