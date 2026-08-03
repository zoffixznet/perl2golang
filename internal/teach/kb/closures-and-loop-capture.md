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
