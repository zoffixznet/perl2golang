# 07-error-log-cluster

**Domain:** log analysis. Groups similar error-log lines into clusters by
normalising volatile tokens (quoted strings, hex addresses, paths, numbers
with units, bare numbers, numbered hostnames) into placeholders, then
reports clusters sorted by frequency with first/last line numbers and an
elided example.

## Constructs exercised
- An ordered pipeline of six `s///g` substitutions where **order is
  load-bearing** (comments in the code explain why quoted strings must go
  before paths, and hex before bare numbers).
- `||=` hashref initialisation caching first-seen metadata (`first_line`,
  `example`) while later hits only bump `count`/`last_line` -- a
  first-wins/last-wins split inside one record.
- Level promotion logic (warn cluster promoted to error if any member was
  an error).
- Three-key sort: count desc, first_line asc, signature asc.
- A non-ASCII placeholder (`'…'`, three UTF-8 bytes in a non-`use utf8`
  source file) flows through byte-semantics strings into the output.

## Conversion challenges
- The placeholder `…` is bytes in Perl (no `use utf8`): Go strings are
  bytes too, so this *works*, but any converter that "helpfully" decodes
  to runes and re-encodes must not change the byte sequence; `elide` uses
  `length` (bytes in this Perl) so a rune-based length would change
  truncation points.
- Latent regex trap worth teaching: rule 4's unit regex ends in
  `(?:ms|s|MB|GB|KB|%)\b` -- the `\b` after `%` can never match before a
  space, so `41%` is actually normalised by the *bare number* rule to `N%`.
  The expected output encodes this real behaviour; a converter that
  "cleans up" the regex changes the signatures.
- `$cluster{$sig} ||= {...}` returns the ref either way; the aliasing via
  `$c` must survive translation (mutating a copy breaks counts).
- Substitution with a replacement containing the captured group
  (`s/\b([a-z]+)-\d+\b/$1-N/g`) -- Go needs `ReplaceAllString` with `$1`
  syntax differences (`${1}-N` to avoid ambiguity).
- The cluster record is struct-shaped (`count/level/first_line/last_line/
  example`) -- another named-type candidate.
