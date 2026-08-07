# Pass criteria

- category: `approximate` (lexical folding, stated at the local)
- diagnostic-must-contain: `local`, `sub`, and either `block` or `call`
- diagnostics reference the `local $/ = '|'` line (`input.pl:23`)
- generated-code-must: convert both calls and run to the last line; the
  "as fields" line may disagree with perl, and that disagreement must be
  exactly what the report's local note describes
- must-not: introduce a mutable global separator the generated reads
  consult; must-not fold the localised value into the sub's own read, which
  would break the first call

The sibling shapes convert exactly: `local $/` governing reads written in
the same block, and a separator value worked out at run time, both land in
the calls as arguments. This entry is dynamic scoping proper, where the
localised value has to reach a sub compiled elsewhere. The honest outcome
is the stated approximation: the sub keeps the default, and the report says
a called sub was meant to see the change and how to pass it explicitly.
