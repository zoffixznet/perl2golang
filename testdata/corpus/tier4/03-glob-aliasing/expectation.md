# Pass criteria

- category: `refuse-statement` (or `approximate` under the documented
  unconditional-top-level narrowing)
- diagnostics cite `input.pl:11`, `input.pl:12`, `input.pl:19`, `input.pl:22`
- diagnostic-must-contain: `typeglob`, `rebind`, the aliased name
- if converted: output must match `expected_stdout` byte-for-byte, proving
  aliasing (not copying) — the string `queue=1 2 3 4 total=42` must appear
- must-not: convert `*jobs = \@queue` into an array COPY
