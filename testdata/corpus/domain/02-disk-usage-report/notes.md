# 02-disk-usage-report

**Domain:** sysadmin glue. Parses `df -k` fixture output (including the
classic wrapped-line form where a long device name sits alone on its own
line), applies warn/crit thresholds, prints a mail-ready table, and exits
with Nagios-style codes (0/1/2). Expected exit is 2 (two CRIT mounts).

## Constructs exercised
- `Getopt::Long` into a hash with negatable option (`skip-pseudo!`) and
  defaulting via `||=`.
- Stateful line gluing (`$pending_dev`) across loop iterations -- a tiny
  state machine driven by the `/^(\S+)\s*$/` "device only" pattern.
- `splice @rest, 0, 4` positional destructuring plus `join ' '` re-glue for
  the mount tail.
- **Dispatch table of sort comparators** (`%SORTERS`) holding coderefs that
  close over the package globals `$a`/`$b` -- `sort $sorter @fs`.
- Record hashes used as structs; classification mutates records in place.
- `exit(($crit ? 2 : 0) || ($warn ? 1 : 0))` -- `||` over integers where 0
  is falsy, an idiom Go must rewrite as explicit if/else.

## Conversion challenges
- The filesystem record (`dev/fstype/size/used/avail/pct/mount/level`) is a
  struct in disguise: a converter should emit a named `Filesystem` type.
- Sort comparators via `$a`/`$b` package globals passed as a scalar coderef:
  in Go this becomes `sort.Slice` with a chosen `less` function selected
  from a map of closures -- the mechanics differ completely.
- `%` stripping with `(my $pct = $pcts) =~ s/%$//` copies-then-mutates; the
  copy semantics matter.
- Numeric/string duality: `$pct` and `$size` stay strings until regex-
  validated, then are used in `>=` comparisons and arithmetic.
- `human_k` does floating division with unit walking and then a string
  fix-up (`s/\.0$//`) -- easy to get subtly wrong with Go float formatting.
- `warn_line` deliberately prints to stdout (comment explains why); a Go
  author's instinct to use stderr would break output comparison.
