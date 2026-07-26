# Pass criteria

- category: `refuse-statement`
- diagnostics: exactly 2, citing `input.pl:11` and `input.pl:18`
- diagnostic-must-contain: `eval EXPR`, `run time`, `stub`
- generated-code-must: contain a panic stub carrying the original Perl text at
  each eval site; all non-eval statements converted normally
- must-not: emit Go that inlines a guess at the evaluated code; must-not report
  the file as fully converted
