# 18-mro-c3: multiple inheritance with a non-trivial MRO

Group: **B — convertible only with an approximation that changes semantics**

## Construct
A diamond: `D` inherits `(B, C)`, both inherit `A`. Default Perl MRO is
DEPTH-FIRST: `D → B → A → C`, so `D->hello` finds `A::hello` even though
`C::hello` exists — `A` shadows `C` (line 22 vs line 18's declaration).
`D3` is the same shape under `use mro 'c3'` (line 25): linearization
`D3 → B → C → A`, so `C::hello` wins. Same diamond, two answers.

## Why naive Go conversion changes semantics
Go has no inheritance; embedding gives compile errors on ambiguous selectors
rather than a linearization, and embedding two structs that embed the same base
duplicates the base. Any conversion must PRECOMPUTE the per-class MRO and
flatten method dispatch. Getting the algorithm wrong (assuming C3 everywhere,
or leftmost-only) silently picks the other diamond answer — exactly the
difference between the `dfs hello:` and `c3 hello:` output lines.

## What the converter should do
- Category: **approximate** (flattening):
  - Compute each class's linearization with the SAME algorithm Perl would use
    for that class (default DFS unless `use mro 'c3'` is in effect for it).
  - Emit one Go method set per class with each method resolved to the winning
    implementation; record the full linearization in the report.
  - `SUPER::` calls (not in this file) resolve against the declaring package's
    order and must be resolved statically the same way.
  - Runtime `@ISA` mutation makes the precomputation invalid: if the converter
    sees any write to `@ISA` outside top-level constant assignments, it must
    refuse the file.
- Forbidden: converting the diamond with Go embedding and letting the Go
  compiler's ambiguity rules decide, or applying one MRO to both classes.

## Ideal diagnostic (word for word)
> input.pl:22: warning P2G-W308: class D uses multiple inheritance (B, C).
> Perl resolves methods depth-first: D, B, A, C — so D->hello is A::hello,
> shadowing C::hello. Methods were flattened into D using that order. Class D3
> (input.pl:26) uses C3: D3, B, C, A — D3->hello is C::hello. Verify the
> shadowing of C::hello in D is intended; it is a common source of surprise.

## What a human should do instead
Collapse the hierarchy: composition plus explicit delegation, or a single
inheritance chain. Where the diamond was intentional, write the winning method
explicitly in the subclass.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `who:      B` (leftmost), `dfs hello: hello from A`
(DFS shadowing), `c3 hello:  hello from C` (C3). The middle line is the trap:
"obviously" C should win, and under C3 it does — but D uses DFS.
