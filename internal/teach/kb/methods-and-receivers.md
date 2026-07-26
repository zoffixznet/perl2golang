---
id: methods-and-receivers
title: Pointer versus value receivers, and the method set rules
tags: [trap, methods, pointers, oo]
perl_triggers: [self-shift, self-field-assignment, bless, method-call]
severity: trap
prerequisites: [structs-and-embedding, pointers-vs-references]
---

Every Perl method gets `$self` as a reference, so mutating `$self->{field}` always sticks. A Go method chooses its receiver form per method: `func (c Counter)` receives a *copy* — mutations evaporate silently — while `func (c *Counter)` receives a pointer and mutations persist. The failure mode is vicious precisely because it compiles and runs cleanly: a setter with a value receiver is a well-typed no-op. On top sits the *method set* rule: the value type `T` only owns the value-receiver methods, so a `T` (as opposed to `*T`) cannot satisfy an interface that needs a pointer-receiver method — a compile error whose message you should meet now, on purpose, rather than at midnight.

## The Perl you know

```perl
package Counter;
sub new { bless { n => 0 }, shift }
sub inc { my $self = shift; $self->{n}++ }   # $self is a ref; always mutates
```

There is one calling convention. `$obj->method` always hands the method the object itself.

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

type Counter struct {
	n int
}

func (c Counter) IncBroken() { // value receiver: c is a COPY
	c.n++
}

func (c *Counter) Inc() { // pointer receiver: modifies the real thing
	c.n++
}

func main() {
	c := Counter{}
	c.IncBroken()
	c.IncBroken()
	fmt.Println(c.n)

	c.Inc() // sugar for (&c).Inc() — Go takes the address for you
	c.Inc()
	fmt.Println(c.n)
}
```

```
0
2
```

Two increments through the value receiver: nothing. Note that `c.Inc()` worked on a plain `c` — for *addressable* values, Go auto-inserts the `&`. The place the distinction refuses to blur is interfaces:

```go-invalid
package main

type Counter struct {
	n int
}

func (c *Counter) Inc() { c.n++ }

type Incrementer interface {
	Inc()
}

var i Incrementer = Counter{} // value type lacks pointer-receiver methods

func main() {}
```

```
./methodset_err.go:13:21: cannot use Counter{} (value of struct type Counter) as Incrementer value in variable declaration: Counter does not implement Incrementer (method Inc has pointer receiver)
```

`var i Incrementer = &Counter{}` compiles: the method set of `*Counter` includes *both* receiver kinds; the method set of `Counter` includes only value receivers.

## The mismatch

Decision rule, then rationale. Rule: if *any* method needs to mutate, give *every* method of that type a pointer receiver; also use pointer receivers for large structs (copy cost) and for types containing a `sync.Mutex` or similar (copying a mutex is a bug `go vet` flags). Value receivers are for small, immutable, value-ish types — `time.Time`, `Celsius` from `constants-iota-named-types`. Mixing receiver kinds on one type is legal and almost always a smell. Rationale for the method-set asymmetry, so it sticks: calling a pointer-receiver method requires an address; interface values and map elements are not addressable, so Go cannot silently take `&` there the way it did for the local variable `c` — hence `m["k"].Inc()` on a map of structs fails to compile too (store `*Counter` in the map instead). Perl reflex to retire: "the method will fix up `$self`" — in Go the *type declaration* fixes the contract, and constructors returning `*T` (the overwhelming convention, `NewCounter() *Counter`) exist partly so every caller starts on the pointer side and never meets this trap.

Further reading: https://go.dev/ref/spec#Method_sets and https://go.dev/doc/faq#methods_on_values_or_pointers
