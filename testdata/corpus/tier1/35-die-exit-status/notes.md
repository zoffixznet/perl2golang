# 35-die-exit-status

## What this exercises
`die` terminating the program. The message ends with `"\n"`, so Perl does
**not** append `" at SCRIPT line N."`. The message goes to STDERR (so it is not
part of `expected_stdout`) and the process exits with status **255**.

This entry carries an `allow_stderr` marker because the stderr output is
intentional.

## Perl constructs
- `die "message\n"` -- trailing newline suppresses the location suffix
- output printed before the `die` is still flushed to stdout
- an uncaught `die` (no `eval`) exiting with 255

## Go concepts a converter must teach
- The exit status for an uncaught `die` is 255 (from `errno`/`$!` when set,
  otherwise 255), **not** 1. A converter that lowers `die` to
  `log.Fatal` gets exit status 1 and a timestamp prefix -- both wrong.
  The faithful lowering is
  `fmt.Fprint(os.Stderr, msg); os.Exit(255)`.
- The trailing-newline rule is a real behavioural difference: `die "x"` emits
  `x at prog.pl line 12.` while `die "x\n"` emits just `x`. The converter must
  inspect the message literal (or check at runtime for a dynamic message) and
  synthesise the location suffix when it is absent.
- `die` is catchable with `eval { }` and unwinds like an exception; Go's
  `os.Exit` is not catchable. If any enclosing scope has an `eval`, the correct
  lowering is `panic`/`recover` (or an error return), and only a provably
  uncaught `die` may become `os.Exit`.
- Anything buffered on stdout must be flushed before exiting, same caveat as
  entry 34.
