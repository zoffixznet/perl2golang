# 04-monkey-patch: runtime replacement of another package's method

Group: **A - genuinely impossible without an interpreter**

## Construct
`*Greeter::hello = sub { ... }` (line 20) overwrites `Greeter`'s `hello` method
at runtime, wrapping the original (captured as a code ref on line 19). The
already-constructed object `$g` sees the new behaviour on its next call.

## Why it resists conversion to Go
Go method sets are fixed at compile time; there is no per-package dispatch table
to mutate. Even modelling methods as function-typed fields fails in general: the
patch targets a DIFFERENT package's table, every existing caller and object must
observe the change instantly, and whether the patch runs at all can depend on
runtime control flow.

## What the converter should do
- Category: **refuse-statement**. Replace the assignment to
  `*Greeter::hello` with a panicking stub, convert the rest, and report it.
- A narrowing is NOT recommended here (unlike 03-glob-aliasing): silently
  converting method replacement requires rewriting the target package's dispatch
  into mutable function values, which changes the shape of ALL generated code for
  that package. If the converter chooses to support it, it must say so per-method
  in the report and demonstrate `expected_stdout` exactly.
- The critical honesty requirement: the generated `Greeter.hello` must NOT keep
  returning `"hello"` as if the patch never happened.

## Ideal diagnostic (word for word)
> input.pl:20: error P2G-E104: assignment to '*Greeter::hello' replaces another
> package's method at run time. Go method sets are immutable. Replaced with a
> panicking stub; calls to Greeter::hello after this point would have used the
> replacement. Rewrite using explicit delegation, a callback field, or an
> interface with two implementations.

## What a human should do instead
Give `Greeter` an explicit extension point: a function-typed field or an
interface (`type Greeting interface { Hello() string }`) with a decorating
implementation that upcases and appends `!`. Wrapping-by-symbol-table becomes
wrapping-by-composition.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `before: hello` then `after:  HELLO!` - the same
object, the same call site, two behaviours. Any conversion that prints `hello`
twice is silently wrong.
