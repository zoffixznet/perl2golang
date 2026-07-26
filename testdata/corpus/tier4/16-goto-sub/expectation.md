# Pass criteria

- category: `approximate`
- report entry cites `input.pl:15` and must cross-reference the `caller` use at
  `input.pl:8`
- diagnostic-must-contain: `goto`, `tail call`, `caller`
- the tool must NOT claim byte-identical behaviour: the report must state the
  observable divergence on the `frame above me:` line
- must-not: drop the goto; must-not convert it to a Go goto statement
