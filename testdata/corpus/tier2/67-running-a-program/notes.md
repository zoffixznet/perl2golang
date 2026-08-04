# 67 - running another program

## What this exercises
The last big thing a sysadmin script does. Every command here is a POSIX shell
builtin, so the transcript is the same anywhere the file can run at all.

The shapes, and what separates them:

- `system(LIST)`, which starts a program directly with no shell in the way,
  and answers with the wait status. Both the returned value and `$?` are read,
  and both are shifted right by eight to get the exit code.
- backticks in scalar context, which capture standard output as one string,
  and in list context, which give one string per line with the newline still
  on the end.
- an argument with a space and a semicolon in it, passed as its own word, so
  that nothing re-splits it. This is the line that would go wrong if a
  conversion turned a list into a command string.
- `open '-|'`, which starts a program and reads its output through a handle,
  and `open '|-'`, which writes to its input. Both are `exec.Cmd` pipes, and
  both have to wait for the child at the end.

## Perl constructs
- `system LIST` for its value and for `$?`, `$? >> 8` and `$? & 127`
- backticks in scalar and list context, `chomp` over the list
- `open my $fh, '-|', PROG, ARGS` with `or die`, read in a `while`
- `open my $fh, '|-', PROG, ARGS`, `print {$fh}`, `close`

## Go concepts a converter must teach
- `exec.Command(name, args...)` takes an argument list and starts the program
  directly. A command written as one string goes to a shell, which splits it,
  expands globs in it, and runs whatever a semicolon introduces; that door is
  closed by default in Go and has to be opened by name.
- The wait status comes back from the call rather than from a global. Shifting
  it by eight is only there to keep the code that reads it working.
- `StdoutPipe` plus `Start`, read to the end, then `Wait`. Calling `Wait`
  early closes the pipe under the reader.
- A handle that reads and writes like a file lets the loop around it stay the
  loop it would be over a file, which is what `io.Reader` is for.
