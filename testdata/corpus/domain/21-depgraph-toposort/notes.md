# 21-depgraph-toposort

**Domain:** build/release glue. Resolves build order from `target: deps`
declarations using Kahn's algorithm with alphabetical tie-breaking, and
prints parallel "levels" for the CI farm. Three input files are
processed independently under `eval`: a good graph (11 targets, 5
levels), a cyclic graph (reports the cycle path), and a graph with
typo'd undeclared dependencies. Exit code is the number of failed
graphs (2).

## Constructs exercised
- **Meaningful `eval`/`die`**: per-file `eval { ...; 1 } or` recovery
  with `$@` printed; `die` messages composed to be user-facing.
- Kahn's algorithm: in-degree map, reverse-dependency hash-of-arrays,
  a ready queue re-sorted each level (`@ready = sort @ready`) for
  determinism, `--$indegree{$waiter} == 0` pre-decrement in a
  condition.
- Cycle reporting via a **recursive closure**: `my $walk; $walk = sub {
  ... $walk->($_) ... }` with an `%on_path` set, a `@path` stack with
  push/pop backtracking, and path-trimming to the cycle proper.
- `$declared{$target}++` post-increment as a "seen before?" test inside
  a die condition.
- `$n != keys %$deps` -- scalar-context keys in a numeric comparison.

## Conversion challenges
- The recursive closure (`my $walk; $walk = sub {...}`) is the sharpest
  edge: Go needs the identical two-step declaration (`var walk
  func(string)`) -- converters that inline or hoist it break the
  self-reference.
- `eval`-per-unit-of-work with error accumulation maps to Go's error
  returns cleanly, and the entry rewards a converter that produces
  `func processFile(...) error` instead of panic/recover.
- Determinism engineering is explicit (sorted ready queue, sorted DFS
  neighbours, sorted stuck nodes): the notes-worthy insight is that
  Perl's unordered hashes forced these sorts, and Go's randomised map
  ranges make the exact same sorts load-bearing -- remove any one and
  the output flaps.
- `push @{ $rdeps{$_} }, $t for @{ $deps->{$t} }` -- autovivified
  reverse index built in a postfix loop over a deref.
- The `$declared{$target}++` idiom inside a `die ... if` needs care:
  the increment must still happen on the non-dying path.
