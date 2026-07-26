---
id: nil-vs-undef
title: nil is not undef, and nothing autovivifies
tags: [trap, nil, undef, autovivification, pointers]
perl_triggers: [undef, defined, autovivification, nested-hash-assignment, deref-maybe-undef]
severity: trap
prerequisites: [static-types-and-zero-values, structs-and-embedding]
---

`undef` is a universal value any scalar can hold; `nil` is not a value of most Go types at all — an `int` or `string` can never be nil, only pointers, maps, slices, channels, functions, and interfaces can. More dangerous for your instincts: Perl rewards optimistic deep access (`$h{a}{b}{c} = 1` builds the path; `$obj->{x}` on undef merely warns), while Go punishes it at runtime — dereferencing a nil pointer panics, writing through a nil inner map panics, and *nothing anywhere autovivifies*. Ported Perl code that walks or builds nested structures is the single most reliable source of Go runtime panics.

## The Perl you know

```perl
my %h;
$h{a}{b}{c} = 1;      # intermediate hashes spring into being
# {'a' => {'b' => {'c' => 1}}}   — verified

print $h{x}{y};        # undef, prints nothing... but ALSO vivifies $h{x}!
```

Verified: even `exists $h{x}{y}` autovivifies `$h{x}` — Perl's forgiveness has its own famous trap, in the opposite direction from Go's.

## The Go you write

A nil pointer dereference is a crash, not a warning — run as shown:

```go
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

Deep assignment does not build the path — this also panics, as shown:

```go
m := map[string]map[string]int{}
m["a"]["b"] = 1 // no autovivification: m["a"] is a nil map
```

```
panic: assignment to entry in nil map
```

Vivify by hand, and note the asymmetry — *reading* through absence is safe:

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

## The mismatch

Translate the concept, not the word: where Perl code means "no value yet" for a whole record, Go uses a nil *pointer* (`*Config`), and every dereference of it must be guarded or provably preceded by initialisation — there is no warn-and-continue mode. Where Perl means "this field might be absent as opposed to zero", Go uses a pointer field or comma-ok lookup (`comma-ok-idiom`); plain `int`/`string` fields cannot express absence at all (`static-types-and-zero-values`). And notice the read/write asymmetry demonstrated above, because it is exactly inverted from Perl: Go reads through missing nested keys safely *without* creating anything (no `exists`-vivifies gotcha), but writes need the path built explicitly. When porting a Perl structure-builder, the `if m[k] == nil { m[k] = ... }` dance is the honest translation of autovivification — or restructure to a flat map with a struct key and skip nesting entirely (`maps-of-slices`). Finally, nil compares only against nilable types: `x == nil` on an `int` is a compile error, so "is it defined?" is frequently a question Go makes unaskable — by design.

Further reading: https://go.dev/ref/spec#The_zero_value and https://go.dev/doc/faq#nil_error
