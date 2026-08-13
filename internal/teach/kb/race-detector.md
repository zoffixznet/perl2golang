---
id: race-detector
title: The race detector, and why correct output proves nothing
tags: [gotcha, concurrency, race, tooling]
perl_triggers: [shared-counter, threads-shared, shared-accumulator]
severity: warning
prerequisites: [waitgroup-and-mutex]
---

A data race - two goroutines touching the same memory, at least one writing, with no synchronisation - is undefined behaviour in Go: not "last write wins" but "the compiler and CPU may do anything, including things no interleaving explains". The evil part, demonstrated below with real output: racy code frequently *produces the correct answer anyway*, so testing cannot clear you. Go's answer is mechanical, not heroic: `go run -race` (or `go test -race`) instruments the binary and reports races that actually occur at runtime, with both stack traces. A Perl background is a specific liability here - fifteen years of process isolation means your instincts have never once flagged a shared write as dangerous.

## The Perl you know

```perl
# fork gave you this safety implicitly: children can't race on $count.
my $count = 0;
for (1..2) {
    next if fork;           # child gets a COPY of $count
    $count += 1000;         # modifies its own copy; parent unaffected
    exit;
}
```

The bug class does not exist, which is why you have no reflex for it.

## The Go you write

This program is *wrong on purpose* - yet observe the plain run:

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	counter := 0
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				counter++ // unsynchronised shared write: a data race
			}
		}()
	}
	wg.Wait()
	fmt.Println(counter)
}
```

```console
$ go run racedemo.go
2000
```

Correct output, broken program. Now with the detector:

```console
$ go run -race racedemo.go
==================
WARNING: DATA RACE
Read at 0x00c000018178 by goroutine 9:
  main.main.func1()
      /.../racedemo.go:16 +0x99

Previous write at 0x00c000018178 by goroutine 8:
  main.main.func1()
      /.../racedemo.go:16 +0xab

Goroutine 9 (running) created at:
  main.main()
      /.../racedemo.go:13 +0x78

Goroutine 8 (finished) created at:
  main.main()
      /.../racedemo.go:13 +0x78
==================
2000
Found 1 data race(s)
exit status 66
```

Both access sites, both goroutines' birthplaces, and a nonzero exit code for CI. The fix is `waitgroup-and-mutex`'s mutex (or `atomic.Int64` for a lone counter).

## The mismatch

Operating knowledge: the detector only reports races that *happen during that run* - it proves presence, never absence - so its home is `go test -race` in CI, where your test suite's concurrency exercises the code paths on every commit; the 2-20x slowdown and extra memory are why it is not the default build. What counts as a race is broader than Perl intuition suggests: concurrent map access panics outright even when reads and writes touch different keys (maps are not internally locked); appending to a shared slice races on the header (`slices-not-arrays`); lazily initialising a shared field ("check if nil, then fill") is the classic racy singleton, fixed by `sync.Once`. What does *not* race: data published through a channel send (the send happens-before the receive - `channels-and-select`), values written before `go` starts the goroutine, and anything guarded consistently by one mutex. The porting rule distilled: every package-level variable and every captured local that more than one goroutine can see is guilty until synchronised - and since `fork` never taught you to run that audit, let `-race` run it for you, always, in CI.

Further reading: https://go.dev/blog/race-detector and https://go.dev/doc/articles/race_detector
