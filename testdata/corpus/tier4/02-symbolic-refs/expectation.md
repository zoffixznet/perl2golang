# Pass criteria

- category: `refuse-statement` (or `approximate` ONLY if the report proves the
  name set is closed and shows the generated dispatch map)
- diagnostics cite `input.pl:14`, `input.pl:15`, `input.pl:23`
- diagnostic-must-contain: `symbolic reference`, `run time`, `symbol table`
- if converted via the narrowing: running the Go must print exactly
  `expected_stdout`
- must-not: silently bind `$$name` to any one concrete variable without proof;
  must-not drop line 15's variable creation
