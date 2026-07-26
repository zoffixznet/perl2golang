---
id: os-exec
title: os/exec runs programs, and there is no shell in the way
tags: [idiom, processes, stdlib]
perl_triggers: [system, backticks, qx, exec, open-pipe, ipc-run, ipc-open3, child-error]
severity: info
prerequisites: [errors-are-values, io-reader-writer]
---

`system("ls $dir")` and `` `ls $dir` `` hand a *string* to `/bin/sh`, which is why quoting bugs and shell injection are a permanent Perl hazard. `exec.Command("ls", dir)` takes an argv *list* and calls the program directly — no shell, no word splitting, no glob expansion, and no injection risk from `dir` no matter what it contains. That one difference reshapes every ported command invocation: the parts of your command line that the shell used to do (pipes, redirection, `*` globs, `&&`) are now either Go code or an explicit `sh -c`, and choosing the latter deliberately re-opens the door you just closed.

## The Perl you know

```perl
system("gzip -9 $file") == 0 or die "gzip failed: $?";
my $out = `git rev-parse HEAD`;      # captures stdout, $? holds the status
chomp $out;
open my $ph, '-|', 'find', '.', '-name', '*.log' or die;   # list form: no shell
while (<$ph>) { ... }
```

`$?` is the wait status, so the exit code is `$? >> 8`, and forgetting the shift is a rite of passage.

## The Go you write

```go
package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func main() {
	// Backticks: capture stdout. Arguments are a list, never a shell string.
	out, err := exec.Command("echo", "hello   $HOME").Output()
	fmt.Printf("%q %v\n", strings.TrimSpace(string(out)), err)

	// Non-zero exit is an error value carrying the exit code.
	err = exec.Command("false").Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		fmt.Println("exit code:", ee.ExitCode())
	}

	// stderr is separate: Output() fills ExitError.Stderr, CombinedOutput merges.
	both, err := exec.Command("sh", "-c", "echo to-stdout; echo to-stderr >&2").CombinedOutput()
	fmt.Printf("%q %v\n", string(both), err)

	// A missing program is a different error, reported before anything runs.
	_, err = exec.Command("no-such-program-here").Output()
	fmt.Println(errors.Is(err, exec.ErrNotFound))
}
```

```
"hello   $HOME" <nil>
exit code: 1
"to-stdout\nto-stderr\n" <nil>
true
```

Note the first line: `$HOME` came back untouched and the doubled spaces survived, because nothing interpreted the argument. Streaming instead of capturing is a matter of assigning the process's pipes — an `io.Writer` for output, an `io.Reader` for input:

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	cmd := exec.Command("tr", "a-z", "A-Z")
	cmd.Stdin = strings.NewReader("one\ntwo\n") // feed it a string as stdin
	cmd.Stderr = os.Stderr                      // let its errors through to ours

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	sc := bufio.NewScanner(pipe)
	for sc.Scan() {
		fmt.Println("read:", sc.Text())
	}
	if err := cmd.Wait(); err != nil { // Wait after draining the pipe, always
		fmt.Fprintln(os.Stderr, err)
	}
}
```

```
read: ONE
read: TWO
```

## The mismatch

The translations, with their traps. `system(LIST)` → `cmd.Run()`, and success is `err == nil`; there is no `$?` to shift, and a non-zero exit arrives as an `*exec.ExitError` whose `ExitCode()` you read (`error-wrapping` explains `errors.As`). Backticks → `cmd.Output()`, which returns only stdout as `[]byte`; stderr lands in `ExitError.Stderr` on failure, and `CombinedOutput()` is the merged-stream variant. `open '-|'` → `StdoutPipe()` plus `Start()`, read to EOF, then `Wait()` — calling `Wait` before you finish reading closes the pipe under you, the one ordering rule that bites everybody once. `exec` (the replace-this-process builtin) is `syscall.Exec` and is almost never what you want. Shell features are opt-in and explicit: to get a pipeline or a redirect, either build it in Go (two `exec.Cmd`s with one's `Stdout` connected to the other's `StdinPipe`, or simply `cmd.Stdout = file`) or run `exec.Command("sh", "-c", script)` and accept that every value you interpolate into `script` is now an injection risk — pass data through arguments or the environment instead. Two more differences worth knowing on day one: `cmd.Env` replaces the entire environment when you set it (append to `os.Environ()` rather than assigning a bare slice), and `exec.CommandContext(ctx, ...)` kills the child when the context expires, which is the well-behaved version of the `alarm`-plus-`kill` dance (`context-cancellation`).

Further reading: https://pkg.go.dev/os/exec
