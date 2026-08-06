---
id: defer-timing
title: defer runs at function exit, but evaluates its arguments now
tags: [gotcha, functions, defer, cleanup]
perl_triggers: [scope-guard, end-block, local, close-filehandle, destroy, eval-cleanup]
severity: warning
prerequisites: [closures-and-loop-capture]
---

`defer` schedules a call to run when the *enclosing function* returns — Go's replacement for `Scope::Guard`, `DESTROY`-driven cleanup, and the discipline of remembering `close $fh` on every exit path. Two timing rules generate all its surprises: the deferred call's *arguments are evaluated immediately* at the `defer` statement (only the call itself is postponed), and multiple defers run LIFO. The classic bug this produces: `defer` inside a loop does not run per-iteration — it queues up until the whole function ends, which for "open file per loop iteration" means running out of file descriptors in a way no Perl idiom ever prepared you for.

## The Perl you know

```perl
use Scope::Guard qw(guard);
sub process {
    open my $fh, '<', $path or die;
    my $g = guard { close $fh };   # runs when $g leaves scope — BLOCK scope
    ...
}
# or more commonly: lexical $fh auto-closes on scope exit via refcount
```

Perl cleanup is scope-based and immediate: refcounts hit zero at block exit, `local` restores at block exit.

## The Go you write

Compiled and run as shown — trace the order and the two `x` values:

```go
package main

import "fmt"

func main() {
	x := 1
	defer fmt.Println("deferred arg sees x =", x)                  // arg evaluated NOW: 1
	defer func() { fmt.Println("deferred closure sees x =", x) }() // reads x at exit
	x = 99
	fmt.Println("end of main, x =", x)

	for i := 0; i < 3; i++ {
		defer fmt.Println("cleanup", i)
	}
}
```

```
end of main, x = 99
cleanup 2
cleanup 1
cleanup 0
deferred closure sees x = 99
deferred arg sees x = 1
```

Everything after the first output line runs during function exit, newest-first. The first `defer` froze `x=1` as an argument; the closure form read `x` late and saw 99 — choose between them deliberately. The canonical daily use:

```go
func dump(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close() // guaranteed on every return path, panics included

	_, err = io.Copy(os.Stdout, f)
	return err
}
```

## The mismatch

Porting a guard object, a class whose `DESTROY` releases something, follows a three-way rule that falls straight out of the granularity difference. The destructor becomes an ordinary method. Then: for an object born and dying inside a function-shaped body (a sub, or an `eval` block, which arrives in Go as a function literal), `defer g.destroy()` on the line after construction is exact, and it keeps the one property refcounting gave you for free, that a `die` unwinding past the object still releases it, because deferred calls run during a panic. For an object scoped to an inner block, an `if` branch or a loop body, `defer` fires too late; the call is written explicitly at the closing brace, which is the instant Perl fired it. And `undef $g`, Perl's way of choosing the instant by hand, becomes the call followed by clearing the variable. What has no mechanical port is an object whose references outlive the block: that lifetime belongs to the reference count, and in Go it has to be redesigned, not translated.

The granularity difference is the trap: Perl cleanup is *block*-scoped, `defer` is *function*-scoped. Any Perl loop that opens-processes-closes per iteration must, in Go, either hoist the body into a named function or wrap it in an immediately-called closure (`func() { ... defer f.Close() ... }()`) so the defer fires each pass — transliterating the flat loop leaks handles until function exit. Second difference: nothing is implicit — Go has no refcount cleanup, no `DESTROY`; an `os.File` you forget to close stays open until the GC maybe-finalises it much later, so `defer f.Close()` on the line after a successful `Open` is reflex-level mandatory. Third: deferred functions run even during a panic, which is why `defer` is the *only* place `recover` works (`panic-and-recover`) and why mutex code is written `mu.Lock(); defer mu.Unlock()` (`waitgroup-and-mutex`). Last, a subtlety for later: a deferred closure can modify *named* return values after the `return` statement executed — the legitimate use of named returns from `multiple-return-values`, common in wrap-the-error patterns.

Further reading: https://go.dev/blog/defer-panic-and-recover
