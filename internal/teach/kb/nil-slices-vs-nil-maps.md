---
id: nil-slices-vs-nil-maps
title: A nil slice works; writing to a nil map panics
tags: [trap, nil, slices, maps]
perl_triggers: [my-array-declaration, my-hash-declaration, push, hash-element-assignment]
severity: trap
prerequisites: [nil-vs-undef, slices-not-arrays]
---

In Perl, `my @list` and `my %hash` are both immediately usable. In Go, their zero values are both `nil` but behave completely differently: a nil slice is fully usable — `append`, `len`, `range` all work and allocation happens lazily — while a nil map supports reads but *panics on the first write*. Since `var m map[string]int` looks exactly as innocent as `my %hash`, the direct transliteration of any Perl sub that declares a hash and fills it crashes at runtime. This asymmetry is arbitrary-feeling, permanent, and must simply be memorised.

## The Perl you know

```perl
my @tags;
push @tags, "prod";     # obviously fine

my %count;
$count{x} = 1;          # equally obviously fine
```

Declaration and readiness are the same thing.

## The Go you write

One program, run as shown — everything works until the map write:

```go-fails
package main

import "fmt"

func main() {
	var tags []string // nil slice: the useful kind of nil
	fmt.Println(len(tags), tags == nil)

	tags = append(tags, "prod") // appending to nil allocates on demand
	fmt.Println(tags)

	var none []int
	for _, v := range none { // ranging over nil: zero iterations, no error
		fmt.Println(v)
	}

	var m map[string]int // nil map
	fmt.Println(len(m), m["missing"]) // reading a nil map is fine

	m["x"] = 1 // writing is not: this panics
	fmt.Println("never reached")
}
```

```
0 true
[prod]
0 0
panic: assignment to entry in nil map

goroutine 1 [running]:
main.main()
	/.../nilslicemap.go:20 +0x1a5
exit status 2
```

The fix is one of two initialisations: `m := make(map[string]int)` or a literal `m := map[string]int{}`. For slices, `var s []T` is *preferred* over `s := []T{}` — the nil slice is idiomatic precisely because it works.

## The mismatch

The porting rule: every Perl `my %h` that is subsequently written to becomes `make(map[...]...)`, no exceptions — including maps nested inside maps (`nil-vs-undef` shows the nested variant) and map fields inside structs, whose zero value is nil and which need a constructor or lazy `if m == nil` guard. Meanwhile resist "fixing" nil slices: `var s []T` appends fine, and the only observable difference from `[]T{}` is `s == nil` and JSON encoding (`null` versus `[]` — a real API-contract concern, see `encoding-json`). Why the asymmetry? `append` returns a new slice value, so it can replace nil with a freshly allocated backing array in the caller's variable; a map write mutates in place through the map reference, and a nil map has no place to mutate — Go chose a panic over hidden allocation. Unsatisfying, coherent, and memorable after your first crash, which this file has now given you for free.

Further reading: https://go.dev/blog/maps
