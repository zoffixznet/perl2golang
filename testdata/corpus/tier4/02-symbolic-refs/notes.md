# 02-symbolic-refs: symbolic references (`$$name`, `&{"sub_$x"}()`)

Group: **A - genuinely impossible without an interpreter**

## Construct
Under `no strict 'refs'`, a string is used as a variable name (`$$name`, line 14),
a new package variable is created from a computed name (`${ $name . "_seen" }`,
line 15), and a sub is called by a name assembled at runtime
(`&{"handler_$which"}(6, 7)`, line 23).

## Why it resists conversion to Go
Go identifiers are resolved at compile time; there is no symbol table reachable
by string at runtime (reflection reaches exported struct members, not local or
package variables). The set of names touched here is only discoverable by running
the program: line 15 mints brand-new globals whose names are computed.

## What the converter should do
- Category: **refuse-statement** (default), with a documented narrowing:
  when every string that can flow into the symbolic lookup is statically
  enumerable (here: the literal `qw(alpha beta gamma)` and `qw(add mul)` lists),
  the converter MAY lower the construct to a generated `map[string]*value` /
  `map[string]func(...)` dispatch and say so in the report.
- If it cannot prove the name set, it must replace the statement with a panicking
  stub and report it. It must never resolve a symbolic ref to "the variable with
  that name I happen to see" without proving the name set is closed.
- The presence of `no strict 'refs'` (line 6) should itself trigger a file-level
  note: symbolic references are likely.

## Ideal diagnostic (word for word)
> input.pl:14: error P2G-E102: symbolic reference '$$name' looks up a variable by
> a string computed at run time. Go has no runtime symbol table. Replaced with a
> panicking stub. If the possible names are a fixed set, rewrite using a hash
> (Perl) / map (Go) keyed by those names.

Analogous diagnostics for line 15 (creation of `${$name . "_seen"}`) and line 23
(`&{"handler_$which"}`), each citing the exact source text.

## What a human should do instead
Replace name-mangled globals with a hash: `$seen{$name} = 1` instead of
`${$name."_seen"} = 1`; replace `&{"handler_$which"}` with a dispatch hash of
code refs. Both rewrites then convert to Go maps mechanically.

## Observed with perl 5.42.2 (x86_64-linux)
See `expected_stdout` (exit 0): `total=6`, `add: 13`, `mul: 42`, `alpha_seen=1`.
The output is the ground truth for a converter that implements the
enumerable-name-set narrowing.
