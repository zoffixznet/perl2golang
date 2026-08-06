# Pass criteria

- category: `approximate` (mid-list array reduced to a stated stand-in) or
  `refuse-statement`
- diagnostic-must-contain: `array`, `return`, `count`
- diagnostics reference `input.pl:12`
- generated-code-must: keep every statement around the return converted and
  runnable; the `flat:` line may differ from perl but must be produced
- must-not: silently emit a fixed-arity return that shifts the values after
  the array with no diagnostic; must-not drop the trailing `scalar(@parts)`
  result

The trailing-array form, `return ($cost, @path)`, converts exactly (the
array becomes the function's final slice result). This entry is the shape
one position to the left, which no fixed result count can carry: `@parts`
sits between two fixed values, so where `scalar(@parts)` lands depends on
how many parts there were. The tool must say what it did to the array, at
the return, in the report.
