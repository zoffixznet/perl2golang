# 19-build-scheduler

Dependency-file-driven build scheduler: Kahn topological sort with a
hash-of-arrays priority queue, a two-worker list-scheduling simulation,
memoized critical-path DFS, and cycle detection via die/eval.

## Constructs exercised
- config parsing: comment stripping `s/#.*//`, `split ' '`, directive
  dispatch on `$f[0]`, `$.` in errors, referential-integrity validation
- graph as `%deps`/`%rdeps` hash-of-arrayrefs built with autovivifying push
- an inline `package PQueue` class in a bare block: hash-of-arrays keyed by
  priority, FIFO within a bucket, `sort { $b <=> $a } grep {...} keys`
  to find the top non-empty bucket
- Kahn's algorithm with all iteration order pinned (`sort` before every
  nondeterministic hash-keys walk) -- determinism is engineered, and a
  converter must preserve exactly these tie-breaks
- `%indegree` built with `map { $_ => ... }`, decrement-and-test
  `--$indegree{$succ} == 0`
- two-worker simulation: `sort { $workers[$a] <=> $workers[$b] or $a <=> $b }`
  sorting INDICES by their values
- memoized recursion with `$memo{$t} //= ...` as the whole function body
- deep-ish graph copy `map { $_ => [ @{...} ] }` before corruption
- `die`-based cycle report listing stuck nodes; caught by `eval`

## Conversion challenges
- hash-of-arrays PQueue: Go wants container/heap; reproducing FIFO-within-
  priority requires a stable heap or sequence numbers -- a classic converter
  bug source since map iteration in Go is deliberately randomized
- Perl's engineered `sort keys` determinism must be recognized as
  semantics, not noise, and carried into Go
- `//=` memoization idiom -> explicit map-presence check
- sorting indices by the values they index
- shallow-vs-deep copy of the graph before mutation (aliasing hazard)

## Go teaching opportunities
- container/heap with tie-break fields, table-driven graph tests,
  topological sort as the canonical "maps need ordered iteration" lesson
