# 34-exit-status

## What this exercises
An explicit non-zero `exit`. The script is invoked with a single argument, so
it takes the usage branch, prints to **stdout** (not stderr) and exits 2.
`expected_exit` is 2.

Note that everything printed before the `exit` still appears: Perl flushes
STDOUT on normal exit.

## Perl constructs
- `exit EXPR`
- `exit` terminating the program from inside a conditional, without a return
  value being threaded back to any caller
- an array in numeric comparison (`@args < 2` puts `@args` in scalar context)

## Go concepts a converter must teach
- `exit N` is `os.Exit(N)`. **`os.Exit` does not run deferred functions and
  does not flush buffered writers.** If the converter wrapped stdout in a
  `bufio.Writer`, a direct `os.Exit` lowering loses all pending output --
  exactly the bytes this entry is checking. The lowering must flush first.
- Perl's `exit` also runs `END` blocks and object destructors; Go's `os.Exit`
  runs nothing. A converter that emits cleanup via `defer` has to restructure
  `exit` into a return-code-propagating `main`.
- `@args < 2` is scalar context on an array: `len(args) < 2`.
- Exit status is masked to 8 bits by the OS in both languages, so `exit 256`
  becomes 0 -- worth a note but not exercised here.
