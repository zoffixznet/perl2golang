# 16-goto-sub: `goto &sub` frame replacement

Group: **B - convertible only with an approximation that changes semantics**

## Construct
`goto &real_work` (line 15) REPLACES the current call frame: `real_work` runs
with the trampoline's `@_`, returns directly to the trampoline's caller, and
`caller()` inside it cannot see that `trampoline` ever existed. The contrast
call through `normal_call` shows an extra frame (`main::normal_call`).

## Why naive Go conversion changes semantics
Go has no tail-call frame replacement. The approximation
`return real_work(args...)` preserves the VALUE and control flow but not the
stack: anything observing the stack - `caller()`, stack traces in panics,
`Carp::croak`-style blame assignment, and AUTOLOAD idioms that use `goto` to
hide themselves - behaves differently.

## What the converter should do
- Category: **approximate**. Translate `goto &real_work` as
  `return real_work(args...)` (forwarding the current argument values), and:
  - If the enclosing file contains any use of `caller`, emit a diagnostic
    NAMING the interaction (this file does: `caller(1)` at input.pl:8), because
    the approximation is observably wrong there.
  - If the file contains no stack introspection, a single report entry (per
    goto) noting the tail-call approximation is enough.
- Forbidden: translating it as a Go `goto` label (type error / nonsense) or
  dropping the statement.

## Ideal diagnostic (word for word)
> input.pl:15: warning P2G-W306: 'goto &real_work' replaces the current stack
> frame; converted to an ordinary tail call 'return real_work(...)'. This
> program inspects the stack (caller at input.pl:8), and WILL observe the
> difference: Perl reports 'top-level' for the goto path, the converted Go
> behaves like a normal nested call. Review the caller()-dependent logic.

## What a human should do instead
If `goto &sub` was only a tail call, keep the plain call and delete the
stack-sensitivity. If the hidden frame mattered (croak blame, AUTOLOAD
self-effacing), restructure so blame/identity is passed explicitly as an
argument.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0). The discriminator pair:
`real_work(1 2): frame above me: top-level` (goto path) vs
`real_work(1 2): frame above me: main::normal_call` (normal path). The naive
tail-call conversion prints `main::trampoline`-equivalent for the first -
i.e. the two lines stop differing in the way Perl's do.
