# 13-mail-header-parse

**Domain:** text/data munging. Reads a raw RFC 5322 message from
**STDIN**, unfolds folded headers (continuation lines starting with
whitespace), parses names case-insensitively into an order-preserving
multimap, prints the ops-relevant fields, reconstructs the Received chain
oldest-first, decodes RFC 2047 Q-encoded subjects, and runs hygiene
checks (DKIM present, single From, Message-ID, spam score).

## Constructs exercised
- Two-phase transformation: slurp-until-blank, then unfold by appending
  to `$unfolded[-1]` (negative index mutation).
- Order-preserving multimap: `%headers` (lc name -> arrayref of values)
  plus `@order` for first-seen sequence -- the classic Perl substitute
  for an ordered dict.
- Header-name regex `[!-9;-~]+` -- a character-class encoding of
  "printable ASCII minus colon" that looks like line noise but is
  RFC-precise.
- `/xi` regex with **named captures and nested optional groups** for
  Received parsing (`from`, `by`, optional `with`, optional `date` after
  `;`), relying on lazy `.*?` scanning.
- `reverse @received` for delivery order; per-hop numbered output.
- Q-decoding with `s{...}{ ...code... }ge` -- an **e-modifier
  substitution** whose replacement runs `tr/_/ /` and a nested
  `s/=HH/chr hex $1/ge` on the captured text.
- `($headers{from} && @{$headers{from}}) > 1` -- `&&` returning its right
  operand which is then evaluated in numeric (scalar) context.

## Conversion challenges
- `s///ge` with a code block replacement is the headline: Go needs
  `ReplaceAllStringFunc` with a nested replacement inside the callback,
  and the inner `chr hex $1` byte semantics (the fixture's `=E2=80=93`
  en-dash decodes to three raw bytes, not a rune conversion).
- The multimap-with-order structure: in Go, `map[string][]string` plus a
  `[]string` order slice -- converters that reach for only a map lose
  first-seen ordering the output depends on.
- Optional named captures that may be absent (`$+{with}` undef vs empty):
  Go's `FindStringSubmatch` returns "" for both; the output format
  distinguishes them (`defined` checks), so the converter must use match
  indices to preserve undef-ness.
- Lazy `.*?` with backtracked alternatives in the Received regex is
  RE2-expressible but capture extents must be verified, not assumed.
- The unfold step's "append to previous element" mutation and the
  `(my $cont = $line) =~ s/.../` copy idiom.
