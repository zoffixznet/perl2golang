# Pass criteria

- category: `refuse-statement`
- diagnostic cites `input.pl:20`
- diagnostic-must-contain: `Greeter::hello`, `method`, `run time`
- generated-code-must: panic at the patch site if reached; must not let
  subsequent Greeter::hello calls silently use the ORIGINAL implementation
- must-not: report success while the converted program would print
  `after:  hello`
