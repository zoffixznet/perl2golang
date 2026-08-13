---
id: waitgroup-and-mutex
title: sync.WaitGroup and sync.Mutex - joining and guarding
tags: [idiom, concurrency, sync, mutex]
perl_triggers: [wait-all-children, waitpid-loop, flock, thread-lock, threads-shared]
severity: info
prerequisites: [goroutines-not-fork]
---

Because goroutines share memory (`goroutines-not-fork`), you need the two primitives `fork` never made you learn properly: `sync.WaitGroup` to wait for a batch of goroutines to finish - the `wait_all_children` of the goroutine world - and `sync.Mutex` to make a compound operation on shared data atomic. The trap for a Perl programmer is not the API, which is tiny; it is the *habit* of unguarded shared state, which forked processes made harmless and goroutines make into silent corruption - even a bare `counter++` is a read-modify-write that two goroutines can interleave (`race-detector` proves it).

## The Perl you know

```perl
# fork world: no shared state, so no locks - the OS was your mutex.
$pm->wait_all_children;

# threads world (rarely used):
use threads::shared;
my $count :shared = 0;
{ lock($count); $count++; }
```

## The Go you write

One thousand goroutines incrementing one counter, correctly - compiled and run as shown:

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	var (
		mu    sync.Mutex
		count int
		wg    sync.WaitGroup
	)

	for i := 0; i < 1000; i++ {
		wg.Go(func() { // Go 1.25+: bundles Add(1) and Done for you
			mu.Lock()
			defer mu.Unlock()
			count++
		})
	}
	wg.Wait()
	fmt.Println(count)
}
```

```
1000
```

`wg.Go(f)` is recent sugar; the long-standing spelling you will see everywhere is:

```go
func spawn(wg *sync.WaitGroup, work func()) {
	wg.Add(1) // BEFORE starting the goroutine, never inside it
	go func() {
		defer wg.Done()
		work()
	}()
}
```

`Add` must happen before `go` - inside the goroutine it races with `Wait`. The mutex pattern is equally fixed: `mu.Lock()` immediately followed by `defer mu.Unlock()` (`defer-timing`), so early returns and panics cannot leave the lock held.

## The mismatch

Design points with no fork-world equivalent. Zero values work: `var mu sync.Mutex` and `var wg sync.WaitGroup` are ready to use - no constructor (`static-types-and-zero-values`) - but they must never be *copied* once used, which is why structs containing a mutex take pointer receivers (`methods-and-receivers`; `go vet` flags violations). The idiomatic packaging is a struct with the mutex physically above the fields it guards, and *unexported* so callers cannot touch the data unguarded - the compile-time privacy of `packages-and-exported-names` doing concurrency work. Mutexes are not re-entrant: a function holding the lock calling another function that locks the same mutex deadlocks *itself* - structure as an exported locking wrapper around an unexported assumes-locked implementation. `sync.RWMutex` (many readers or one writer) is the read-heavy upgrade; `sync.Once` runs initialisation exactly once no matter how many goroutines arrive - the thread-safe `state $x = init()`. For a lone counter, `sync/atomic`'s `atomic.Int64` is lighter than a mutex; for everything compound, mutex. Choosing between mutex and channels (`channels-and-select`): guard *state* with a mutex, transfer *data and ownership* with channels - porting advice, since most Perl shared state is a hash serving as a cache or tally, which in Go is a struct with a mutex, or the ready-made `sync.Map` only in its documented niche (mostly-read caches with disjoint key sets).

Further reading: https://pkg.go.dev/sync
