---
id: os-exec
title: os/exec runs programs, and there is no shell in the way
tags: [idiom, processes, stdlib]
perl_triggers: [system, backticks, qx, exec, open-pipe, ipc-run, ipc-open3, child-error]
severity: info
prerequisites: [errors-are-values, io-reader-writer]
---

`system("ls $dir")` and `` `ls $dir` `` hand a *string* to `/bin/sh`, which is why quoting bugs and shell injection are a permanent Perl hazard. `exec.Command("ls", dir)` takes an argv *list* and calls the program directly - no shell, no word splitting, no glob expansion, and no injection risk from `dir` no matter what it contains. That one difference reshapes every ported command invocation: the parts of your command line that the shell used to do (pipes, redirection, `*` globs, `&&`) are now either Go code or an explicit `sh -c`, and choosing the latter deliberately re-opens the door you just closed.

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

Note the first line: `$HOME` came back untouched and the doubled spaces survived, because nothing interpreted the argument. Streaming instead of capturing is a matter of assigning the process's pipes - an `io.Writer` for output, an `io.Reader` for input:

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

## Where $? went, and the one ordering rule

Perl leaves the wait status of the last child in `$?`, a global that the next child overwrites, and every script that cares reads it as `$? >> 8`. Go hands the status back from the call that produced it, which means there is nothing to overwrite and nothing to read too late. Getting the number back into the shape the old code expects is four lines, written once:

```go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
)

// waitStatus encodes what a child did the way $? did: the exit code in the
// high byte, so that shifting right by eight gives the code back.
func waitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode() << 8
	}
	return -1
}

func main() {
	// system(LIST): run it, keep the status.
	status := waitStatus(exec.Command("sh", "-c", "exit 42").Run())
	fmt.Println("decoded:", status>>8, "signal:", status&127)

	// open '-|': start the program, read it to the end, then wait. The order
	// is the one rule that bites everybody once.
	cmd := exec.Command("sh", "-c", `printf "10\n20\n30\n"`)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("pipe:", err)
		return
	}
	if err := cmd.Start(); err != nil {
		fmt.Println("start:", err)
		return
	}
	sum := 0
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		var n int
		fmt.Sscan(scanner.Text(), &n)
		sum += n
	}
	fmt.Println("sum:", sum, "close status:", waitStatus(cmd.Wait())>>8)
}
```

```
decoded: 42 signal: 0
sum: 60 close status: 0
```

Three things about that. `errors.As` rather than a type assertion, because the error may be wrapped by the time it reaches you. The shift by eight is not Go's idea of anything: it is the shape the old code reads, kept so the lines after it do not have to change, and new code should just use `ExitCode()`. And a child killed by a signal is the one case the encoding cannot reproduce, because the standard library reports the code as `-1` without saying which signal.

The ordering rule in the second half is worth saying twice. `Start`, then read the pipe to the end, then `Wait`. Calling `Wait` first closes the pipe under the reader, and calling it never leaves a process behind. Wrapping the pipe and the wait in one small type with `Read` and `Close` methods is the tidy way to keep that order: `Close` does the wait, the code that reads the handle looks exactly like code reading a file, and the rule stops being something anyone has to remember.

There is no `fork`. Go's runtime is multi-threaded from the first line, and forking a process with threads in it is not safe to do anything with except `exec` immediately, so the standard library does not offer it. A Perl script that forked to do two things at once wants a goroutine, and one that forked to run a program wants `exec.Command`.

## The mismatch

The translations, with their traps. `system(LIST)` → `cmd.Run()`, and success is `err == nil`; there is no `$?` to shift, and a non-zero exit arrives as an `*exec.ExitError` whose `ExitCode()` you read (`error-wrapping` explains `errors.As`). Backticks → `cmd.Output()`, which returns only stdout as `[]byte`; stderr lands in `ExitError.Stderr` on failure, and `CombinedOutput()` is the merged-stream variant. `open '-|'` → `StdoutPipe()` plus `Start()`, read to EOF, then `Wait()` - calling `Wait` before you finish reading closes the pipe under you, the one ordering rule that bites everybody once. `exec` (the replace-this-process builtin) is `syscall.Exec` and is almost never what you want. Shell features are opt-in and explicit: to get a pipeline or a redirect, either build it in Go (two `exec.Cmd`s with one's `Stdout` connected to the other's `StdinPipe`, or simply `cmd.Stdout = file`) or run `exec.Command("sh", "-c", script)` and accept that every value you interpolate into `script` is now an injection risk - pass data through arguments or the environment instead. Two more differences worth knowing on day one: `cmd.Env` replaces the entire environment when you set it (append to `os.Environ()` rather than assigning a bare slice), and `exec.CommandContext(ctx, ...)` kills the child when the context expires, which is the well-behaved version of the `alarm`-plus-`kill` dance (`context-cancellation`).

Further reading: https://pkg.go.dev/os/exec
