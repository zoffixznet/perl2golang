# Pass criteria

- category: `refuse-statement` (both the recursion at `input.pl:7` and the
  code blocks at `input.pl:13` and `input.pl:17`)
- diagnostic-must-contain: `recursive` (line 7), `code`, `during` matching
  (lines 13, 17)
- if the balanced-delimiter special case is generated: the three
  balanced/NOT-balanced lines must match `expected_stdout`; the code-block
  lines must STILL be refused
- must-not: strip `(?{...})` or `(*FAIL)` from patterns while keeping the
  match; must-not run the side effects a different number of times
