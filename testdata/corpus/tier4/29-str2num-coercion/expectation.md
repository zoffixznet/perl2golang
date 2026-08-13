# Pass criteria

- category: `convert-verify`
- converted program output must match `expected_stdout` byte-for-byte; the
  discriminating rows are `'0x10' -> 0`, `'010' -> 10`, `'3abc' -> 3`,
  `' 12 ' -> 12`, `'inf' -> Inf`
- report-must-contain: `coercion` `numify` - either word
- report-must-contain: `prefix`
- must-not: use strconv.Atoi/ParseFloat with ignored errors; must-not use
  ParseInt with base 0 for implicit coercions
