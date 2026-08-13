# Pass criteria

- category: `approximate` (static MRO flattening, linearizations in report)
- report must show D's order as D,B,A,C and D3's order as D3,B,C,A
- diagnostic-must-contain: `depth-first` (or `DFS`), `C3`, `hello`
- converted program output must match `expected_stdout` byte-for-byte -
  `dfs hello: hello from A` AND `c3 hello:  hello from C` must both hold
- must-not: use Go struct embedding to resolve the diamond; must-not apply one
  MRO to both D and D3
