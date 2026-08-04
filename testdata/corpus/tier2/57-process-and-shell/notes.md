# 57 - running another program, which does not convert yet

## What this exercises
The last big thing a sysadmin script does that has no rule yet: capturing a
command's output, checking its exit status, and talking to it through a pipe.
The only external command is the interpreter itself, so the transcript is the
same anywhere this file can run at all.

This is the recorded target. Every construct in it is refused, and the
refusals now degrade properly: the handles are declared, everything below them
still compiles, and the program fails at the line that cannot work rather than
refusing to build.

## Perl constructs
- backticks read for one value and for a list
- `system` with a list, and the exit status in the return value and in `$?`
- `$? >> 8` and `$? & 127`, the two halves of a wait status
- the list form of `system` and of a pipe open, which involves no shell and
  therefore no quoting hazard
- `open my $rd, '-|', ...` and `open my $wr, '|-', ...`

## Go concepts a converter must teach
- `exec.Command(name, args...)` never involves a shell, so there is no
  quoting hazard and no `2>&1`: redirection is assigning to `cmd.Stdout` and
  `cmd.Stderr`. Backticks always used a shell, which is where the difference
  in behaviour lives.
- `cmd.Output()` captures standard output; `cmd.CombinedOutput()` is the
  `2>&1` form; `cmd.Run()` waits and reports an error.
- The exit status is `exitErr.ExitCode()` from an `*exec.ExitError`, not a
  number shifted by eight. Go took the encoding apart so nobody has to
  remember the shift.
- `cmd.StdoutPipe()` and `cmd.StdinPipe()` are the pipe opens, and both need
  `cmd.Start()` before and `cmd.Wait()` after, with the write pipe closed
  before the wait or the child never sees end of input.
- A command that outlives its caller wants a `context.Context` with a
  deadline, which Perl had no equivalent of at all.
