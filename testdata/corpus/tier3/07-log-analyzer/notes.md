# 07-log-analyzer

Access-log summarizer: CLI options, an `/x` regex, autovivified two-level
counters, and a report with runtime-computed column widths.

## Constructs exercised
- `Getopt::Long` into a hash: `top=i`, `min-bytes=i` (hyphenated key!),
  negatable `errors-only!`; `shift @ARGV // default` for the positional arg
- `%ENV` threshold with fixed default (`$ENV{LOG_BIG_BYTES} // 4096`)
- `qr{...}x` extended regex with comments; list capture directly in boolean
  context: `my (...) = /$line_re/ or ++$malformed, next;` (comma operator +
  `next` inside a statement modifier)
- chained ternary ladder classifying status codes
- autovivification: `$by_path{$path}{count}++` creating nested hashes
- `splice @paths, $opt{top}` truncating a sorted list
- `sprintf`/`printf` with `%-*s` and `%*d` -- widths computed via
  `max(map { length } ...)` at runtime
- two-key sorts (count desc then name), `sum0`, array-of-arrayrefs for big
  hits, `printf ... @$_` flattening a tuple into format args
- implicit `$_` in `while (<$fh>)` + bare `chomp`, malformed-line counting

## Conversion challenges
- `%*d` star-widths: Go's fmt supports `%*d` since 1.24-ish via explicit
  width args -- but historically converters must thread widths correctly
- `... or ++$malformed, next;` -- comma-operator side effect plus loop
  control fused into one expression; naive translation drops the increment
  or the `next`
- autovivified `map[string]map[string]int` requires explicit inner-map
  initialisation in Go
- hyphenated option name `min-bytes` cannot be a Go identifier
- regex with `/x` whitespace/comments must be collapsed for Go's RE2
- the boolean flag `errors-only!` (allows --no-errors-only)

## Go teaching opportunities
- flag package vs manual arg handling; text/tabwriter as the idiomatic
  alternative to computed printf widths; structured counters via typed maps
