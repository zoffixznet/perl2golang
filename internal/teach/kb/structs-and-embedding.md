---
id: structs-and-embedding
title: Structs replace hashrefs, and embedding is not inheritance
tags: [orientation, types, structs, embedding, oo]
perl_triggers: [hashref-object, bless, use-parent, isa-array, self-field, moose-has]
severity: info
prerequisites: [static-types-and-zero-values, packages-and-exported-names]
---

The blessed hashref — Perl's universal object — becomes a struct: a fixed set of named, typed fields declared up front, where accessing a misspelled field is a compile error instead of a silently autovivified key. Inheritance does not come along for the ride: Go has no `@ISA`, no `use parent`, no method resolution order. What it has is *embedding*, which looks like inheritance for the first hour (fields and methods of the embedded type are reachable through the outer one) and then reveals itself as pure composition with sugar — and code ported as if it were a class hierarchy will fight the language at every step.

## The Perl you know

```perl
package Person;
use parent -norequire, 'Address';   # is-a, MRO, the whole apparatus

sub new {
    my ($class, %args) = @_;
    return bless { name => $args{name}, emial => $args{email} }, $class;
    #                                   ^^^^^ typo lives until runtime, or forever
}
```

Fields are hash keys: dynamic, typo-prone, discoverable only by reading `new` and every method that touches `$self`.

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

type Address struct {
	Street string
	City   string
}

type Person struct {
	Name  string
	Email string
	Address
}

func (a Address) OneLine() string {
	return a.Street + ", " + a.City
}

func main() {
	p := Person{
		Name:  "Jane Doe",
		Email: "jane.doe@example.com",
		Address: Address{
			Street: "123 Example St",
			City:   "Springfield",
		},
	}

	fmt.Println(p.City)      // promoted field from the embedded Address
	fmt.Println(p.OneLine()) // promoted method
	fmt.Printf("%+v\n", p)

	q := p // structs are values: this copies everything
	q.City = "Shelbyville"
	fmt.Println(p.City, q.City)
}
```

```
Springfield
123 Example St, Springfield
{Name:Jane Doe Email:jane.doe@example.com Address:{Street:123 Example St City:Springfield}}
Springfield Shelbyville
```

`Address` appears in `Person` with no field name — that is embedding. `p.City` and `p.OneLine()` are *promotion*: shorthand for `p.Address.City` and `p.Address.OneLine()`. Always use field names in literals (`Name: ...`) — positional literals `Person{"Jane", ...}` compile but break when fields are added, and vet flags them in other packages.

## The mismatch

Three re-wirings. First, structs are values: `q := p` deep-copies every field (the output proves `p` was untouched), where copying a hashref copies a pointer — Perl's reference semantics correspond to `*Person`, not `Person`, and choosing value versus pointer is now an explicit, recurring decision (see `pointers-vs-references` and `methods-and-receivers`). Second, embedding is not is-a: `OneLine` above runs with the *Address* as its receiver and cannot see `Person` fields — there is no `$self` rebinding, no `SUPER::`, no override-and-call-parent chain; if `Person` declares its own `OneLine`, the outer one simply shadows the inner, which remains reachable as `p.Address.OneLine()`. Method dispatch is resolved at compile time; polymorphism lives exclusively in interfaces (`implicit-interfaces`). Third, there is no constructor protocol: `Person{...}` is the literal syntax, the zero value should ideally be usable as-is, and when construction needs logic the convention is a plain function `NewPerson(...) (*Person, error)` — a naming custom, not a language feature, with no `new`/`bless` magic behind it.

Further reading: https://go.dev/ref/spec#Struct_types and https://go.dev/doc/effective_go#embedding
