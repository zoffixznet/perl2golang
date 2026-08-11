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

## Asking the question at run time

The compiler answers "does this have the method?" wherever the type is known, which is nearly everywhere. Where it is not known, because the value came out of a container that holds anything, the same question is asked with a type assertion against a *capability* interface: an interface named for the one thing you need the value to do. Closing is the everyday case, and `io.Closer` is that interface with exactly one method in it.

```go
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// closeIfPossible asks a value of no fixed type whether it can be closed, and
// closes it if it can. This is `$fh->can('close')` asked at the same moment,
// except that once it has been answered the value is typed.
func closeIfPossible(v any) bool {
	c, ok := v.(io.Closer)
	if !ok {
		return false
	}
	return c.Close() == nil
}

// journal is an ordinary type that happens to have a Close method. Nothing
// anywhere declares that it satisfies io.Closer; having the method is all of it.
type journal struct {
	name string
	out  *strings.Builder
}

func (j *journal) Close() error {
	fmt.Fprintf(j.out, "journal %s closed\n", j.name)
	return nil
}

func main() {
	var log strings.Builder

	f, err := os.CreateTemp("", "handles")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer os.Remove(f.Name())

	handles := []any{
		f,
		&journal{name: "audit", out: &log},
		"a filename, not a handle",
		42,
	}
	for _, h := range handles {
		fmt.Printf("%T: closed %t\n", h, closeIfPossible(h))
	}
	fmt.Print(log.String())
}
```

```
*os.File: closed true
*main.journal: closed true
string: closed false
int: closed false
journal audit closed
```

A file and a type written this afternoon are equally acceptable, and the string and the number are rejected without stopping the program. Two habits to take from it. Name the interface for the capability, not for the family of types that has it: `io.Closer` says what will be done, and `Handle` would only have said where the value came from. And keep the assertion at the edge, where the untyped value arrives; the moment it succeeds you are back in typed code, and every line after it is checked by the compiler again.

## The mismatch

Culture first: Go interfaces are *small* — one to three methods — and named for what they do (`Reader`, `Stringer`, `Notifier`; the `-er` suffix is near-universal). The Perl instinct of designing a wide role/duck contract up front inverts: in Go you write concrete types first and *extract* an interface at the point of use, usually when a second implementation or a test double appears — defining an interface with one implementation and no consumer is flagged in review as speculative. Mechanics to note: interface satisfaction respects method sets, so pointer-receiver methods mean only `*T` satisfies (`methods-and-receivers`); interfaces embed other interfaces (`io.ReadWriter` is `Reader` + `Writer` — `io-reader-writer`); and the runtime `can()` equivalent still exists when you genuinely need it, as a type assertion to a capability interface — `if s, ok := v.(interface{ Flush() error }); ok { s.Flush() }` — the checked, typed version of your defensive dispatch (`type-assertions-and-switches`). What has no equivalent: `AUTOLOAD` (nothing can claim to satisfy methods it does not have), runtime method injection, and `DOES`/role application — composition is done in struct fields and embedding, at compile time.

The place a Perl program most often needs one and never says so is a collection: `my @shapes = (Rectangle->new(...), Circle->new(...))` and then `$_->area for @shapes`. Perl needs no declaration because the method is looked up on each value as the loop reaches it. Go has one element type per slice, so the array's own type is the thing that has to be named: `[]Shaper`, where `Shaper` lists the methods every element answers to. Writing it down is not overhead, it is the part the Perl left implicit, and it belongs beside the loop that consumes the values rather than beside the types that satisfy it. The one thing it cannot carry across is a field: `$_->{name}` inside such a loop has no interface spelling, and the fix is to give the shared base a method that returns it.

Further reading: https://go.dev/doc/effective_go#interfaces and https://go.dev/ref/spec#Interface_types
