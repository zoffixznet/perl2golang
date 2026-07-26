# 12-field-cleaner

**Domain:** text/data munging. Reads a pipe-delimited CRM export from
**STDIN**, applies a per-column cleaner selected by header name (names,
emails, phones, dates, US states), re-emits cleaned rows, then a comment
report of rewrites and unfixable values. Handles the exporter's habit of
wrapping long values onto a continuation line with no delimiters. Exit 1
because unfixable values exist. All sample data is obvious placeholder
data (example.com addresses, 555-01xx phones).

## Constructs exercised
- **Dispatch table of code refs** (`%CLEANERS`) wired into an ordered
  `@pipeline` via `map { $CLEANERS{$_} || \&clean_passthrough } @cols` --
  column order in the input decides execution order.
- Uniform cleaner contract: `($raw) -> ($clean, $changed, $note)`; a
  three-value list return where the third element is usually undef.
- Continuation-line gluing driven by counting delimiters with `tr/|//`
  against `$#cols`, pulling extra lines from STDIN mid-loop.
- `tr/0-9//cd` (complement+delete) for phone digit extraction;
  `s/^1(?=\d{10}$)//` lookahead for country-code stripping.
- Case folding with `\u$1` in replacements (`s/\b(\w)/\u$1/g`) plus the
  Mc/O' surname exceptions -- replacement-side case escapes.
- Date parsing with three alternative regexes (one `/x` with embedded
  spacing), a two-digit-year pivot, and a leap-year table (`@dim`).
- `(my $digits = $raw) =~ tr/.../` copy-and-modify idiom.

## Conversion challenges
- `\u` in the replacement string does not exist in Go regexp; title-casing
  needs `ReplaceAllStringFunc` -- a mechanical-translation blocker.
- The lookahead `(?=\d{10}$)` is not RE2-compatible; the converter must
  restructure (length check + prefix test).
- The cleaner signature `(clean, changed, note)` where note is
  undef-or-string: Go wants `(string, bool, error)` or a small result
  struct -- forcing an API design decision, not just syntax translation.
- Reading extra lines from STDIN *inside* the processing loop (`defined(my
  $more = <STDIN>)` in a while condition, with `my` in the condition)
  breaks the simple scanner-loop shape.
- `%CLEANERS` holding `\&sub` refs vs. Go function values: fine, but the
  fallback `|| \&clean_passthrough` needs the "hash miss is falsy"
  behaviour made explicit.
- Numeric string checks (`$y < 100`, `$y % 4`) on captured strings --
  parse points must be inserted correctly.
