---
id: nil-vs-undef
title: nil is not undef, and nothing autovivifies
tags: [trap, nil, undef, autovivification, pointers]
perl_triggers: [undef, defined, autovivification, nested-hash-assignment, deref-maybe-undef]
severity: trap
prerequisites: [static-types-and-zero-values, structs-and-embedding]
---

`undef` is a universal value any scalar can hold; `nil` is not a value of most Go types at all - an `int` or `string` can never be nil, only pointers, maps, slices, channels, functions, and interfaces can. More dangerous for your instincts: Perl rewards optimistic deep access (`$h{a}{b}{c} = 1` builds the path; `$obj->{x}` on undef merely warns), while Go punishes it at runtime - dereferencing a nil pointer panics, writing through a nil inner map panics, and *nothing anywhere autovivifies*. Ported Perl code that walks or builds nested structures is the single most reliable source of Go runtime panics.

## The Perl you know

```perl
my %h;
$h{a}{b}{c} = 1;      # intermediate hashes spring into being
# {'a' => {'b' => {'c' => 1}}}

print $h{x}{y};        # undef, prints nothing... but ALSO vivifies $h{x}!
```

Even `exists $h{x}{y}` autovivifies `$h{x}` - Perl's forgiveness has its own famous trap, in the opposite direction from Go's.

## The Go you write

A nil pointer dereference is a crash, not a warning - run as shown:

```go-fails
package main

import "fmt"

type Config struct {
	Host string
}

func main() {
	var cfg *Config // nil: no Config was ever loaded
	fmt.Println(cfg.Host)
}
```

```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x49e196]

goroutine 1 [running]:
main.main()
	/.../nilpanic.go:11 +0x16
exit status 2
```

Deep assignment does not build the path - this also panics, as shown:

```go-fails
m := map[string]map[string]int{}
m["a"]["b"] = 1 // no autovivification: m["a"] is a nil map
```

```
panic: assignment to entry in nil map
```

Vivify by hand, and note the asymmetry - *reading* through absence is safe:

```go
package main

import "fmt"

func main() {
	m := map[string]map[string]int{}

	if m["a"] == nil {
		m["a"] = map[string]int{} // vivify by hand
	}
	m["a"]["b"] = 1

	fmt.Println(m)
	fmt.Println(m["missing"]["x"]) // READING through absence is safe: 0
	fmt.Println(len(m))            // and it did not create m["missing"]
}
```

```
map[a:map[b:1]]
0
1
```

## // is a different question from ||, and Go can only answer one of them

`$port || 8080` and `$port // 8080` differ on exactly one input, and it is the one a configuration file is most likely to contain: a zero. `||` asks whether the value is true and replaces a port of 0; `//` asks whether it has a value at all and keeps it. Perl grew the second operator because the first kept getting this wrong.

Go can answer the second question in one place and nowhere else. A map read has a two-result form, and that form is the question:

```go
package main

import "fmt"

func main() {
	conf := map[string]int{"port": 0}

	// $conf{port} // 8080 -- the question is whether the key is there.
	port, ok := conf["port"]
	if !ok {
		port = 8080
	}

	// $conf{port} || 8080 -- the question is whether the value is true.
	loud := conf["port"]
	if loud == 0 {
		loud = 8080
	}
	fmt.Println(port, loud)

	// A key that is present and one that is absent read the same way on
	// their own, which is why the comma-ok form exists at all.
	fmt.Println(conf["port"], conf["timeout"])

	// Where a value really can be absent, the type says so.
	timeout := map[string]*int{"idle": nil, "read": intp(30)}
	for _, k := range []string{"idle", "read", "write"} {
		if v, found := timeout[k]; found && v != nil {
			fmt.Printf("%s=%d\n", k, *v)
		} else if found {
			fmt.Printf("%s is there and empty\n", k)
		} else {
			fmt.Printf("%s is not there\n", k)
		}
	}
}

func intp(n int) *int { return &n }
```

```
0 8080
0 0
idle is there and empty
read=30
write is not there
```

### A run of // is one decision, not a chain of two-value expressions

`$opt{a} // $opt{b} // 5` cannot be built up one operator at a time, because the value of `$opt{a} // $opt{b}` is itself either a number or nothing, and no Go expression has room for that second answer. Take the whole run as one decision and it becomes an if/else ladder where each rung asks the question its own type can answer:

```go
package main

import "fmt"

func main() {
	opt := map[string]int{"retries": 0}

	// $opt{retries} // $opt{tries} // 5
	var retries int
	if v, ok := opt["retries"]; ok {
		retries = v
	} else if v, ok := opt["tries"]; ok {
		retries = v
	} else {
		retries = 5
	}

	// $opt{tries} // $opt{retries} // 5
	var tries int
	if v, ok := opt["tries"]; ok {
		tries = v
	} else if v, ok := opt["retries"]; ok {
		tries = v
	} else {
		tries = 5
	}

	// The truth test that || would have become gets both of them wrong.
	loud := opt["retries"]
	if loud == 0 {
		loud = 5
	}

	fmt.Println(retries, tries, loud)
}
```

```
0 0 5
```

Two things are worth noticing. The ladder is lazy in the same way the operator was: a rung's lookup only happens when every rung above it has failed, which is what the `if` statement's own initialiser buys. And each rung is free to ask differently: a plain map asks with the two-result form, a `map[string]*int` asks with `!= nil`, and a variable that was given a value where it was declared has no question to ask at all, so that rung ends the ladder and everything below it is dead code worth deleting rather than translating.

For a plain variable there is no such form, and there is no way to add one: `var n int` holds 0 from the moment it exists, and nothing distinguishes that from a 0 someone stored. Two consequences follow, and they are worth taking seriously rather than working around.

The first is that a `defined` test on an ordinary variable usually has one answer. A variable given a value where it is declared and never set to undef always has one, so the test is a constant and writing it out as a zero-value comparison would be worse than useless: it would answer a different question, and answer it wrongly for exactly the values that matter.

The second is that where a value genuinely can be absent, the type has to say so. `*int` is Go's undef, and it is a real declaration with real costs: a dereference at every read, and a nil check before each one. The loop at the end of the sample shows the three states a `map[string]*int` can be in -- absent, present and empty, present with a value -- which is precisely what a Perl hash gives you for free and what a `map[string]int` cannot express at all.

## The mismatch

Translate the concept, not the word: where Perl code means "no value yet" for a whole record, Go uses a nil *pointer* (`*Config`), and every dereference of it must be guarded or provably preceded by initialisation - there is no warn-and-continue mode. Where Perl means "this field might be absent as opposed to zero", Go uses a pointer field or comma-ok lookup (`comma-ok-idiom`); plain `int`/`string` fields cannot express absence at all (`static-types-and-zero-values`). And notice the read/write asymmetry demonstrated above, because it is exactly inverted from Perl: Go reads through missing nested keys safely *without* creating anything (no `exists`-vivifies gotcha), but writes need the path built explicitly. When porting a Perl structure-builder, the `if m[k] == nil { m[k] = ... }` dance is the honest translation of autovivification - or restructure to a flat map with a struct key and skip nesting entirely (`maps-of-slices`). Finally, nil compares only against nilable types: `x == nil` on an `int` is a compile error, so "is it defined?" is frequently a question Go makes unaskable - by design.

Further reading: https://go.dev/ref/spec#The_zero_value and https://go.dev/doc/faq#nil_error
