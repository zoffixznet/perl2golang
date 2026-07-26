# 04-cron-lockwrap

**Domain:** sysadmin glue. A cron job wrapper with lock files, stale-lock
detection against an injectable clock (`--now`), a registry of runnable
jobs, and sysexits-style exit codes. The cmd runs three jobs in `--dry-run`
(read-only) mode: one behind a stale lock that fails its check (exit 1),
one that finds a service down (exit 2), and one skipped behind a fresh
lock (EX_TEMPFAIL). Expected exit is 75 -- the worst of the three.

## Constructs exercised
- **Dispatch table of code refs**: `%JOBS` maps names to
  `{desc, code => \&sub}` records; invocation via `$job->{code}->()`.
- Jobs return `(exit_code, @report_lines)` -- a list-context return with a
  variable-length tail, unpacked as `my ($rc, @report) = ...`.
- Lock files parsed into hashes with `/^(\w+)=(\S+)$/`, defensive
  validation, `return undef` for junk locks.
- Fake-clock pattern (`$opt{now} // time`) that keeps output deterministic.
- Exit-code max-accumulation (`bump`), sysexits constants (75, 64, 66).
- `gmtime`/`sprintf` ISO-8601 formatting with an array slice `@g[3,2,1,0]`.
- Conditional side effects guarded by `--dry-run` (lock write/unlink paths
  exist in code but are not exercised, so the fixture tree is never
  mutated between runs).

## Conversion challenges
- The `(exit, @lines)` multi-return: Go wants `(int, []string)`, but the
  Perl call sites build the list incrementally (`return @bad ? (1, $hdr,
  @bad) : (0, $hdr)`) -- list flattening has no direct Go analogue.
- `%JOBS` values are heterogeneous records holding closures: in Go this is
  a `map[string]Job` with a func field -- one of the places where the
  converter must invent a named struct + function type.
- `$job->{code}->()` and `\&job_backup_verify` reference-taking; Go method
  values/function values differ syntactically but map cleanly -- a good
  teaching moment.
- `grep { $_ eq '--list' } @requested` scanning argv after Getopt::Long
  has already consumed options -- argument handling is split across two
  mechanisms.
- `$$` (PID) and `time` appear only on non-exercised paths; a converter
  must still compile them without leaking nondeterminism into the tested
  path.
