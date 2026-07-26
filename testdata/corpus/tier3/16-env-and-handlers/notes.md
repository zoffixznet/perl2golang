# 16-env-and-handlers

Deployment preflight: env-driven config with pinned defaults, `local %ENV`
scoping, `%SIG` handler installation and classification, warning capture.

## Constructs exercised
- pinned clock: `$ENV{DEPLOY_EPOCH} // 1717243200`, `gmtime` list return
  (`$gm[6]`, `$gm[7]`), `POSIX::strftime` with an explicit Z suffix
- `%ENV` reads with `//` defaults; env var set, then `local $ENV{...}`
  override in a block, NESTED `local` + `delete` inside that, each layer
  restored on scope exit (three-deep dynamic scoping demo)
- `$SIG{__WARN__}` handler collecting `warn` messages into an array (so
  stderr stays clean by design)
- signal handler installation without delivery: coderef, `'IGNORE'`,
  `'DEFAULT'`, and unset -- classified via chained ternary over
  `ref $h eq 'CODE'` etc.
- `sort grep { /re/ } keys %ENV` scan
- PATH-style split with empty-segment skip and `$seen{$d}++` dedupe in one
  `next if` condition
- multi-branch ternary verdict, computed `exit()` code

## Conversion challenges
- `local %ENV` dynamic scoping with *nested* save/restore has no Go
  equivalent: os.Setenv is process-global, so the converter must emit
  explicit save/defer-restore stacks (and get the restore ORDER right --
  the expected output encodes it)
- `$SIG{__WARN__}` is not a real signal; it's a hook on `warn` -- maps to
  a logger seam, not os/signal
- `%SIG` values being polymorphic (coderef | 'IGNORE' | 'DEFAULT' | undef)
  vs Go's signal.Notify/Ignore/Reset API split
- `gmtime` list slots (weekday 6 = Saturday, yday 0-based) vs Go's
  time.Time methods (Weekday enum, YearDay 1-based) -- off-by-one traps
  the expected output will catch
- deciding exit code from data (`exit 2` path exists but is not taken)

## Go teaching opportunities
- time.Unix().UTC() formatting; os.LookupEnv defaulting; signal.NotifyContext
  as the modern pattern; capturing "warnings" via an injected io.Writer
