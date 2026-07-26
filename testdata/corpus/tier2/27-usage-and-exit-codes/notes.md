# 27 - usage messages and exit codes

## What this exercises
A script that validates its arguments and exits with a distinct status per
failure mode - the shape expected by cron, `make`, and shell `&&` chains.
**This entry exits non-zero (65).**

**cmd:** `--mode=avg --strict 10 20 abc 30` &nbsp; **expected_exit:** `65`

## Perl constructs
- `use constant { EX_OK => 0, EX_USAGE => 2, ... };` - compile-time constants
  used as bareword function calls
- a `usage` sub taking an optional message
- `$0` in the usage line
- a manual `for my $arg (@ARGV)` classification loop
- `exit EX_USAGE;` / `exit EX_NOINPUT;` / `exit EX_DATAERR;` / `exit EX_OK;`
  from several points in the program
- a dispatch table `%modes` of code refs, one per aggregation mode, with
  `exists $modes{$mode}` validation
- `grep { !/^-?\d+(?:\.\d+)?$/ } @counts` to find bad input, then the inverse
  `grep` to filter it out
- `$modes{$mode}->(@counts)` invoking the selected reducer
- a final conditional `exit` after the "done" line

## Go concepts a converter must teach
- `use constant` is a `const` block - one of the few one-to-one mappings, but
  Perl constants are subs, so they can be exported and called with parentheses.
- **`exit` in Perl runs `END` blocks and object destructors; `os.Exit` in Go
  runs neither, and skips `defer`.** Any script that both `exit`s and relies on
  cleanup needs restructuring: the usual answer is `func run() int` called from
  `main` as `os.Exit(run())`.
- Multiple exit points scattered through a script convert badly to Go's
  `defer`-based cleanup; a converter should hoist them into returned status
  codes.
- Perl's `exit 65` truncates to 8 bits like Go's; values above 255 wrap in both.
- Usage output going to stdout vs stderr is a policy decision the converter
  must preserve exactly, because tests capture streams separately.
- The `%modes` dispatch table is `map[string]func([]float64) float64`; note
  that `avg` divides by `len`, so Go needs a float conversion and a
  zero-length guard that Perl handles by dying.
