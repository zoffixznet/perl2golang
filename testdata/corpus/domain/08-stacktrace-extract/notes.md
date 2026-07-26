# 08-stacktrace-extract

**Domain:** log analysis. A two-state machine (NORMAL / IN_TRACE) that
pulls multi-line Java stack traces out of an application log, groups them
by exception class + first in-house frame (not by message, which embeds
order ids), and reports each distinct trace with its app frame, depth,
cause chain, and the line numbers where it appeared.

## Constructs exercised
- Explicit state machine with a `$state` string and a `$cur` accumulator
  hashref shared between the main loop and a `finish_trace()` helper that
  closes over it (file-scoped `my` acting as shared mutable state).
- Two `/x` regexes with named captures (`class`/`message`,
  `frame`/`src`), one relying on greedy `\S+` backtracking to find the
  `(` boundary.
- The back-to-back-trace re-check: after ending a trace, the *same* line
  is re-tested against the header regex -- a "reconsume current input"
  pattern that trips naive loop translations.
- Insertion-ordered grouping via parallel `%groups` + `@order` (push key
  on first sight) -- deterministic without sorting.
- `grep` in list context taking the *first* matching frame via
  `my ($app) = grep {...}`.
- `index($_->{frame}, $APP_PREFIX) == 0` as a starts-with test.

## Conversion challenges
- The trace accumulator is the clearest named-struct candidate in the
  corpus: `class, message, frames []Frame, caused []string, start, key,
  app_frame` with `Frame{frame, src}` nested -- a converter emitting
  `map[string]interface{}` here produces unmaintainable Go.
- `finish_trace` mutates outer `my` variables (`$cur`, `@traces`):
  becomes either closures or methods on a parser struct in Go; the
  `undef $cur` reset must translate to a nil assignment, not a fresh
  allocation aliasing bug.
- `my ($app) = grep {...}` -- list assignment truncation; Go needs an
  explicit find-first loop, and `$app` being undef (falsy) drives a
  ternary later.
- The header regex's `(?:Exception|Error)` suffix after greedy `[\w.]+`
  depends on backtracking; RE2 handles this (regular), but converters
  doing literal translation must confirm capture boundaries match.
- `$+{message} // '('none')'`-style defaulting and `$t->{app_frame} //
  '(no app frames)'` -- defined-or on possibly-absent keys, i.e. Go
  pointer-vs-zero-value decisions.
