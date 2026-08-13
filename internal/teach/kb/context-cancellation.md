---
id: context-cancellation
title: context carries deadlines and cancellation everywhere
tags: [idiom, concurrency, context, timeouts]
perl_triggers: [alarm, sig-alrm, timeout-eval, lwp-timeout, kill-child-pid]
severity: info
prerequisites: [channels-and-select, error-wrapping]
---

Perl timeouts are `alarm` plus a signal handler plus an `eval` fence - global, process-wide, famously fragile. Go threads a value called a `context.Context` through call chains instead: it carries a deadline or a cancellation signal, every blocking operation along the chain watches it, and when the deadline passes or a caller cancels, everything downstream unwinds promptly with a consistent error. You cannot avoid learning this even for simple ports, because the standard library demands it - database queries, HTTP requests, and command execution all take a `ctx` as their first argument, and `ctx context.Context` leading a signature is as much a fixture of Go as `$self = shift` was of Perl.

## The Perl you know

```perl
my $result = eval {
    local $SIG{ALRM} = sub { die "timeout\n" };
    alarm 5;
    my $r = slow_query($dbh);
    alarm 0;
    $r;
};
die $@ if $@ && $@ ne "timeout\n";
```

One alarm per process, signals interacting with XS code unpredictably, cleanup easy to get wrong - you know the drill and its scars.

## The Go you write

Compiled and run as shown - same function, one call given 50ms, the other 1s, against a 150ms "query":

```go
package main

import (
	"context"
	"fmt"
	"time"
)

func fetch(ctx context.Context, host string) (string, error) {
	select {
	case <-time.After(150 * time.Millisecond): // pretend network call
		return "data from " + host, nil
	case <-ctx.Done():
		return "", fmt.Errorf("fetch %s: %w", host, ctx.Err())
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	res, err := fetch(ctx, "db1")
	fmt.Printf("%q %v\n", res, err)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()
	res, err = fetch(ctx2, "db2")
	fmt.Printf("%q %v\n", res, err)
}
```

```
"" fetch db1: context deadline exceeded
"data from db2" <nil>
```

The moving parts: `context.Background()` is the root; `WithTimeout`/`WithDeadline`/`WithCancel` derive children; `ctx.Done()` is a channel that closes on cancellation, built for `select` (`channels-and-select`); `ctx.Err()` explains why (`context.DeadlineExceeded` or `context.Canceled`, both matchable with `errors.Is` through wrapping - `error-wrapping`). Always `defer cancel()` - it releases the timer and subtree even on the success path; `go vet` reminds you.

## The mismatch

What makes this better than `alarm`, concretely: contexts nest (a request-scoped 5s context can hand a 1s sub-context to one query - try that with one process-global alarm); cancellation is *cooperative and race-free* (a closed channel observed by `select`, no signals interrupting who-knows-what); and it composes across libraries because everyone agreed on the one type - `http.NewRequestWithContext`, `db.QueryContext`, `exec.CommandContext` (which kills the subprocess on expiry - the `kill $pid` dance, done right; `os-exec`). The conventions you must follow for your code to compose too: `ctx` is the *first parameter*, never stored in a struct field; pass it down every call that might block; check `ctx.Err()` or select on `ctx.Done()` inside your own long loops, because nothing preempts a goroutine that refuses to look - cancellation you ignore simply does not happen, which is also how goroutine leaks end: the abandoned worker sees `Done` and returns (`goroutines-not-fork`). One misuse to skip on day one: `context.WithValue` as a grab-bag for passing ordinary parameters - it is for request-scoped metadata (trace IDs), and stuffing your actual arguments into it is the Go equivalent of communicating via `%ENV`.

Further reading: https://go.dev/blog/context and https://pkg.go.dev/context
