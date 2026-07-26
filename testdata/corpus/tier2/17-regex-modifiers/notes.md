# 17 - /x /i /m /s and pattern reuse

## What this exercises
The four modifier flags that change what a pattern *means*, each demonstrated
against a case where the flag's presence or absence changes the result.

## Perl constructs
- `/m`: `^` and `$` match at internal newlines - shown against the same
  pattern without `/m` (1 match vs 3)
- `/mg` combined, in list context
- `/i`: case-insensitive, matching both `ERROR` and `Error`
- `/s`: `.` matches newline - shown as "no match" without it, "matched" with it
- `/x`: a multi-line IPv4 validator with inline `#` comments and free
  whitespace, stored in a `qr//`
- `qr//` stored in a lexical and interpolated into another match (`/$word_re/g`)
- combining `/xi` with named captures and `%+`
- alternation with numeric ranges (`25[0-5] | 2[0-4]\d | 1\d\d | [1-9]?\d`)
- a non-capturing repeated group `(?: ... ){3}`
- `\b` word boundaries, `\w{5,}` bounded quantifiers
- `split /\n/` plus `grep { /^[A-Z]/ }` to filter lines

## Go concepts a converter must teach
- Go spells flags inline: `(?m)`, `(?i)`, `(?s)`. A converter must move Perl's
  trailing flags into a leading group - and must apply them to the *whole*
  pattern, since Perl's trailing flags are pattern-wide.
- **Go has no `/x`.** Free-spacing patterns must be mechanically collapsed:
  strip unescaped whitespace and `#`-to-end-of-line comments, being careful not
  to strip inside character classes or escaped whitespace. Getting this wrong
  silently changes the pattern.
- Go's `$` by default matches only at end of text (and not before a final
  newline the way Perl's does without `/m`). Perl's `$` without `/m` also
  matches *before* a trailing newline - a real off-by-one difference when
  matching chomped vs unchomped lines.
- `(?s)` in Go is `s` = "let `.` match `\n`", same as Perl - one of the clean
  correspondences.
- `qr//` variables become package-level `regexp.MustCompile`; interpolating one
  `qr` into another pattern requires string-building the source, which loses
  the compiled form.
- Perl's `/i` is Unicode-aware case folding; Go's `(?i)` is also Unicode-aware
  but the fold sets differ in edge cases - worth flagging for non-ASCII input.
