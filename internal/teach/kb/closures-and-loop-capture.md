---
id: closures-and-loop-capture
title: Closures work like yours, and Go 1.22 fixed the loop trap
tags: [gotcha, functions, closures, loops]
perl_triggers: [anonymous-sub, coderef, callback, closure-over-loop-variable, dispatch-table, hash-of-subs, callback-pipeline]
severity: warning
prerequisites: [var-vs-short-declaration, range-is-not-foreach]
---

Good news first: Go closures are real closures over variables, exactly like Perl's, so your counter-factory and callback instincts transfer intact. The history you must know anyway: before Go 1.22, a `for` loop had *one* loop variable reused across iterations, so every closure created in the loop captured the same variable and observed its final value — the single most-written-about bug in the language's history. Go 1.22 (2024) changed the semantics: each iteration now gets a fresh variable. You will still meet the old pattern's scar tissue (`i := i` lines) and, in modules whose `go.mod` declares a pre-1.22 language version, the old behaviour itself is still in force.

## The Perl you know

```perl
sub make_counter {
    my $n = 0;
    return sub { return ++$n };     # closes over $n itself
}

my @subs;
push @subs, sub { print "$_\n" } for 0..2;   # each iteration's $_... careful
for my $i (0..2) {
    push @subs, sub { print "$i\n" };        # foreach my $i: fresh $i each pass — fine
}
```

Perl's `foreach my $i` always gave a fresh lexical per iteration, so Perl programmers largely never developed this bug. Go programmers did, for twelve years.

## The Go you write

Compiled and run as shown, under Go 1.22 or newer:

```go
package main

import "fmt"

func makeCounter() func() int {
	n := 0
	return func() int { // closes over n itself, not a copy
		n++
		return n
	}
}

func main() {
	next := makeCounter()
	fmt.Println(next(), next(), next())

	other := makeCounter() // independent state
	fmt.Println(other())

	// Loop variables: since Go 1.22 each iteration gets a FRESH i.
	var funcs []func()
	for i := 0; i < 3; i++ {
		funcs = append(funcs, func() { fmt.Print(i, " ") })
	}
	for _, f := range funcs {
		f()
	}
	fmt.Println()
}
```

```
1 2 3
1
0 1 2 
```

Under pre-1.22 semantics that last line printed `3 3 3`, and the ritual fix was shadowing the loop variable with `i := i` as the loop body's first line. The semantics that apply are chosen by the `go` directive in `go.mod` (the language version), not by the installed toolchain — old modules keep old behaviour until their directive is bumped.

## The mismatch

Functions are ordinary typed values — `var cb func(int) error` declares a nil callback, invocable only after assignment (calling a nil func panics), passable and returnable like any Perl coderef, minus the `->()` arrow: `cb(5)`, not `&$cb(5)`. What Perl's coderef culture has that Go's does not: no string `eval` to build code, no closures over *package* state via `local`, no `AUTOLOAD`-style late binding — a closure captures lexical variables and that is the entire mechanism. Watch one Perl-specific hazard in reverse: because closures capture *variables*, a closure and its surrounding code share mutable state, and handing such a closure to a goroutine creates a data race unless synchronised (`race-detector`) — Perl's threadless daily life never made you think about that. Also note methods can be captured as values too: `f := buf.WriteString` binds receiver and method into a plain function value (a "method value"), the Go answer to `$obj->can('method')` handles.

## Calling one, and getting the arguments there

`$code->(@args)` is one character of Perl and two decisions in Go.

The first is what the reference's type is. A closure whose signature the compiler can see is called like any other function, with no arrow and no assertion, and a factory that hands several of them back says so in its result list. Writing the result types out is worth doing even when Go would infer them, because the caller reads the signature and not the body:

```go
package main

import (
	"fmt"
	"strings"
)

// makeCounter is a Perl closure factory with its signatures written down. Both
// closures share `start`, and the caller can see from the result types which
// one takes an argument.
func makeCounter(start int) (bump func(int) int, peek func() int) {
	bump = func(by int) int {
		start += by
		return start
	}
	peek = func() int { return start }
	return bump, peek
}

func main() {
	bump, peek := makeCounter(100)
	fmt.Println(bump(5), bump(20), peek())

	// $joiner->(@parts) has to spread: Go passes one slice to a variadic
	// parameter, and only with the three dots.
	joiner := func(parts ...string) string { return strings.Join(parts, "|") }
	parts := []string{"alpha", "beta", "gamma"}
	fmt.Println(joiner(parts...))

	// Perl flattens every array in the call into one list. Go spreads exactly
	// one slice and will not mix it with other arguments, so the list is
	// built first.
	all := []string{"first"}
	all = append(all, parts...)
	all = append(all, parts...)
	fmt.Println(joiner(all...))
	fmt.Printf("[%s]\n", joiner())
}
```

```
105 125 125
alpha|beta|gamma
first|alpha|beta|gamma|alpha|beta|gamma
[]
```

Both closures in the factory read and write `start`, and neither of them copied it: a Go closure captures the variable, not its value, which is the behaviour Perl's `my` gives you and the reason a counter written this way works in either language.

The second decision is the arguments. Perl flattens every array in a call into one flat `@_`, so `$code->($x, @rest)` and `$code->(@rest)` are both just lists. Go spreads exactly one slice into a variadic parameter, it must be the last argument, and it cannot be mixed with others: `f(x, rest...)` is a compile error when `x` is also meant for the variadic part. So a mixed call becomes a slice built first and spread as a whole, which is what the third line above shows.

Two smaller traps live here. `f(parts...)` passes the *same backing array*, so a variadic function that writes to its parameter writes through to the caller's slice; Perl's `@_` aliases its arguments too, so this one is familiar. And a variadic function called with no arguments gets a nil slice rather than an empty one, which `strings.Join` and `range` both handle without complaint.

## The dispatch table, and why it costs something

The hash of subs is one of Perl's best idioms and one of the hardest things to bring across, because a Go map has one value type and the handlers in a real table do not agree about their signatures:

```perl
my %actions = (
    upper   => sub { uc $_[0] },                       # one arg, returns text
    tag     => sub { my ($s, $t) = @_; "[$t] $s" },    # two args
    note    => sub { push @audit, $_[0]; return },     # returns nothing
    measure => sub { length $_[0] },                   # returns a number
);
```

There are three ways across, in increasing order of how much Go you get for the work.

The mechanical one gives every handler the same signature: `map[string]func(...any) any`. It compiles, it keeps the table, and it throws away everything the compiler could have checked. Each handler reads its arguments out of a slice and returns a value nobody can use without an assertion. Take this when porting and move on; it is the one a converter can produce on its own.

The next one splits the table by shape. Most real tables are less varied than they look: three handlers take a string and return a string, one takes two, one returns nothing. `map[string]func(string) string` for the three, called directly, with the odd ones out written as plain functions, is usually smaller *and* clearer than the original.

The one Go actually wants is an interface:

```go
type Action interface {
	Name() string
	Apply(s string) (string, error)
}
```

Now the table is `map[string]Action`, each handler is a small type, the compiler checks every one of them, and adding a handler that forgets a method is a build error rather than a run-time surprise. It is more lines and it is the shape a Go reviewer will ask for.

One thing that gets *easier*: a closure created in a loop closes over the variable declared inside the loop, so each one really does get its own. That was true in Perl and, since Go 1.22, is true of the range variable as well, which removed the classic version of this bug from both languages at once.

Further reading: https://go.dev/blog/loopvar-preview and https://go.dev/ref/spec#Go_1.22
