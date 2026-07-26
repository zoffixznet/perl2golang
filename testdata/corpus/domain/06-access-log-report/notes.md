# 06-access-log-report

**Domain:** log analysis. Combined-format Apache/nginx access log parser
reading **two files through `<>`** (rotated file first), with per-status
percentages, method counts, a top-paths table built from a three-level
rollup, an hourly histogram, and a server-error exit code.

## Constructs exercised
- `<>` diamond operator over multiple ARGV files, with `$ARGV` and the
  non-resetting `$.` recorded for skipped lines (a documented historical
  quirk in a comment).
- A large `/x` regex with **nine named captures** (`%+` hash access,
  including a `@+{qw(...)}` hash slice of the capture hash).
- **Three-level nested hash** `%agg{path}{status}{hits|bytes}` built purely
  by autovivification, later flattened into `%path_totals` (itself
  two-level and autovivified, including a conditional `errors` bucket that
  only exists for paths that had 4xx/5xx).
- Path normalisation with `s{/\d+(?=/|$)}{/:id}g` -- lookahead assertion.
- `'#' x $count` bar chart; `commify` via `reverse`+regex, the perlfaq5
  classic.
- `-`-vs-number bytes field (`$+{bytes} eq '-' ? 0 : $+{bytes}`).

## Conversion challenges
- `<>` semantics (implicit multi-file concatenation, `$ARGV`, running `$.`)
  need an explicit file loop in Go; the "line 10" in the expected output is
  a *global* line counter, which a per-file counter would get wrong.
- Named captures with `%+` and hash slices of it: Go's `regexp` has
  `SubexpNames` but no `%+` equivalent -- the converter must build the
  binding explicitly. All features here are RE2-safe *except* the
  `(?=/|$)` lookahead in `normalize_path`, which RE2 rejects; that one
  substitution must be reimplemented (segment split or manual scan).
- Autovivified numeric `+=` on paths that may not exist yet: Go needs
  nested map initialisation or a struct with a two-key map
  (`map[string]map[string]*PathStats`).
- `$st >= 400` compares the string status key numerically -- string keys
  with numeric comparisons throughout.
- `commify` relies on `reverse` in scalar vs list context (`scalar reverse`)
  -- context sensitivity a converter must not miss.
- The percentage lines depend on Perl `%.1f` rounding; `100*16/27` etc.
  must round identically in Go (`fmt` matches here, but only if the math
  stays in float64).
