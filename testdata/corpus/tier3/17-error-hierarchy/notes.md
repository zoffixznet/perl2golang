# 17-error-hierarchy

Exception objects as blessed hashrefs with a class hierarchy
(`App::Error` <- `IO` <- `Timeout`, plus `Parse`), rethrow-with-context,
nested eval recovery, and a DESTROY/`local $@` guard demo.

## Constructs exercised
- `die $object` with blessed-hashref exceptions; `throw` class method
- error hierarchy via `@ISA`, behavior override (`is_retryable`),
  `Timeout` inheriting retryability from `IO` two levels down
- catch-classify with an `isa` ternary ladder (order matters: Timeout
  before IO)
- string exceptions *promoted* to objects (`ref $err && $err->isa(...)`
  discrimination, `chomp(my $msg = $err)`)
- rethrow after annotation: `die $err->with_context(...)` from inside the
  catch path of an inner eval, caught again by the outer eval
- nested eval where the inner failure is handled and a fallback value
  becomes the outer eval's result
- `DESTROY` running during exception unwind; `local $@` inside DESTROY;
  a class-level log buffer drained by a class method
- dispatch table `%jobs` of coderefs; multi-value return `($result, $@)`

## Conversion challenges
- Perl exceptions are dynamically typed values thrown through the stack;
  Go needs error types + errors.As chains, or panic/recover -- the
  classify-by-isa ladder maps to errors.As with ordering preserved
- string-vs-object duality of `$@` (checked with `ref`) has no direct Go
  analogue; converter must normalize to a single error interface
- rethrow-with-context == error wrapping (`fmt.Errorf("%w")` or a custom
  chain), and `describe`'s `a <- b` context trail must reproduce exactly
- DESTROY-during-unwind == deferred cleanup running while an error is in
  flight; Go's defer naturally avoids the clobber hazard -- notes in the
  source explain the perl-version history so the converter's tests don't
  encode the pre-5.14 bug
- `is_retryable` virtual dispatch across the hierarchy

## Go teaching opportunities
- sentinel vs typed errors, errors.Is/As, wrapping chains, defer for
  guaranteed cleanup, table-driven error-path tests
