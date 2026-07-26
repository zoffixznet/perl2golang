# 05-passwd-audit

**Domain:** sysadmin glue. Audits a passwd-format dump against an
/etc/shells whitelist: duplicate UIDs, non-root UID 0, empty password
fields, malformed lines, home-directory policy, and a "review for removal"
heuristic over GECOS. Findings are graded CRIT/WARN/INFO and the exit code
is 1 when any CRIT exists.

## Constructs exercised
- `split /:/, $line, -1` -- the **-1 limit** keeps trailing empty fields,
  which is load-bearing: without it the malformed-line detection and the
  empty-shell case break.
- Hash-of-arrays inverted index (`%by_uid`) built by `push @{ $by_uid{...} }`
  (autovivification of the arrayref).
- A custom sort comparator sub (`numeric_or_string`) called explicitly from
  a `sort { }` block -- mixed numeric/string comparison depending on regex
  tests of both operands.
- Severity ranking via a lookup hash feeding a three-key sort.
- `grep`/`map`/`sort` pipeline for the stale-account heuristic, including a
  case-insensitive alternation regex over GECOS.
- Interpolated array in a string (`"@stale"`) with its space-join
  semantics.

## Conversion challenges
- The user record is a named-struct candidate (`name/pw/uid/gid/gecos/
  home/shell/line`); converting to `map[string]string` would obscure every
  downstream field access.
- `split` with negative limit has no direct Go equivalent --
  `strings.Split` keeps trailing empties by default, so the converter must
  know Perl's default *drops* them and that `-1` restores them (a
  semantics inversion between the languages).
- UID is compared with `eq '0'` (string) in one audit and `>= 1000`
  (numeric) in another; `%by_uid` keys are strings that sort numerically
  "when they can". This numeric/string duality forces Go to choose a
  representation and convert at the right boundaries.
- The exceptional-case suppression (`next if $uid eq '0' and join(...) eq
  'root,toor'`) encodes cross-audit deduplication ordering -- fragile if a
  converter reorders the audit passes.
- `$count{$f->[0]}++` inside the print loop means summary counts depend on
  iterating the *sorted* findings -- correct but easy to decouple wrongly.
