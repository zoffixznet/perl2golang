# 05-json-pipeline

Order-file ETL: decode JSON fixture, roll up nested orders into a
per-customer summary, emit canonical pretty JSON plus a plain-text table.

## Constructs exercised
- `JSON::PP` with `canonical(1)` and `pretty`; `JSON::PP::true/false` booleans
- slurp-file-via-`do`-block (`local $/; <$fh>`) used as an expression
- deeply nested autovivification-adjacent update: `$by_customer{$name} //= {...}`
  returning the slot, then mutating through it
- nested loops over arrayrefs of hashrefs (`@{ $doc->{orders} }`)
- accumulating into two hashes at once; float normalisation with
  `0 + sprintf '%.2f'` so JSON emits numbers, not strings
- `sort grep { ... } keys`, list slice of a sort result `(sort ...)[0..1]`
- booleans surviving an encode -> decode -> encode round trip
- table formatting via `printf` before machine output

## Conversion challenges
- JSON booleans: Perl's `JSON::PP::true` objects vs Go `bool`; the truthiness
  test `$c->{any_pending} ? 'yes' : 'no'` works on the object
- schema-less decoding: Perl gets hashrefs/arrayrefs of mixed types; Go must
  either define structs or navigate `map[string]interface{}` with assertions
- canonical (sorted-key) encoding: Go's encoding/json sorts map keys but NOT
  struct fields -- field order must be arranged or maps used
- `spend` printed as `83.25` / `22.5` / `0` -- Perl number stringification in
  JSON differs from Go's float formatting (`22.5` not `22.50`, `0` not `0.00`)
- pretty-print format is JSON::PP's 3-space style; exact-output matching
  requires custom indentation (Go's MarshalIndent differs: no space before `:`)
- `//=` initialise-and-return-slot idiom

## Go teaching opportunities
- typed structs + `encoding/json` tags vs generic maps; json.Number tradeoffs
- deliberate float rounding before marshal; stable ordering strategies
