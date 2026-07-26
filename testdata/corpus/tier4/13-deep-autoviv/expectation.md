# Pass criteria

- category: `shim`
- report entries cite `input.pl:8`, `input.pl:14`, `input.pl:19`
- diagnostic-must-contain: `autoviv`, `exists`
- converted program output must match `expected_stdout` byte-for-byte; the
  discriminating lines are `h{a} EXISTS` / `h{a}{b} EXISTS` / `leaf absent` /
  `EXISTS (viv'd by a sub call)`
- must-not: convert deep reads as plain map indexing with no diagnostic
