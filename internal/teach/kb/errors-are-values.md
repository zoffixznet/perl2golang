---
id: errors-are-values
title: Errors are return values, not exceptions
tags: [orientation, errors]
perl_triggers: [die, eval-block, eval-error-var, try-tiny, try-catch, or-die, croak, carp, errno, stat, chmod, rename, unlink, symlink]
severity: info
prerequisites: [multiple-return-values]
---

Go has no exceptions in the working sense: nothing you call will unwind your stack on failure (barring genuine panics - `panic-and-recover`). A function that can fail says so in its type - `func f() (T, error)` - and failure travels back through return values like any other data, handled with `if`, passed around, stored in slices, compared. Your `die`/`eval`/`$@` architecture does not translate construct-for-construct; it dissolves into a different control-flow style where the failure path is ordinary visible code at every call site.

## The Perl you know

```perl
my $ok = eval {
    die "insufficient funds\n";
    1;
};
say "caught: $@" if !$ok;     # caught: insufficient funds
say "still running";
```

`die` throws anything (strings, objects), `eval` catches everything, `$@` is fragile shared state (hence `Try::Tiny`), and a forgotten `eval` means a crashed program. And a Perl function's failure behaviour is invisible in its signature - you learn it from docs or outages.

## The Go you write

Compiled and run as shown:

```go
package main

import (
	"errors"
	"fmt"
)

func withdraw(balance, amount int) (int, error) {
	if amount > balance {
		return balance, errors.New("insufficient funds")
	}
	return balance - amount, nil
}

func main() {
	bal, err := withdraw(100, 250)
	if err != nil {
		fmt.Println("declined:", err)
	}
	fmt.Println(bal)

	// error is just an interface value; nothing was thrown or caught.
	var e error = errors.New("plain value")
	fmt.Println(e == nil, e.Error())
}
```

```
declined: insufficient funds
100
false plain value
```

`error` is a one-method interface - `Error() string` - defined in the language. `errors.New` makes an opaque one; `fmt.Errorf` formats one; your own types can be errors (`sentinel-and-custom-errors`). Success is the nil error, which is why `nil-vs-undef` and `typed-nil-interface` matter so much here.

## The mismatch

The honest translation table: `die $msg` inside a function → `return fmt.Errorf(...)`; `or die` after a call → `if err != nil { return ... }` (`if-err-nil-rhythm`); `eval {}` + `if ($@)` → nothing, because the error *arrived in a variable* and there is no unwinding to intercept; `Try::Tiny`'s `try/catch/finally` → plain `if` for catch and `defer` for finally (`defer-timing`); `croak` (blame the caller) → error *wrapping* so context accumulates instead of a stack trace (`error-wrapping`). What you lose: automatic propagation - an unhandled Perl `die` climbs until something cares, while an unchecked Go error goes exactly nowhere; forgetting to check is the Go equivalent of forgetting `eval`, with inverted failure modes (Perl: crash you notice; Go: silence you do not - `if-err-nil-rhythm` covers the discipline). What you gain: failure is in the signature, so every caller can see *that* a call can fail and *must* type a decision about it; error handling code is guaranteed-local, greppable, and testable as data. Resist the urge to build a try/catch out of panic/recover - the community will read it as not knowing the language, because that is what it is.

## The calls that answered with a count, and now answer with a reason

Half the standard library a script actually uses reports failure the older way: `unlink` gives back how many files it removed, `rename` and `symlink` give true or false, `chmod` gives a count, and every one of them leaves the reason in `$!` for whoever thinks to look before the next call overwrites it. Go's counterparts all return an `error` and nothing else.

That is a straight upgrade, and it changes the shape of the line:

```perl
rename($old, $new) or die "cannot rename: $!";
unlink(@stale) == @stale or warn "some files stayed";
```

```go
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

func tidy(old, newName string, stale []string) error {
	if err := os.Rename(old, newName); err != nil {
		return fmt.Errorf("renaming %s: %w", old, err)
	}
	for _, p := range stale {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			fmt.Printf("could not remove %s: %v\n", p, err)
		}
	}
	return nil
}

func main() {
	err := tidy("/no/such/file", "/tmp/whatever", []string{"/no/such/other"})
	fmt.Println(errors.Is(err, fs.ErrNotExist), err != nil)
}
```

```
true true
```

Three things to notice. The reason travels with the failure rather than sitting in a global that the next system call clobbers, which is the bug `$!` invites and the reason experienced Perl reads it on the very next line. `errors.Is(err, fs.ErrNotExist)` is how you say "this particular failure is fine", which a count could never express: `unlink` returning 0 does not say whether the file was missing or the directory was read-only. And `os.Remove` takes one path, so removing a list is a loop, which is exactly where you want to be when deciding what a partial failure means.

The same shift applies to `stat`. Perl hands back thirteen numbers and an empty list on failure; `os.Stat` hands back an `fs.FileInfo` and an error, and every field is a method on the value. Nothing has to remember that index 7 is the size, the fields that are not portable are simply absent, and one call answers every question the file tests ask separately - which is why a Go program that wants several of them keeps the `FileInfo` instead of asking again.

Further reading: https://go.dev/blog/error-handling-and-go and https://go.dev/blog/errors-are-values
