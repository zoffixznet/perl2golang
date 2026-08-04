# 68 - the process half of running a program, which does not convert yet

## What this exercises
The neighbour of entry 67. There everything was about the command line; here
everything is about the process.

- **The status a pipe close leaves.** Closing a pipe waits for the child and
  puts its status in `$?`, so `close $fh; $? >> 8` is how a script finds out
  whether the program it read from succeeded. The conversion closes the pipe
  and waits, but nothing carries the status back to `$?`.
- **Keeping the two streams apart.** `2>&1` and `2>/dev/null` inside a
  backtick command are shell redirections, so they convert for free once the
  command goes through a shell. Capturing standard error *separately*, which
  is what `IPC::Open3` is for, does not.
- **`fork`.** Go's runtime is multi-threaded from the first line, so forking
  is not something the standard library offers. A script that forks to do two
  things at once wants a goroutine; one that forks to run a program wants
  `exec.Command`. Neither is a mechanical translation.
- **`$^X`.** The path to the interpreter running the script. A converted
  program has no interpreter, and `os.Executable()` names the wrong thing, so
  this is refused rather than guessed at.

## What goes wrong today
The close status reads 0 where Perl says 7, and the program stops at the
`fork`, which is refused. The refusal is load-bearing: the script dies at the
`die` that follows it, which is the right place to stop.

## Go concepts a converter must teach
- `cmd.Wait()` returns the error the status is read out of, and a small type
  that wraps the pipe and the wait can hand that status back to whoever asks.
- Redirections belong to the shell. Building the same thing in Go means
  setting `cmd.Stdout` and `cmd.Stderr` to what you want, which is more code
  and considerably clearer.
- There is no `fork`. That is a deliberate absence, not a gap.
