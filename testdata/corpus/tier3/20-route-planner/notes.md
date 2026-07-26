# 20-route-planner

Dijkstra transit planner over a hash-of-hashes adjacency map, with mixed
directed/undirected edges, path reconstruction, and reachability reports.

## Constructs exercised
- CLI args with defaults: `my ($file, @queries) = @ARGV;` + `||=` and an
  `unless @queries` fallback list; `Src:Dst` query strings split at use
- graph fixture parsing: `s/^>\s*//` RETURN VALUE used as the
  "is directed" flag (substitution-as-boolean idiom)
- `%edge` hash-of-hashes with autovivification (`$edge{$a}{$b} = $w`),
  duplicate detection via `exists` on a nested key
- edge count via `scalar( map { keys %$_ } values %edge )` -- `keys` in
  list-flattening context
- Dijkstra with deterministic extract-min:
  `sort { $dist{$a} <=> $dist{$b} or $a cmp $b } grep {...} keys %dist`
  (O(V^2) but order-pinned)
- path reconstruction with
  `unshift @path, $prev->{ $path[0] } while exists $prev->{ $path[0] };`
  (statement-modifier while + unshift walking a prev-chain backwards)
- multi-value return `( \%dist, \%prev )` and `( $cost, @path )` -- scalar
  followed by list; empty-return signalling unreachability
- eval-wrapped per-query error handling; early-exit optimization
  (`last if $u eq $dst`)
- nested `$i/$j` loops over a probe list, parallel assignment
  `( $best, @bestpair ) = ( $val, @probe[ $i, $j ] )` with an array slice

## Conversion challenges
- `( $cost, @path )` mixed scalar+list returns and bare `return;` as
  "not found" -- Go needs (int, []string, bool) or error; converters
  routinely mangle Perl's list-return conventions
- substitution return value as a boolean flag is easy to mistranslate
- `%dist` doubling as both distance map AND visited-frontier (presence =
  discovered) -- two maps in Go, or comma-ok discipline
- deterministic linear extract-min must not be "optimized" into a heap
  without keeping the name tie-break
- autovivified nested map writes

## Go teaching opportunities
- map[string]map[string]int graphs, comma-ok idiom, slices for path
  building (prepend vs append+reverse), multiple return values done right
