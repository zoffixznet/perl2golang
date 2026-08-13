---
id: goroutines-not-fork
title: Goroutines are not fork, and main waits for nobody
tags: [orientation, concurrency, goroutines]
perl_triggers: [fork, waitpid, parallel-forkmanager, threads, background-system, sigchld]
severity: info
prerequisites: [closures-and-loop-capture]
---

`go f()` starts a goroutine: a function running concurrently *inside your process*, sharing all memory with everything else - the opposite of `fork`'s share-nothing copied process. Two consequences arrive immediately. First, the isolation `fork` gave you for free is gone: concurrent access to shared variables is now your bug to prevent (`race-detector`). Second, `main` waits for nobody - when `main` returns, the program exits, instantly abandoning every running goroutine, with no SIGCHLD, no zombie, no trace. And a cultural note up front: most Perl scripts that would be ported never needed concurrency and still do not - reaching for goroutines because they are cheap is how converters manufacture bugs.

## The Perl you know

```perl
use Parallel::ForkManager;
my $pm = Parallel::ForkManager->new(4);
for my $host (@hosts) {
    $pm->start and next;      # child: separate memory, crash-isolated
    ping($host);
    $pm->finish;
}
$pm->wait_all_children;       # explicit reaping, or zombies
```

Children cannot corrupt each other's data; getting results *back* is the hard part (pipes, files, serialisation).

## The Go you write

```go
package main

import "fmt"

func main() {
	go fmt.Println("from a goroutine")
	fmt.Println("main is done")
	// main returns here - the program exits, waiting for nobody
}
```

```
main is done
```

Run it as often as you like: the goroutine's line never appears, because `main` wins the race to exit every time and the runtime does not wait for anybody.

```console
$ go run racy.go
main is done
$ go run racy.go
main is done
$ go run racy.go
main is done
```

Waiting is explicit, via `sync.WaitGroup` (`waitgroup-and-mutex`) - and goroutines are cheap enough to spend like Perl spends hash lookups; run as shown:

```go
package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 100000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
		}()
	}
	wg.Wait()
	fmt.Println("spawned and joined 100000 goroutines in", time.Since(start))
}
```

```text
spawned and joined 100000 goroutines in 19.686224ms
```

A hundred thousand of them in under twenty milliseconds - `fork` could not do a hundred in that time. They are multiplexed onto OS threads by the runtime; you never manage threads.

## The mismatch

Rebuild the mental model on three axes. Memory: `fork` copies, goroutines share - so passing data *in* is free (closures capture it - beware capturing loop variables pre-1.22, `closures-and-loop-capture`) and getting results *out* is easy (channels - `channels-and-select`), the exact inverse of `fork`'s trade-off. Failure: a died child is a reaped exit status; a panicking goroutine *kills the whole program*, unrecoverably from outside (`panic-and-recover`) - goroutines are not a crash-isolation boundary, ever. Lifecycle: there is no `wait_all_children` built in and no way to kill a goroutine from outside; goroutines end by returning, and you *ask* them to stop via `context-cancellation` - a goroutine blocked forever on a channel nobody writes is a leak, Go's version of the zombie. When porting: `Parallel::ForkManager` loops become the worker pool in `channels-and-select`; backgrounded `system("cmd &")` stays `os/exec` (`os-exec`) - a subprocess is still sometimes the right tool, including for crash isolation. And the restraint rule: a sequential script ported sequentially is correct; add goroutines only where the Perl was already parallel or measurably I/O-bound over many independent targets.

Further reading: https://go.dev/tour/concurrency/1 and https://go.dev/doc/effective_go#goroutines
