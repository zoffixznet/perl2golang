# 15-local-dynamic: `local` dynamic scoping of a package variable

Group: **B - convertible only with an approximation that changes semantics**

## Construct
`local $mode = "debug"` (line 15) temporarily rebinds the GLOBAL `$mode` for the
dynamic extent of `with_debug`, including subs it calls indirectly (`deeper` →
`describe`). The old value is restored on scope exit - including exits via `die`
(line 22, caught by `eval {}`).

## Why naive Go conversion changes semantics
Lexical translation gets it exactly backwards: a converter that makes `local`
into a new local variable produces `describe()` reading the untouched global -
Perl would print `mode=debug`, the wrong Go prints `mode=normal`. The
restore-on-die behaviour additionally requires panic-safe restoration.

## What the converter should do
- Category: **shim** - this one is mechanically translatable:
  ```go
  saved := mode; mode = "debug"; defer func() { mode = saved }()
  ```
  `defer` gives function-exit timing AND panic-path restoration, matching
  Perl's die-path restore. When the `local` is in an inner block rather than at
  function top, the converter must wrap the block in a func literal or restore
  explicitly at the block end and on every early exit, and say which it chose.
- Caveats the report must state: (1) goroutines are not dynamic scopes - if the
  converted program spawns any, the shimmed global is shared, which Perl never
  had to worry about; (2) `local` on hash/array ELEMENTS needs elementwise
  save/restore.
- Forbidden: converting `local $mode = ...` to `mode := ...` (new lexical) or
  to a plain global assignment without restore.

## Ideal diagnostic (word for word)
> input.pl:15: warning P2G-W305: 'local $mode' is dynamic scoping: callees
> (describe, input.pl:8) observe the temporary value, and the original is
> restored even if this sub dies. Converted to save/defer-restore on the global
> 'mode'. Note: unlike Perl, any goroutine started inside this extent shares
> the modified global.

## What a human should do instead
Thread the value as a parameter (`describe(mode)`), or restructure as an
explicit context struct passed down the call chain - the Go-native shape.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `mode=normal`, `mode=debug`, `mode=normal`,
`mode=normal (restored even after die)`. Line 2 proves callee visibility; line 4
proves die-path restoration.
