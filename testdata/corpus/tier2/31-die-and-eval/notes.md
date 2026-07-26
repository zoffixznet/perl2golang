# 31 - die, eval and $@

## What this exercises
Perl's exception mechanism: `die` to throw, `eval {}` to catch, `$@` to
inspect, plus warning capture, nested eval with rethrow, and the `local $@`
discipline that stops cleanup code from destroying an error in flight.

## Perl constructs
- `local $SIG{__WARN__} = sub { print "WARN: $_[0]" };` - **redirecting
  warnings**, so `warn` output lands on stdout instead of stderr
- `die "message\n"` - the trailing newline suppresses the " at FILE line N"
  suffix, which is why the expected output is stable
- `my $value = eval { ... }; if ($@) { ... }` - the basic try/catch pair
- `if (my $err = $@) { ... }` - capturing in the condition
- **runtime errors are caught too**: `eval { 100 / 0 }` sets `$@` to
  "Illegal division by zero at ... line N.", stripped with `s/ at .*//s`
- nested eval: an inner failure caught, annotated with context, and rethrown
- `eval { ... } || $default` - the "or fall back" idiom
- `warn` continuing execution (unlike `die`)
- **`local $@` inside a cleanup sub**, so `eval` in the cleanup does not clobber
  the caller's `$@`
- the fact that a *successful* `eval` clears `$@`, so it must be captured
  immediately
- `my ($k, $v) = $line =~ /.../ or die "malformed line $.: $line\n";` - a match
  in list context used as a boolean guard
- `$.` in an error message

## Go concepts a converter must teach
- **`die`/`eval` is not `panic`/`recover`.** In Perl, `die` is the normal way to
  report a failure and `eval` the normal way to handle it; in Go the normal way
  is a returned `error`. A converter should turn `die` into `return
  fmt.Errorf(...)` and `eval {}` into `if err != nil`, reserving panic/recover
  for cases where the control flow genuinely cannot be restructured.
- That restructuring is invasive: every sub between the `die` and the `eval`
  gains an `error` return and every call site gains a check.
- `$@` is a single global, cleared by the next successful `eval`. The
  capture-immediately rule and the `local $@` idiom both exist because of that
  global; Go's per-value errors make both unnecessary, and a converter should
  say so rather than emulating them.
- **Runtime errors** (division by zero, calling a method on undef) are catchable
  in Perl. In Go, integer division by zero is a panic and float division by
  zero is `+Inf` - neither matches. This needs an explicit guard.
- `warn` is `log.Printf` to stderr; `$SIG{__WARN__}` is swapping the logger's
  output - `log.SetOutput(os.Stdout)`.
- The die-message format (`"...\n"` vs no newline) determines whether file/line
  is appended; a converter must preserve the exact string to match output.
