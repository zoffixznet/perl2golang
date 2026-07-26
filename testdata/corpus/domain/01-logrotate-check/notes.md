# 01-logrotate-check

**Domain:** sysadmin glue. Validates a logrotate-style config, then cross-checks
it against a collector snapshot of real log sizes/mtimes (fake clock passed as
argv so nothing depends on wall time). Exit 1 when any ERROR finding exists.

## Constructs exercised
- Two-phase line-oriented parser with an index-based `while` loop and manual
  lookahead (`$i` arithmetic), not a simple per-line loop.
- Block/stanza parsing with nested state (`postrotate ... endscript` swallows a
  script body).
- `eval { } or do { $@ ... }` per stanza: one malformed stanza (missing brace)
  is reported and skipped while parsing resumes -- the fixture actually
  triggers this path.
- `/x` regex with comments for directive parsing; `(?:...)?` optional groups.
- Hash-of-hashes stanza records; `%managed` alias map where several paths share
  one stanza reference (aliased references, not copies).
- List-of-arrayrefs findings sorted with a three-key `sort` chain.
- `map { $_ => 1 }` set idiom, `grep` over `sort keys` for deterministic order.

## Conversion challenges
- The stanza record is a hash used as a struct (`paths`, `line`, `directives`,
  `unknown`): Go should synthesize a named `Stanza` type, not `map[string]any`.
- `%managed{$path} = $st for @paths` shares one stanza pointer across keys;
  a naive value-copy translation changes semantics.
- The `eval`/`die` recovery with resynchronisation to the next `}` becomes an
  error-return plus explicit skip loop in Go; the `$@` string is part of the
  output so message text must be preserved (including the trailing-newline
  `die` convention that suppresses "at line" suffixes).
- `parse_directive` returns either a 2-list or empty list; callers test
  `defined $k`. Go needs an `(k, v string, ok bool)` shape.
- Directive values are string-or-1 (`defined $v ? $v : 1`): numeric/string
  duality where `rotate` is later regex-tested for numericness.
- Hash iteration (`keys %FREQ_SECONDS`, `keys %global`) is always wrapped in
  `sort` before affecting output -- a converter must preserve that or the Go
  version silently stays correct while the Perl idiom looks unordered.
