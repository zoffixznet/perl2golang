# Pass criteria

- category: `refuse-file`
- diagnostic cites `input.pl:8` (the BEGIN block) and references line 20 (the
  parse-dependent call)
- diagnostic-must-contain: `BEGIN`, `compile`, `parse`
- must-not: emit any Go translation of line 20; must-not pick one prototype
  silently (note: default run prints `[a]|b`, TAG_GREEDY=1 prints `[a,b]` —
  emitting either unconditionally is a fail)
