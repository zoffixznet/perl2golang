---
id: pointers-vs-references
title: Pointers are explicit references, and nothing aliases @_
tags: [idiom, pointers, functions, references]
perl_triggers: [scalar-ref, array-ref, hash-ref, deref, arrow-deref, arg-aliasing]
severity: info
prerequisites: [static-types-and-zero-values, structs-and-embedding]
---

Go pointers are Perl references with the training wheels *and* the magic removed: `&x` takes one (like `\$x`), `*p` dereferences (like `$$ref`), there is no pointer arithmetic, and — the part that bites — nothing is ever implicitly by-reference. Perl secretly aliases `@_` to the caller's arguments, so `$_[0]++` mutates the caller's variable; Go copies *every* argument, every time — scalars, structs, arrays — and mutation-at-a-distance requires the caller to visibly pass `&x`. The reflex to build: seeing `f(&job)` in Go code tells you `f` may mutate `job`; seeing `f(job)` guarantees it cannot (with the slice/map asterisk below).

## The Perl you know

```perl
sub bump { $_[0]++ }         # @_ aliases the caller's args
my $n = 1;
bump($n);
say $n;                      # 2, mutated through the alias

my $job = { retries => 0 };  # references are how everything nontrivial moves
$job->{retries} = 5;
```

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

func bumpValue(n int) { n++ } // operates on a copy

func bumpPtr(n *int) { *n++ }

type Job struct{ Retries int }

func tune(j *Job) { j.Retries = 5 } // j.Retries, not (*j).Retries: auto-deref

func main() {
	n := 1
	bumpValue(n)
	fmt.Println(n)
	bumpPtr(&n)
	fmt.Println(n)

	j := Job{}
	tune(&j)
	fmt.Println(j.Retries)

	p := &Job{Retries: 1} // & on a literal; escape analysis decides where it lives
	fmt.Println(p.Retries)
}
```

```
1
2
5
1
```

Note `j.Retries` working through a pointer with a plain dot — Go auto-dereferences struct field access, so Perl's visual distinction between `$job->{retries}` and `$job{retries}` has no Go counterpart; `.` serves both. Explicit `*p` is only needed for whole-value operations like `*n++` or `*p = Job{}`.

## The third reason: a pointer is how a value says it might not be there

The other two reasons to take a pointer are about mutation and about size. This one is about information, and it is the one a Perl program runs into first, because `undef` is a value every Perl scalar can hold and no Go number or string can. A `map[string]int` has two states per key, present and absent, and 0 belongs to the present one. A `map[string]*int` has three, which is what the Perl hash had all along:

```go
package main

import "fmt"

// ptr puts a value somewhere a *T can point at; deref reads one back and
// treats nil as the zero value.
func ptr[T any](v T) *T { return &v }

func deref[T any](p *T) T {
	if p == nil {
		var missing T
		return missing
	}
	return *p
}

func main() {
	limit := map[string]*int{"retries": ptr(0), "burst": nil}

	for _, k := range []string{"retries", "burst", "window"} {
		v, present := limit[k]
		fmt.Printf("%-8s present=%t set=%t value=%d\n", k, present, v != nil, deref(v))
	}

	// Filling a default in is the nil test, so a stored 0 keeps its place.
	if limit["retries"] == nil {
		limit["retries"] = ptr(3)
	}
	if limit["burst"] == nil {
		limit["burst"] = ptr(5)
	}
	fmt.Println(deref(limit["retries"]), deref(limit["burst"]))
}
```

```
retries  present=true set=true value=0
burst    present=true set=false value=0
window   present=false set=false value=0
0 5
```

`ptr` exists because Go will not take the address of a literal, of an arithmetic result, or of a map element, so a value going into such a slot needs a variable of its own to point at. `deref` exists because the alternative, writing `*p` at every read, is the line that crashes the first time the slot is empty, and because reading absence as the zero value is exactly what Perl did when undef met an operator.

The cost is real and worth weighing: every read grows a call, `++` stops working, and the type now has a nil case at every use. Pay it where the difference between "unset" and "zero" is something the program acts on, which for a configuration table or a sparse record it usually is, and decline it where every value is always present, where the plain type says something true and says it more simply.

## The mismatch

The mental model shift: Perl makes *aggregates* reference-typed by default (you cannot pass a hash without flattening it, so hashrefs rule), while Go passes aggregates by value happily and cheaply — a struct of five fields copies in nanoseconds, and returning structs by value is normal. Take pointers for three reasons only: the callee must mutate; the struct is genuinely large; or nil-ness must represent absence (`nil-vs-undef`). The asterisk footnote: slices and maps are small headers containing an internal pointer, so passing them by value still lets the callee mutate *elements* (`m[k] = v`, `s[0] = v` are visible to the caller) but not *resize* — the callee's `append` may go unseen (`slices-not-arrays`); this half-reference behaviour is the closest Go gets to `@_` aliasing and surprises in both directions. What does not exist at all: references to references as a data-structure trick, `ref()` introspection (see `type-assertions-and-switches`), symbolic references, and taking the address of a map element (`&m[k]` is a compile error — map storage moves). `new(T)` exists and returns `*T` with zero value, but idiomatic code writes `&T{}`; you will meet `new` mostly for `new(int)`-style pointers to primitives.

Further reading: https://go.dev/ref/spec#Address_operators and https://go.dev/doc/faq#pass_by_value
