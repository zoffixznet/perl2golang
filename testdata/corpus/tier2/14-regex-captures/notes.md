# 14 - captures, named captures, alternation, anchors

## What this exercises
Parsing real syslog lines: a large `/x` pattern with named captures, numbered
captures with alternation, optional groups that may not participate, greedy vs
non-greedy, anchors and character classes.

## Perl constructs
- `qr/.../x` - a precompiled pattern object stored in a lexical and reused
- named captures `(?<mon>...)` and the `%+` magic hash
- `push @records, { %+ };` copying `%+` (it is reset by the next match)
- an optional non-capturing group with a capture inside:
  `(?:\[(?<pid>\d+)\])?` - `$+{pid}` is `undef` when absent
- numbered captures `$1 .. $4` with alternation `(Accepted|Failed)` and an
  optional non-capturing `(?:invalid user\s+)?`
- greedy `<(.+)>` versus non-greedy `<(.+?)>` on the same string
- POSIX class `[[:space:]]`, `\w`, `\d`, `\S`, `\s`
- anchors `^` and `$`, `\b`
- a capture group that may not participate (`(?:\.(\d+))?`) with a
  `defined $2 ? $2 : '000'` guard
- list-context match: `my ($h, $m, $s) = $str =~ /^(\d+):(\d+):(\d+)$/;`
- `substr($str, 0, 30)` for truncation

## Go concepts a converter must teach
- Go's `regexp` is RE2: no backreferences, no lookaround. None are used here,
  but a converter must detect and reject them elsewhere.
- Named captures exist (`(?P<name>...)`) but there is no `%+`. The idiom is
  `re.SubexpIndex("name")` or zipping `re.SubexpNames()` against
  `FindStringSubmatch` - a converter should generate a helper that builds a
  `map[string]string` once.
- **Non-participating groups are the trap:** Perl gives `undef`, Go gives `""`.
  `defined $2` cannot be expressed by inspecting the string, so the converter
  must use `FindStringSubmatchIndex` and check for `-1` to preserve the
  distinction.
- `$1` etc. are globals that persist after the match; Go returns a slice. Code
  that reads `$1` far from the match site needs the slice threaded through.
- `/x` needs the whitespace and comments stripped, or Go's `(?x)` flag... which
  does not exist. The converter must rewrite the pattern.
- `qr//` objects map to package-level `regexp.MustCompile` vars - a real
  performance win worth doing automatically.
- Perl `\d` is Unicode-aware by default in some builds; Go's `\d` is ASCII
  unless `(?U)`-style classes are used. Worth pinning.
