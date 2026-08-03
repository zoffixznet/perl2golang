---
id: late-binding-vs-embedding
title: Embedding resolves at compile time, and Perl's method lookup does not
tags: [oo, embedding, interfaces, dispatch]
perl_triggers: [isa-array, use-parent, super-call, template-method, abstract-method, virtual-method, isa-check, error-hierarchy, exception-objects, die-with-object]
severity: warning
prerequisites: [structs-and-embedding, implicit-interfaces, methods-and-receivers]
---

Embedding gets you most of the way from `@ISA` to Go, and then stops at one exact place: a method of the base type calling another method *on itself*. Perl looks that call up on the object's real class every time, so the subclass's version runs. Go resolved it when it compiled the base method, against the type the receiver is declared as — which inside a base method is always the base. The code compiles, the shapes look right, and the answer is quietly wrong. This is the single most instructive difference between the two object systems, and the fix has a name in Go: an interface plus composition.

## The Perl you know

```perl
package Shape;
sub new { my ($class, %a) = @_; return bless { name => $a{name} }, $class }
sub area { 0 }                              # the step a subclass supplies
sub describe {                              # the template method
    my $self = shift;
    return sprintf "%s with area %.2f", $self->{name}, $self->area;
}

package Rectangle;
our @ISA = ('Shape');
sub area { my $self = shift; return $self->{w} * $self->{h} }
```

`describe` is written once and every subclass gets it. `$self->area` inside it is a fresh lookup on `ref($self)`, so a `Rectangle` reaches `Rectangle::area`: `rect with area 12.00`.

## The Go you write

Translate `@ISA` to embedding and the outer call still works, because the caller holds a `*Rectangle`. The inner one does not:

```go
package main

import "fmt"

type Shape struct{ Name string }

func (s *Shape) Area() float64 { return 0 }

func (s *Shape) Describe() string {
	return fmt.Sprintf("%s with area %.2f", s.Name, s.Area())
}

type Rectangle struct {
	Shape
	W, H float64
}

func (r *Rectangle) Area() float64 { return r.W * r.H }

func main() {
	r := &Rectangle{Shape: Shape{Name: "rect"}, W: 4, H: 3}
	fmt.Println(r.Area())
	fmt.Println(r.Describe())
}
```

```
12
rect with area 0.00
```

`r.Area()` finds the override, because `r` is a `*Rectangle`. `r.Describe()` is shorthand for `r.Shape.Describe()`, whose receiver is a `*Shape` that has never heard of `Rectangle`, so `s.Area()` is `Shape.Area` and the answer is zero. Nothing warns. There is no `virtual` keyword to have forgotten and no override to mark: promotion is a rewrite of the call, not a slot in a table.

The Go shape for this is an interface for the varying step, and a base that holds the finished object so it can call back through it:

```go
package main

import "fmt"

// Shaper is the contract: what every shape can do. Describe comes from the
// shared base, Area from each concrete type.
type Shaper interface {
	Area() float64
	Describe() string
}

// base holds the shared state and the shared behaviour, plus the finished
// object, which is what lets a base method reach an override.
type base struct {
	name string
	self Shaper
}

func (b *base) Describe() string {
	return fmt.Sprintf("%s with area %.2f", b.name, b.self.Area())
}

type Rectangle struct {
	base
	W, H float64
}

func NewRectangle(name string, w, h float64) *Rectangle {
	r := &Rectangle{base: base{name: name}, W: w, H: h}
	r.self = r
	return r
}

func (r *Rectangle) Area() float64 { return r.W * r.H }

type Circle struct {
	base
	R float64
}

func NewCircle(name string, radius float64) *Circle {
	c := &Circle{base: base{name: name}, R: radius}
	c.self = c
	return c
}

func (c *Circle) Area() float64 { return 3.14159 * c.R * c.R }

func main() {
	for _, s := range []Shaper{NewRectangle("rect", 4, 3), NewCircle("wheel", 2.5)} {
		fmt.Println(s.Describe())
	}
}
```

```
rect with area 12.00
wheel with area 19.63
```

`base.self` is what `$self` was doing all along, written down. The constructor is the one place that can set it, which is why each type needs one — a bare `Rectangle{}` literal would leave `self` nil and `Describe` would panic, and that is the cost of the pattern.

## The other half: embedding is not subtyping

The template-method problem is about *calls*. There is a second, quieter half about *values*, and it bites the moment an error hierarchy is ported. Perl's `isa` walks `@ISA`, so a `Failure::Timeout` is a `Failure::Network` and a `Failure` at the same time, and one variable holds any of them. Go has no such relation: embedding `NetworkFailure` inside `TimeoutFailure` promotes its fields and its methods, and does nothing else. `*TimeoutFailure` is not assignable to `*NetworkFailure`, no assertion will convert one to the other, and there is no cast that pretends otherwise.

So the two questions Perl asks of an object split apart in Go. "Can it do this?" is an interface, and an assertion answers it for a value of unknown type. "Is it one of these?" has no built-in answer at all: you write the list.

```go
package main

import "fmt"

type Failure struct {
	detail string
	code   int
}

// Detail is a method rather than an exported field, so the interface below
// can promise it. An interface never promises a field.
func (f *Failure) Detail() string { return f.detail }
func (f *Failure) Code() int      { return f.code }
func (f *Failure) Label() string  { return "failure" }

type NetworkFailure struct{ Failure }

func (n *NetworkFailure) Label() string { return "network" }

type TimeoutFailure struct{ NetworkFailure }

func (t *TimeoutFailure) Label() string { return "timeout" }

// Reporter is what every failure here answers to, written next to the code
// that needs it rather than next to the types.
type Reporter interface {
	Detail() string
	Code() int
	Label() string
}

// isNetworkFailure is what isa('Failure::Network') becomes. Embedding
// promotes fields and methods but is not subtyping, so a *TimeoutFailure
// does not satisfy an assertion to *NetworkFailure and the types have to be
// listed.
func isNetworkFailure(v any) bool {
	switch v.(type) {
	case *NetworkFailure, *TimeoutFailure:
		return true
	}
	return false
}

func attempt(kind string) (result string) {
	defer func() {
		// recover hands back exactly what panic was given, so an object
		// thrown survives whole and can still be asked questions.
		if caught := recover(); caught != nil {
			r, ok := caught.(Reporter)
			if !ok {
				result = fmt.Sprintf("not a failure: %v", caught)
				return
			}
			result = fmt.Sprintf("%s(%d): %s network=%t",
				r.Label(), r.Code(), r.Detail(), isNetworkFailure(caught))
		}
	}()
	switch kind {
	case "net":
		panic(&NetworkFailure{Failure{detail: "connection refused", code: 61}})
	case "slow":
		panic(&TimeoutFailure{NetworkFailure{Failure{detail: "no answer", code: 60}}})
	case "odd":
		panic("a bare string, not an object")
	}
	return kind + " ok"
}

func main() {
	for _, kind := range []string{"fine", "net", "slow", "odd"} {
		fmt.Println(attempt(kind))
	}
	// The assertion is checked: a value that is not a Reporter is caught
	// here rather than three frames later.
	var v any = 42
	_, ok := v.(Reporter)
	fmt.Println("42 is a Reporter:", ok)
}
```

```
fine ok
network(61): connection refused network=true
timeout(60): no answer network=true
not a failure: a bare string, not an object
42 is a Reporter: false
```

Three details worth keeping. `Detail` and `Code` are methods rather than exported fields, because an interface can promise a method and never a field; that is the reason a Perl accessor, which usually wants to vanish into an exported field, has to stay a method as soon as anything calls it through an interface. The predicate has to be maintained: a new subclass is a new `case`, and forgetting it is a bug the compiler cannot see, which is the price Go charges for deciding the rest at compile time. And note that `Label` still resolves the wrong way inside a base method for the reason the first half of this page explains: `isNetworkFailure` and the interface fix the *value* half of the problem, not the *call* half.

## The mismatch

Three things to carry over. First, the rule for when embedding is enough: a base method that calls only *its own* fields and its own helpers is fine, and so is any call made from outside the type, because there the caller holds the concrete value. Only a base method calling a method the subclass overrides needs the interface. Second, prefer to avoid the callback entirely — Go's usual answer is not a base class at all but a small interface consumed by a plain function: `func Describe(s Shaper) string` takes any shape, needs no `self` field, and cannot be left nil. Reach for the `self` field only when a real hierarchy is being ported and the base holds state the subclasses share. Third, `SUPER::` is the one direction that does survive: it becomes a call on the embedded field by name, `r.base.Describe()`, and Go resolves it at compile time so a rename is a build error rather than a surprise at run time.

Further reading: https://go.dev/doc/effective_go#embedding and https://go.dev/doc/faq#inheritance
