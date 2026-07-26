# 20-version-bump

**Domain:** build/release glue. Bumps the release version across four
files that each embed it differently (Makefile, Perl module, README
prose, RPM spec). Runs in `--dry-run` here: emits a diff-style preview
instead of writing. Before bumping it cross-checks that every file
agrees on the current version and exits 2 on drift (path present but not
triggered by this fixture -- the spec's `%changelog` mentions an old
version, which the handler correctly ignores).

## Constructs exercised
- **Dispatch table of code refs keyed by basename** (`%HANDLERS`), each
  returning `(version, sprintf_template)` or the empty list -- the
  template carries `%s` plus captured prefix/suffix text
  (`return ($2, "$1%s$3")`), i.e. data built from regex captures.
- Slurp-to-array editing model: `my @lines = <$fh>`, `chomp @lines`,
  line index arithmetic, rewrite-by-index, and a write-back branch
  (`--write`) that is compiled but not exercised.
- Uniqueness enforcement (die if two lines match) using `$rec` as both
  accumulator and "already found" flag.
- Consensus check via a version -> [files] hash-of-arrays and a
  majority sort over `@{ $by_version{$b} }` counts.
- Mutually-exclusive flag validation; semver arithmetic in `bump` with
  cascading resets.

## Conversion challenges
- Handlers returning `(string, template)` *or* empty list: the Go
  translation needs `(ver, tmpl string, ok bool)` -- and the templates
  are runtime `sprintf` format strings assembled from captures, so the
  converter must keep them data (not inline them), or the `--write`
  path breaks.
- Capture variables (`$1`, `$2`, `$3`) interpolated into a *returned*
  string after the match -- their lifetimes end at the next match in
  Perl; a lazy translation that defers reading captures is wrong.
- The `%HANDLERS` iteration is `sort keys` so the diff order is
  alphabetical -- an unsorted Go map range would scramble the output
  nondeterministically: this entry is a direct detector for that bug
  class.
- In-place file editing semantics (read-all, mutate line, rewrite)
  contrasted with Perl's `-i` flag culture; the entry deliberately does
  it manually so the Go port is straightforward but must preserve the
  trailing-newline discipline (`map { "$_\n" }`).
- Prose-file safety: only the `Current release:` line matches; the
  README contains a decoy version-like string (`2019.06`) with a
  comment explaining the incident -- a test that overbroad regex
  "improvements" fail.
