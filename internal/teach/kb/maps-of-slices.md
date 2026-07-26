---
id: maps-of-slices
title: Hash-of-arrays, hash slices, and composite keys in Go
tags: [idiom, maps, slices]
perl_triggers: [hash-of-arrays, push-to-hash-elem, hash-slice, nested-hash-access, multidim-hash-key]
severity: info
prerequisites: [nil-slices-vs-nil-maps, nil-vs-undef]
---

The Perl workhorse `push @{$h{$k}}, $v` — autovivifying hash-of-arrays accumulation — has a Go translation that is *shorter* than you would fear, because of a happy collision of rules: reading a missing map key yields a nil slice, and appending to a nil slice works. So `m[k] = append(m[k], v)` is Go's one accidental autovivification, and it is completely idiomatic. Around it, though, two Perl conveniences vanish outright — hash slices have no syntax at all, and nested-hash lookups often deserve restructuring into a single map with a struct key.

## The Perl you know

```perl
my %by_owner;
push @{ $by_owner{$_->[0]} }, $_->[1] for @pets;   # autoviv HoA

my @vals = @config{qw(host port user)};            # hash slice

my %seen;
$seen{$host}{$port} = 1;                           # nested hash as a 2-key set
```

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

func main() {
	byOwner := map[string][]string{}

	pets := [][2]string{
		{"jane", "cat"}, {"raj", "dog"}, {"jane", "iguana"},
	}
	for _, p := range pets {
		owner, pet := p[0], p[1]
		// The nil zero value makes this work with no vivification dance:
		byOwner[owner] = append(byOwner[owner], pet)
	}
	fmt.Println(byOwner["jane"], byOwner["raj"])

	// No hash slices: @h{qw(host port user)} becomes a loop.
	config := map[string]string{"host": "db1", "port": "5432", "user": "app", "tmp": "x"}
	var picked []string
	for _, k := range []string{"host", "port", "user"} {
		picked = append(picked, config[k])
	}
	fmt.Println(picked)

	// Composite struct keys replace nested hashes when you only need lookup:
	type hostPort struct{ host, port string }
	seen := map[hostPort]bool{}
	seen[hostPort{"db1", "5432"}] = true
	fmt.Println(seen[hostPort{"db1", "5432"}], seen[hostPort{"db2", "5432"}])
}
```

```
[cat iguana] [dog]
[db1 5432 app]
true false
```

## The mismatch

Why `m[k] = append(m[k], v)` works when `m[a][b] = v` panics (`nil-vs-undef`): the slice version *reads* `m[k]` (safe, yields nil), appends (nil-tolerant), and *assigns the result back to the top-level key* — every step is legal; the nested-map version tries to write through an inner nil map with no reassignment, which is the forbidden step. Corollary: the outer map itself must still be non-nil (`make` it — `nil-slices-vs-nil-maps`). Hash slices for *assignment* (`@h{qw(a b)} = (1,2)`) likewise become explicit loops or repeated assignments; there is no bulk syntax, and Go culture does not miss it. For nested structures, ask what the Perl was *for*: a two-level hash used as a set or counter keyed by two values is better as `map[hostPort]bool` — one allocation level, one lookup, comparable struct keys natively supported (any key type is fine if `==` works on it: strings, ints, arrays, pointer types, structs of comparable fields — but not slices, maps, or functions), and it replaces the old `$h{"$a$;$b"}` join-the-key hack exactly. Keep genuine nesting when the inner map is manipulated as a unit (handed out, iterated, deleted wholesale). One more absence: `values %h` for collecting slice contents has `maps.Values` (Go 1.23+, returns an iterator — `slices.Collect(maps.Values(m))` for a slice), again in random order.

Further reading: https://go.dev/blog/maps and https://go.dev/ref/spec#Map_types
