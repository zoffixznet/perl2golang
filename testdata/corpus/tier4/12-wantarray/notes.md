# 12-wantarray: context-polymorphic returns

Group: **A - genuinely impossible without an interpreter** (in general; this
file is the statically-analyzable special case)

## Construct
`pieces()` returns a LIST of fragments in list context and a COUNT in scalar
context (line 9). `ctx()` distinguishes list/scalar/VOID (line 16; `wantarray`
is undef in void context, line 20). `wrapper()` (the killer) contains no
`wantarray` itself but transparently inherits its caller's context and passes it
into `pieces()`.

## Why it resists conversion to Go
Go functions have ONE return shape. The Perl sub's behaviour depends on an
implicit hidden argument - the caller's context - which propagates through
plain-looking wrappers. In general the context at a call site is not statically
knowable (calls through code refs, `&$f()`, method dispatch, `return f()` from
subs whose own context is unknown), so a single translation of the sub does not
exist.

## What the converter should do
- Category: **refuse-statement** for any sub whose body reaches `wantarray`,
  UNLESS the converter can prove the full call graph and specialize:
  - Generate `piecesList(s) []string` and `piecesScalar(s) int` variants.
  - Rewrite each call site to the variant matching its statically known
    context, including THROUGH wrappers (`wrapper` must become two variants
    too, or be inlined).
  - Void-context calls map to the scalar variant with the result discarded only
    if the body treats undef/scalar identically; here `ctx()` distinguishes
    VOID, so a third variant (or an explicit ctx argument) is required.
- If any call site's context cannot be proven, refuse the sub with a diagnostic
  naming that call site - never default to one context silently.

## Ideal diagnostic (word for word)
> input.pl:9: error P2G-E109: sub 'pieces' uses wantarray to return a list or a
> count depending on the caller's context. Go functions have one return type.
> All 4 call sites in this file have statically known context, so the sub was
> split into piecesList/piecesScalar and call sites rewritten (see report). If
> you add a call through a code ref, this conversion becomes invalid.

(For a non-specializing converter: same first two sentences, then "Replaced the
sub with a panicking stub; split it into two subs by hand.")

## What a human should do instead
Split the API explicitly: `pieces_list()` and `pieces_count()`; make wrappers
name which one they mean. Delete void-context cleverness.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `list=a b c count=3`,
`ctx: LIST SCALAR (void call ran too)`, `wrapped: n=4 w=x y z w`. The
`wrapped:` line is the specialization test: context must flow THROUGH wrapper().
