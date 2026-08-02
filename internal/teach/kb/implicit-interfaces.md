---
id: implicit-interfaces
title: Interfaces are satisfied implicitly - duck typing, checked
tags: [orientation, interfaces, polymorphism]
perl_triggers: [can-method, duck-typing, isa-dispatch, does-method, role-composition]
severity: info
prerequisites: [methods-and-receivers]
---

Go's interface is Perl duck typing with the runtime risk removed: a type satisfies an interface by simply *having* the right methods — no `implements` declaration, no registration, no inheritance relationship, nothing at the type's definition site at all. The polymorphism you built with `$obj->can('notify')` checks or "just call it and pray" works the same way in Go, except the compiler performs the `can()` check at every assignment, so "method not found" ceases to exist as a runtime event. The under-appreciated consequence: you can define an interface *after the fact* for types you do not own, including stdlib types — the consumer, not the provider, owns the contract.

## The Perl you know

```perl
# Duck typing: anything with a notify() works, verified at call time or never.
for my $sink (@sinks) {
    $sink->notify($msg) if $sink->can('notify');   # defensive can()
}
# can() answers method existence at run time, per object
```

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

type Notifier interface {
	Notify(msg string) error
}

type EmailSender struct{ Addr string }

func (e EmailSender) Notify(msg string) error {
	fmt.Println("email to", e.Addr+":", msg)
	return nil
}

type SlackSender struct{ Channel string }

func (s SlackSender) Notify(msg string) error {
	fmt.Println("slack", s.Channel+":", msg)
	return nil
}

// Compile-time conformance check; costs nothing at runtime.
var _ Notifier = EmailSender{}

func alertAll(ns []Notifier, msg string) {
	for _, n := range ns {
		if err := n.Notify(msg); err != nil {
			fmt.Println("notify failed:", err)
		}
	}
}

func main() {
	alertAll([]Notifier{
		EmailSender{Addr: "ops@example.com"},
		SlackSender{Channel: "#alerts"},
	}, "disk almost full")
}
```

```
email to ops@example.com: disk almost full
slack #alerts: disk almost full
```

Neither sender type mentions `Notifier` — satisfaction is structural. Get the shape wrong and the error is precise, at compile time:

```go-invalid
package main

type Notifier interface {
	Notify(msg string) error
}

type Pager struct{}

func (p Pager) Notify(msg string) {} // wrong signature: missing error return

var n Notifier = Pager{}

func main() {}
```

```
./implicit_err.go:11:18: cannot use Pager{} (value of struct type Pager) as Notifier value in variable declaration: Pager does not implement Notifier (wrong type for method Notify)
		have Notify(string)
		want Notify(string) error
```

The `var _ Notifier = EmailSender{}` line is the idiomatic way to assert conformance in the implementing package — it fails the build the moment a refactor breaks the contract.

## The mismatch

Culture first: Go interfaces are *small* — one to three methods — and named for what they do (`Reader`, `Stringer`, `Notifier`; the `-er` suffix is near-universal). The Perl instinct of designing a wide role/duck contract up front inverts: in Go you write concrete types first and *extract* an interface at the point of use, usually when a second implementation or a test double appears — defining an interface with one implementation and no consumer is flagged in review as speculative. Mechanics to note: interface satisfaction respects method sets, so pointer-receiver methods mean only `*T` satisfies (`methods-and-receivers`); interfaces embed other interfaces (`io.ReadWriter` is `Reader` + `Writer` — `io-reader-writer`); and the runtime `can()` equivalent still exists when you genuinely need it, as a type assertion to a capability interface — `if s, ok := v.(interface{ Flush() error }); ok { s.Flush() }` — the checked, typed version of your defensive dispatch (`type-assertions-and-switches`). What has no equivalent: `AUTOLOAD` (nothing can claim to satisfy methods it does not have), runtime method injection, and `DOES`/role application — composition is done in struct fields and embedding, at compile time.

The place a Perl program most often needs one and never says so is a collection: `my @shapes = (Rectangle->new(...), Circle->new(...))` and then `$_->area for @shapes`. Perl needs no declaration because the method is looked up on each value as the loop reaches it. Go has one element type per slice, so the array's own type is the thing that has to be named: `[]Shaper`, where `Shaper` lists the methods every element answers to. Writing it down is not overhead, it is the part the Perl left implicit, and it belongs beside the loop that consumes the values rather than beside the types that satisfy it. The one thing it cannot carry across is a field: `$_->{name}` inside such a loop has no interface spelling, and the fix is to give the shared base a method that returns it.

Further reading: https://go.dev/doc/effective_go#interfaces and https://go.dev/ref/spec#Interface_types
