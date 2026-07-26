# 08 - hash of hashes (a sparse matrix)

## What this exercises
A two-level hash used as a host x metric matrix, walked both row-wise and
column-wise, then extended to three levels with derived data.

## Perl constructs
- nested anonymous hash literal `%metrics = (host => { metric => value })`
- collecting the union of inner keys to get a stable column list
- `keys %{ $metrics{$host} }` - deref of a nested hash
- `$metrics{$host}{$col}` implicit-arrow subscript chaining
- `printf` inside a statement-modifier `for` to build a table row
- a sort whose comparator indexes into the outer hash:
  `sort { $metrics{$b}{$col} <=> $metrics{$a}{$col} || $a cmp $b }`
- taking the first element of a sort: `my ($hi_host) = sort {...} @hosts;`
- building `%detail` as host -> metric -> { value, status }
- nested ternary chain for the status bucket
- `exists $metrics{web1}{swap}` (does not create the key)
- `delete $metrics{cache1}{disk}` and re-reading `keys`
- `scalar(keys %metrics)`

## Go concepts a converter must teach
- `map[string]map[string]int`. Unlike the hash-of-arrays case, Go does *not*
  autovivify the inner map: `m["a"]["b"] = 1` panics if `m["a"]` is nil. Every
  nested write needs an explicit `if _, ok := m[k]; !ok { m[k] = map[...]{} }`.
- Reading `m["missing"]["x"]` in Go is safe and yields the zero value, whereas
  Perl *creates* `m{"missing"}` as a side effect (covered in entry 09).
- `exists` maps to the two-value map read `v, ok := m[k]`; `delete` maps to
  Go's builtin `delete`, which is one of the cleanest correspondences here.
- Three-level `map[string]map[string]struct{...}` is where a converter should
  start suggesting a real struct or a flattened composite key instead.
- The union-of-inner-keys pass is needed in Go for the same reason: neither
  language guarantees every row has every column.
- Ternary chains become `if/else if` - Go has no conditional expression.
