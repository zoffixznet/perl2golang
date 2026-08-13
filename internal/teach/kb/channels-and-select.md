---
id: channels-and-select
title: Channels, select, and the worker pool
tags: [idiom, concurrency, channels, select, worker-pool]
perl_triggers: [pipe, socketpair, thread-queue, ipc-open2, ipc-open3, parallel-forkmanager-results]
severity: info
prerequisites: [goroutines-not-fork, comma-ok-idiom]
---

Channels are typed, in-process pipes with synchronisation built in - the structured replacement for every pipe, socketpair, and results-file dance you have used to get data out of forked children. The part that surprises: an *unbuffered* channel is not a queue at all but a rendezvous - the send blocks until a receive is ready, transferring the value and synchronising the two goroutines in one act. Get blocking wrong with no partner ever coming, and the runtime tells you bluntly at runtime. `select` is the multiplexer over channel operations - `IO::Select`'s role, but for channels - and it is how timeouts and cancellation exist.

## The Perl you know

```perl
pipe my $r, my $w;
if (fork) { close $w; my $line = <$r>; }      # bytes, buffering, EOF checks
else      { close $r; print {$w} "result\n"; exit }

# or Thread::Queue under threads: $q->enqueue($job); my $j = $q->dequeue;
```

Typed data means serialising by hand; synchronisation means blocking reads and `waitpid`.

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

func main() {
	// Unbuffered: a send blocks until someone receives - a rendezvous.
	ch := make(chan string)
	go func() { ch <- "result" }()
	fmt.Println(<-ch)

	// Buffered: sends don't block until the buffer fills.
	buf := make(chan int, 2)
	buf <- 1
	buf <- 2
	fmt.Println(<-buf, <-buf)

	// close + range: how a producer says "no more".
	jobs := make(chan int, 3)
	for i := 1; i <= 3; i++ {
		jobs <- i * 10
	}
	close(jobs)
	for j := range jobs {
		fmt.Print(j, " ")
	}
	fmt.Println()

	// comma-ok on receive: a closed channel yields zero values.
	v, ok := <-jobs
	fmt.Println(v, ok)
}
```

```
result
1 2
10 20 30 
0 false
```

Blocking with no possible partner is a runtime-detected crash - run as shown:

```go-fails
ch := make(chan int)
ch <- 1 // nobody will ever receive: the runtime detects the deadlock
```

```
fatal error: all goroutines are asleep - deadlock!

goroutine 1 [chan send]:
main.main()
	/.../deadlock.go:5 +0x28
exit status 2
```

`select` waits on several operations and takes whichever is ready first - the timeout idiom, run as shown (the query needs 200ms, the deadline is 50ms):

```go
package main

import (
	"fmt"
	"time"
)

// slowQuery returns immediately with a channel the answer will arrive on. The
// buffer of 1 matters: it lets the abandoned goroutine finish its send and
// exit even after nobody is listening.
func slowQuery() <-chan string {
	out := make(chan string, 1)
	go func() {
		time.Sleep(200 * time.Millisecond)
		out <- "42 rows"
	}()
	return out
}

func main() {
	out := slowQuery()
	select {
	case res := <-out:
		fmt.Println("got:", res)
	case <-time.After(50 * time.Millisecond):
		fmt.Println("timed out; moving on")
	}
}
```

```
timed out; moving on
```

The canonical composition - a worker pool, `Parallel::ForkManager`'s job in ~30 lines, run as shown:

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	jobs := make(chan int)
	results := make(chan int)

	var wg sync.WaitGroup
	for w := 0; w < 3; w++ { // three workers: a forking pool without forks
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				results <- j * j
			}
		}()
	}

	go func() {
		for i := 1; i <= 5; i++ {
			jobs <- i
		}
		close(jobs) // workers' range loops end when the channel drains
	}()

	go func() {
		wg.Wait()
		close(results) // close results only after every worker is done
	}()

	sum := 0
	for r := range results {
		sum += r
	}
	fmt.Println("sum of squares:", sum)
}
```

```
sum of squares: 55
```

## The mismatch

Rules that have no pipe-world analogue: close is a *broadcast* ("no more values"), not a resource release - you rarely need to close channels except to end a `range`, only the *sender* may close, closing twice panics, and sending on a closed channel panics; the comma-ok receive distinguishes "zero value sent" from "channel closed", the same two-state pattern as map lookups (`comma-ok-idiom`). Buffering is not a performance knob to sprinkle: unbuffered channels give you synchronisation guarantees; a buffer size is a semantic statement about decoupling (and `make(chan T, 1)` in the timeout example lets the abandoned `slowQuery` complete its send instead of leaking - a real pattern, not an accident). `select` with a `default` branch makes operations non-blocking; `select` over a `ctx.Done()` case is how cancellation reaches you (`context-cancellation`). And the proverb that organises all of it: *share memory by communicating* - send the data itself through the channel and let one goroutine own it at a time, rather than sharing a structure and locking around it (`waitgroup-and-mutex` covers when a mutex is still the simpler tool).

Further reading: https://go.dev/doc/effective_go#channels and https://go.dev/blog/pipelines
