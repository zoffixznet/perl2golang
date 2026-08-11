# Pass criteria

- category: `convert-verify` (invariant check via verify.pl, not byte diff —
  no expected_stdout exists for this entry)
- report-must-contain: `iteration order` — a warning for lines 11/12/14
- report-must-contain: `output`
- converted program invariants, checked by verify.pl: five keys each exactly
  once per line; the keys and again lines equal within one run; the csv
  order equal to the keys order
- must-not, checked by verify.pl: silently sort (sorting requires a report
  entry saying order was changed)
