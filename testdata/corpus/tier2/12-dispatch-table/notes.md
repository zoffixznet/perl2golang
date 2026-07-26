# 12 - dispatch table and callbacks

## What this exercises
The hash-of-code-refs pattern Perl uses instead of a long `if/elsif` chain,
plus callbacks passed into a generic recursive walker.

## Perl constructs
- `my %commands = (set => sub {...}, get => sub {...}, ...);` a dispatch table
  whose values are anonymous subs closing over `%store`
- lookup-then-call with a fallback: `my $h = $commands{$verb}; return "..."
  unless $h; return $h->(@args);`
- `delete` returning the deleted value
- `split ' ', $line` to peel a verb off its arguments
- callbacks passed as named arguments: `walk($doc, array => sub {...}, ...)`
- `$cb{leaf}->($_[0]) if $cb{leaf};` optional-callback guard
- recursion dispatching on `ref $data`
- building code refs in a loop from a list of `[name, sub]` pairs
- a function that *returns* a code ref, with `||` supplying a default sub:
  `return $ops{$n} || sub { 'n/a' }`
- immediate invocation of the returned ref: `op_for('mul')->(6, 7)`
- closures capturing loop-local `$name`/`$fn` from the destructured pair

## Go concepts a converter must teach
- `map[string]func(...)` is a direct analogue, but Perl's subs are all
  variadic-and-untyped. The converter must unify the signatures: here every
  handler takes `...string` and returns `string`, which is inferable from the
  call site, but that will not always be true.
- The `%cb` optional-callback pattern becomes a struct of function fields with
  nil checks, or an interface with default methods.
- `walk` dispatches on `ref $data` - a Go type switch over
  `[]any` / `map[string]any` / default.
- Closures capturing `%store` mean the handlers mutate shared state; in Go this
  should become methods on a receiver rather than free closures over a package
  variable.
- `$ops{$n} || sub { 'n/a' }` relies on a missing map entry being false. In Go
  a missing func value is `nil`, so it is `if f == nil { f = fallback }` - the
  `||` shortcut does not survive.
- Perl's `sort keys %commands` for a stable listing is required in Go too.
