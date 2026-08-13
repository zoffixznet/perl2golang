---
id: multiple-return-values
title: Multiple returns replace both list-return and wantarray
tags: [idiom, functions, returns]
perl_triggers: [list-return, wantarray, list-assignment, context-sensitive-return]
severity: info
prerequisites: [var-vs-short-declaration]
---

Go functions return a fixed, typed tuple - `(int, error)` is the canonical shape - and the caller must account for every value: taking one value from a two-value function is a compile error, not a silent scalar-context conversion. This kills `wantarray` and the entire notion of context-sensitive returns: a Go function returns the same things to every caller, and if you do not want one of them, you discard it *visibly* with `_`.

## The Perl you know

```perl
sub minmax { my @s = sort { $a <=> $b } @_; return ($s[0], $s[-1]) }
my ($lo, $hi) = minmax(4, 9, 1, 7);
my $count = () = minmax(4, 9);        # context tricks

sub ctx { return wantarray ? "list" : "scalar" }
my @a = ctx();   # "list"
my $s = ctx();   # "scalar"    - one sub, two behaviours
```

## The Go you write

Compiled and run as shown:

```go
package main

import (
	"fmt"
	"strconv"
)

func parsePort(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("bad port %q: %w", s, err)
	}
	return n, nil
}

// Named returns document meaning and enable naked returns.
func minMax(xs []int) (lo, hi int) {
	lo, hi = xs[0], xs[0]
	for _, x := range xs[1:] {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	return // "naked" return: returns the current lo, hi
}

func main() {
	p, err := parsePort("8080")
	fmt.Println(p, err)

	_, err = parsePort("http")
	fmt.Println(err)

	lo, hi := minMax([]int{4, 9, 1, 7})
	fmt.Println(lo, hi)
}
```

```
8080 <nil>
bad port "http": strconv.Atoi: parsing "http": invalid syntax
1 9
```

Under-taking values is rejected outright:

```go-invalid
package main

import "strconv"

func main() {
	n := strconv.Atoi("8080") // forgot the error
	_ = n
}
```

```
./multiret_err.go:6:7: assignment mismatch: 1 variable but strconv.Atoi returns 2 values
```

## A list whose length only the array knows

The shape that looks impossible at first is `return ($cost, @path)`: a flat
list with a fixed head and a tail of unknown length. It has a precise Go
spelling, because the caller was never really consuming a flat list; the
idiomatic Perl caller wrote `my ($cost, @path) = route($a, $b)`, peeling the
fixed values off the front and letting the array swallow the rest. Give the
function that same split as its signature, fixed results first and one slice
last:

```go
package main

import (
	"fmt"
	"strings"
)

// route returns the cost and the stops. Perl returned one flat list,
// "return ($cost, @path)", whose length only the array knew; the slice as
// the final result is the same split the caller was already making with
// "my ($cost, @path) = route(...)".
func route(dst string) (int, []string) {
	if dst == "island" {
		return 0, nil // Perl's bare return: the empty list
	}
	return 17, []string{"depot", "central", dst}
}

func main() {
	for _, dst := range []string{"airport", "island"} {
		cost, path := route(dst)
		if len(path) == 0 {
			fmt.Printf("%s: unreachable\n", dst)
			continue
		}
		fmt.Printf("%s: %d min via %s\n", dst, cost, strings.Join(path, " -> "))
	}
}
```

```
airport: 17 min via depot -> central -> airport
island: unreachable
```

Three details carry over exactly. Perl's bare `return`, the empty list, is
`return 0, nil`: a nil slice has length zero, so the caller's "did I get a
path" test, which was `!@path`, becomes `len(path) == 0` and keeps working.
The array must come last, because only the final position can absorb a run of
unknown length; an array in the middle of a Perl return shifts everything
after it and has no fixed-arity Go spelling at all. And a caller who wanted
the whole thing as one flat list again, which Perl gave for free, writes the
flattening out: `append([]any{cost}, ...)` or a small struct, whichever names
the meaning better. Reaching for the struct is usually the right instinct;
`(cost int, path []string)` at two or three call sites is fine, but the
moment the pair travels together through more code than that, it wants to be
a type.

## The mismatch

`my ($x, $y) = f()` translates directly to `x, y := f()`, but the resemblance is syntactic only: Perl's list assignment tolerates any length mismatch, Go's tolerates none - the arity is part of the function's type. `wantarray`-driven subs must be split into two named functions (or return the richer shape and let callers ignore parts with `_`); there is no way, and deliberately so, for a Go function to know how it is being called. On *named returns* (`(lo, hi int)` above): they pre-declare zero-valued result variables and permit a bare `return`, which reads nicely in ten-line functions and turns hostile in fifty-line ones - a naked `return` forces readers to reconstruct the current state of mutable variables, and a mid-function `return` that forgets to set one of them returns a silent zero. House style in most Go shops: use named returns for documentation (especially two same-typed results like `(latitude, longitude float64)`) or when a deferred function must modify the result (`panic-and-recover`), but always `return lo, hi` explicitly. Also note what does not exist: no returning a list that flattens into caller context, no `return;` meaning empty-list-or-undef - a bare `return` is only legal with named results or a void function.

Further reading: https://go.dev/doc/effective_go#multiple-returns
