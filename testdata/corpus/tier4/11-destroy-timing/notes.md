# 11-destroy-timing: DESTROY with load-bearing deterministic timing

Group: **A - genuinely impossible without an interpreter** (impossible to
reproduce implicitly; convertible only by making destruction explicit)

## Construct
`Guard::DESTROY` (line 10) prints `release ...`. Perl's refcounting guarantees
it runs at an exact, deterministic instant: the closing brace of the block
(after `working under lock-A`, before `between`) and the `undef $g2` statement.
Real code uses this for lock guards, temp-file cleanup, and flush-on-drop.

## Why it resists conversion to Go
Go finalizers (`runtime.SetFinalizer`) run at an unspecified time, possibly
never. There is no destructor tied to scope exit. Any conversion that maps
DESTROY to a finalizer silently changes WHEN (and whether) the release happens -
for a lock guard, that is a correctness bug, not a style issue.

## What the converter should do
- Category: **todo** with a mechanical transformation offer:
  - Where a blessed object's lifetime is a single lexical scope (the `{ my $g =
    ...; }` block), convert DESTROY to an explicit method and emit
    `defer g.Destroy()` at the ACQUISITION point. `defer` runs at function exit,
    not block exit - if the object's scope is an inner block (as here), the
    converter must either introduce a function literal wrapping the block or
    place an explicit `g.Destroy()` call at the block's end, and must say which
    it did.
  - `undef $g2` becomes an explicit `g2.Destroy()` call.
- Every DESTROY conversion needs a report entry: refcount-exact timing was
  replaced by explicit calls at the statically determined death points; if a
  death point cannot be determined statically (object stored in a structure,
  returned, or aliased), the converter must refuse that object's conversion
  rather than attach a finalizer.
- Forbidden: `runtime.SetFinalizer`, or dropping DESTROY.

## Ideal diagnostic (word for word)
> input.pl:10: warning P2G-W302: package Guard defines DESTROY; Perl runs it at
> exact refcount-zero points. Converted to an explicit Destroy() call at the two
> statically known death points (end of block at input.pl:17, 'undef $g2' at
> input.pl:22). Verify no other code paths need the release; Go will not run it
> automatically.

## What a human should do instead
Adopt the explicit-cleanup idiom: constructor returns the object AND schedules
`defer g.Close()`, exactly as Go code does natively.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0). The ORDER is the specification:
`release lock-A` strictly between `working under lock-A` and `between`;
`release lock-B` strictly before `end`. A finalizer-based conversion cannot
guarantee either line appears at all.
