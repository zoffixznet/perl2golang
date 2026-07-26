---
id: if-err-nil-rhythm
title: The if err != nil rhythm, and why silence still compiles
tags: [idiom, errors, discipline]
perl_triggers: [open-or-die, autodie, unchecked-call, errno-check]
severity: info
prerequisites: [errors-are-values, var-vs-short-declaration]
---

Every fallible call in Go is followed by three lines: `if err != nil { return ..., err-with-context }`. Newcomers read this as boilerplate; the design intent is that the failure path is *written*, *visible*, and *decided* at every step — each check is a place where you chose to propagate, enrich, retry, or absorb. The dangerous flip side: Go's compiler forces you to *receive* an error into a variable but never forces you to *check* it, so a disciplined-looking program can be silently swallowing failures — `use warnings` has no compiler equivalent here, and linters plus culture stand in for it.

## The Perl you know

```perl
open my $fh, '<', $path  or die "can't open $path: $!";
my $data = do_thing()    or die "do_thing failed";
$obj->save;                       # did it fail? who knows — silent unless autodie
```

Perl's checked-call story is opt-in per call (`or die`), per file (`use autodie`), and inconsistent across codebases; unchecked failures are routine.

## The Go you write

The rhythm in its natural habitat — early returns, context added at each level, run as shown:

```go
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func readPort(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read port file: %w", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse port: %w", err)
	}
	return port, nil
}

func main() {
	port, err := readPort("/no/such/port.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
	fmt.Println(port)
}
```

```
fatal: read port file: open /no/such/port.txt: no such file or directory
exit status 1
```

And the hazard — both of these compile and run without a murmur:

```go
package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	n, _ := strconv.Atoi("not a number") // discard is explicit and greppable
	fmt.Println(n)

	f, _ := os.Open("/no/such/file") // this compiles...
	fmt.Println(f == nil)            // ...and leaves a nil *os.File time bomb
}
```

```
0
true
```

## The mismatch

Structural habits to adopt. Keep the happy path at zero indentation: check-and-return-early, never `if err == nil { ... } else { ... }` ladders — Go code reads as a straight line of successes with failure exits branching off. Reuse one `err` variable down a function (`var-vs-short-declaration` explains why `:=` then `=` matters; a shadowed `err` inside an `if` block is the classic silent bug). Add context at each hop with `%w` (`error-wrapping`) so the final message reads like a story: `read port file: open ...: no such file or directory` above is three functions' worth of context concatenated — Go's replacement for a stack trace in well-run codebases. On discipline: `_ = f()` or `n, _ := f()` is the *visible* opt-out, acceptable where failure is truly irrelevant (`fmt.Println` to stdout); an invisible opt-out — calling a `func() error` as a bare statement — also compiles, which is why serious projects run `errcheck` or `staticcheck` in CI (`vet-and-staticcheck`) to restore what the compiler declines to enforce. Perl's `$!` has a shadow here: some low-level failures are `syscall.Errno` under the hood, but you never read a global — the errno arrives inside the returned error, where it belongs.

Further reading: https://go.dev/blog/error-handling-and-go and https://go.dev/wiki/CodeReviewComments#indent-error-flow
