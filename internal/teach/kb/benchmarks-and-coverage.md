---
id: benchmarks-and-coverage
title: Benchmarks, coverage, fuzzing, and profiles are all one command
tags: [idiom, testing, tooling, performance]
perl_triggers: [benchmark, timethese, cmpthese, devel-nytprof, devel-cover, devel-size, profiling]
severity: info
prerequisites: [table-driven-tests]
---

Everything Perl needs a module for, Go builds into `go test`. Benchmarks are functions in the same `_test.go` files as your tests, coverage is a flag, fuzzing is another kind of test function, and CPU and memory profiles come out of the same command and feed a viewer that ships with the toolchain. Nothing to install and nothing to configure.

The thing to internalise is that a benchmark is not "run it once and time it". The framework runs your loop repeatedly, increasing the iteration count until the measurement is long enough to mean something, and reports the per-operation cost. Your job is to write the loop and to make sure the compiler cannot delete the work you are trying to measure.

## The Perl you know

```perl
use Benchmark qw(cmpthese);

cmpthese(-2, {
    regex => sub { $addr =~ /^([^:]*):(\d+)$/ },
    split => sub { split /:/, $addr, 2 },
});

# and, separately:
#   perl -d:NYTProf script.pl && nytprofhtml
#   cover -test
```

## The Go you write

The code under test, `hostport.go`:

```go
package hostport

import "strings"

// Split splits "host:port" into its two halves.
func Split(addr string) (host, port string) {
	h, p, _ := strings.Cut(addr, ":")
	return h, p
}
```

And the benchmark, in `hostport_test.go` beside it, taking `*testing.B` and looping with `b.Loop()`:

```go
package hostport

import "testing"

// Package-level sinks: assigning to them stops the compiler deleting the
// call because nothing uses the result.
var sinkHost, sinkPort string

func BenchmarkSplit(b *testing.B) {
	for b.Loop() {
		sinkHost, sinkPort = Split("db.internal:5432")
	}
}

// Benchmarks take subtests too, which is how you compare inputs.
func BenchmarkSplitTable(b *testing.B) {
	for _, addr := range []string{"db.internal:5432", ":8080"} {
		b.Run(addr, func(b *testing.B) {
			for b.Loop() {
				sinkHost, sinkPort = Split(addr)
			}
		})
	}
}
```

```console
$ go test -bench=. -benchmem -run '^$'
goos: linux
goarch: amd64
pkg: example/hostport
cpu: <your processor, as the runtime reports it>
BenchmarkSplit-24                            	400731218	         2.790 ns/op	       0 B/op	       0 allocs/op
BenchmarkSplitTable/db.internal:5432-24      	356062050	         2.908 ns/op	       0 B/op	       0 allocs/op
BenchmarkSplitTable/:8080-24                 	390024504	         2.813 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	example/hostport	3.257s
```

Read the columns right to left. `allocs/op` is the number the experienced Go programmer looks at first, because allocation is usually what separates a fast function from a slow one; `B/op` is how much it allocated; `ns/op` is the time; and the large integer before them is how many iterations were needed to measure it. `-run '^$'` matches no test, so only benchmarks run. The `-24` suffix is `GOMAXPROCS`.

Coverage reports what the *tests* exercised, so the numbers below come from the tested package in `table-driven-tests`, with an untested `Join` function added to it. It is a flag on the same command, and `-func` says which functions nobody reached:

```console
$ go test -cover ./...
ok  	example/hostport	0.002s	coverage: 70.0% of statements

$ go test -coverprofile=c.out ./... && go tool cover -func=c.out
hostport.go:11:	Split		100.0%
hostport.go:24:	Join		0.0%
total:		(statements)	70.0%

$ go tool cover -html=c.out
```

The last command opens a browser view with every covered line in green and every uncovered one in red, which is the fastest way to find the branch nobody tests. `-covermode=atomic` is the variant to use when the code under test runs goroutines.

## The mismatch

Two habits from `Benchmark.pm` to unlearn. The first is timing a single call: `time.Since(start)` around one invocation measures scheduler noise as much as your code, which is why `testing` decides the iteration count itself. The second is trusting a number the compiler was free to optimise away. If a benchmark body computes something and discards it, the compiler may delete the whole call and report an impossibly fast result. Assigning to a package-level variable is the standard defence; `b.Loop()` (Go 1.24 and later) also keeps the loop body alive and is the form to write in new code. The older `for i := 0; i < b.N; i++` is what you will see in existing code and is still correct. Setup that should not be timed goes before the loop with `b.ResetTimer()` after it, or in a `b.StopTimer()`/`b.StartTimer()` pair.

Comparing two implementations is not a built-in like `cmpthese`: you write both benchmarks, run them, and compare with `benchstat` (`go install golang.org/x/perf/cmd/benchstat@latest`), which runs the statistics and tells you whether the difference is real. Getting a stable measurement usually means `-count=10` and a machine that is not doing anything else.

The other tools in the same family are worth knowing about before you need them. `-race` builds with the data race detector and is the single highest-value flag in the toolchain for concurrent code (`race-detector`). A `FuzzXxx(f *testing.F)` function with `f.Add` seeds and `f.Fuzz(func(t *testing.T, s string) {...})` runs as an ordinary test against its seeds, and as a fuzzer under `go test -fuzz=Fuzz`, saving any crashing input into `testdata/` as a permanent regression test. `-cpuprofile=cpu.out` and `-memprofile=mem.out` write profiles that `go tool pprof` reads, with `top`, `list <func>`, and `web` for a call graph. All of it is in the box, none of it needs a module from anywhere, and none of it changes how your program is built for production.

Further reading: https://pkg.go.dev/testing#hdr-Benchmarks
