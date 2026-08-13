---
id: panic-and-recover
title: panic is not die, recover is not eval
tags: [trap, errors, panic, recover]
perl_triggers: [die-as-control-flow, eval-to-catch, die-in-library, sig-die]
severity: trap
prerequisites: [errors-are-values, defer-timing]
---

`panic` looks like `die` and `recover` looks like `eval`, and building your error handling on that resemblance produces un-Go that reviewers will reject wholesale. A panic is for *programmer* errors - impossible states, violated invariants, out-of-range indexes - and the correct frequency of `panic` in application code is approximately zero, with `recover` rarer still. Runtime panics (nil dereference, index out of range) will nonetheless happen to your ported code, so you need to read their output fluently; and the one legitimate recover pattern - converting a third-party panic into an error at a package boundary - is worth having verbatim.

## The Perl you know

```perl
die "not found\n" if !$row;              # ordinary, expected failure
my $val = eval { risky() } // $default;  # catching is routine
```

In Perl, `die`/`eval` handle *expected* failures daily. That entire workload moves to error returns in Go (`errors-are-values`); what remains for panic is the stuff Perl would also consider a bug.

## The Go you write

An unrecovered panic kills the whole program with a goroutine dump - run as shown:

```go-fails
package main

import "fmt"

func main() {
	var xs []int
	fmt.Println(xs[3]) // index out of range: runtime panic
}
```

```
panic: runtime error: index out of range [3] with length 0

goroutine 1 [running]:
main.main()
	/.../paniccrash.go:7 +0x9
exit status 2
```

The boundary-recovery pattern - `recover` works *only* inside a deferred function, and named returns let it substitute an error - run as shown:

```go
package main

import "fmt"

func parseAll(inputs []string) (results []int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parseAll panicked: %v", r)
		}
	}()
	for _, in := range inputs {
		results = append(results, mustParse(in))
	}
	return results, nil
}

func mustParse(s string) int {
	if s == "" {
		panic("empty input") // stand-in for a misbehaving library
	}
	return len(s)
}

func main() {
	got, err := parseAll([]string{"ab", "cde"})
	fmt.Println(got, err)

	got, err = parseAll([]string{"ab", ""})
	fmt.Println(got, err)
}
```

```
[2 3] <nil>
[2] parseAll panicked: empty input
```

## The mismatch

Where the analogy breaks, point by point. Scope: `eval` catches everything below it; `recover` only rescues *its own goroutine* - a panic in a goroutine you spawned crashes the entire program regardless of recovers elsewhere, a failure mode with no Perl analogue (`goroutines-not-fork`). Ergonomics: there is no catch *block*; recover lives in a defer, applies to the whole function, and cannot resume where the panic happened - control continues at the function's return, so "catch, patch, continue the loop" needs restructuring. Legitimacy: `panic` is correct at initialisation time for programmer error - `regexp.MustCompile` with a bad pattern (`mustcompile-pattern`), missing required configuration at startup - where crashing early beats limping; the `MustXxx` naming convention marks exactly these. `log.Fatal` (print and `os.Exit(1)`) is *also* not `die` - it skips deferred functions entirely, so never use it outside `main`. And `$SIG{__DIE__}`-style global hooks do not exist; `net/http` recovering a handler's panic to keep the server up is the framework-level version, done for you. The blunt cultural rule: if a condition can be caused by input, environment, or another system, it is an `error`; panic is reserved for "this program is broken", and your reviewers will hold that line.

One thing the analogy gets *right*, and it is worth knowing because the Perl idiom depends on it: `panic` takes a value of any type, not a message, and `recover` returns exactly that value. `die $object` and `$@` holding the object come across intact, so the code that recovers can assert the value to an interface and go on calling methods on it. Perl's own `$@` behaves the same way: it holds a string when a string was thrown and a reference when a reference was, and `ref $@` is how the catching side tells them apart. In Go that test is `caught.(Reporter)` in its two-result form, or a type switch when several shapes are possible; `recover()` returning `nil` is the "nothing was thrown" case and is why the result is always tested (`comma-ok-idiom`, `type-assertions-and-switches`). Where you have a choice, panic with something implementing `error`: `recover` then hands the caller a value that fits every function that already takes an error, and converting the panic into a returned error at the package boundary becomes one line.

The other detail Perl programmers trip over: `$@` is cleared when an `eval` finishes without failing, so a nested `eval` that failed and was handled does not leave its message behind for the outer one to be blamed for. Nothing in Go corresponds, because there is no shared variable to clear: `recover` returns the value once, to the one deferred call that asked for it.

Further reading: https://go.dev/blog/defer-panic-and-recover
