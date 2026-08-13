---
id: maps-of-slices
title: Hash-of-arrays, hash slices, and composite keys in Go
tags: [idiom, maps, slices]
perl_triggers: [hash-of-arrays, push-to-hash-elem, hash-slice, nested-hash-access, multidim-hash-key]
severity: info
prerequisites: [nil-slices-vs-nil-maps, nil-vs-undef]
---

The Perl workhorse `push @{$h{$k}}, $v` - autovivifying hash-of-arrays accumulation - has a Go translation that is *shorter* than you would fear, because of a happy collision of rules: reading a missing map key yields a nil slice, and appending to a nil slice works. So `m[k] = append(m[k], v)` is Go's one accidental autovivification, and it is completely idiomatic. Around it, though, two Perl conveniences vanish outright - hash slices have no syntax at all, and nested-hash lookups often deserve restructuring into a single map with a struct key.

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

## What the element type has to be

The rule that makes `m[k] = append(m[k], v)` work is worth stating on its own,
because it is half of a pair and the other half is a runtime panic. Reading a
missing key gives the zero value of the element type. For a slice, that is nil,
and `append` to a nil slice returns a new one, so the accumulation works with
no check. For a map, the zero value is also nil, and writing to a nil map
panics. Hash-of-arrays needs nothing; hash-of-hashes needs the inner map made
first.

The other thing Go asks that Perl does not is the element type itself, and it
has to be one type for the whole map. `map[string][]string` and
`map[string]map[string]int` are both perfectly ordinary; what is not ordinary,
though it is where a mechanical translation lands, is `map[string]any`, which
buys nothing and costs an assertion at every read.

Compiled and run as shown:

```go
package main

import (
	"fmt"
	"maps"
	"slices"
)

func main() {
	// A hash of arrays: one element type, decided here and checked everywhere.
	byOwner := map[string][]string{}
	for _, row := range [][2]string{{"ann", "disk"}, {"bob", "net"}, {"ann", "cpu"}} {
		byOwner[row[0]] = append(byOwner[row[0]], row[1])
	}
	for _, k := range slices.Sorted(maps.Keys(byOwner)) {
		fmt.Printf("%s: %v\n", k, byOwner[k])
	}

	// A hash of hashes needs the inner map made before anything goes in it.
	counts := map[string]map[string]int{}
	for _, pair := range [][2]string{{"web", "200"}, {"web", "404"}, {"web", "200"}} {
		if _, ok := counts[pair[0]]; !ok {
			counts[pair[0]] = map[string]int{}
		}
		counts[pair[0]][pair[1]]++
	}
	fmt.Println(counts["web"]["200"], counts["web"]["404"])

	// The asymmetry in one line: append to a missing slice is fine,
	// writing to a missing map is not.
	var missing map[string]int
	fmt.Println(len(missing), missing["anything"])
}
```

```
ann: [disk cpu]
bob: [net]
2 1
0 0
```

Reading from a nil map is legal and gives the zero value, which is why the last
line prints two zeros rather than crashing. Only writing panics. That
asymmetry is worth memorising, because the failure arrives in production rather
than at the first test.

A note for anything hand-written after a conversion: what the element type
should be is decided by what goes in, and the strongest evidence is usually the
`push`. A hash whose values are only ever appended to holds lists of whatever
was appended, and saying so at the declaration removes every assertion below
it.

## The counting hash, three levels down

`$matrix{$host}{$path}{$status}++` is one line of Perl and one of the commonest lines in any report script. It declares nothing: the `++` builds every level on the way down. In Go the shape has to be written at the top, and once it is, the body is barely longer than the Perl and has no assertions in it at all:

```go
package main

import (
	"fmt"
	"maps"
	"slices"
)

func main() {
	// host -> path -> status -> count, built entirely by ++.
	matrix := map[string]map[string]map[string]int{}
	rows := [][3]string{
		{"web1", "/index.html", "200"},
		{"web1", "/index.html", "200"},
		{"web1", "/login", "302"},
		{"db1", "/health", "500"},
	}
	for _, r := range rows {
		host, path, status := r[0], r[1], r[2]
		if matrix[host] == nil {
			matrix[host] = map[string]map[string]int{}
		}
		if matrix[host][path] == nil {
			matrix[host][path] = map[string]int{}
		}
		matrix[host][path][status]++
	}

	for _, host := range slices.Sorted(maps.Keys(matrix)) {
		for _, path := range slices.Sorted(maps.Keys(matrix[host])) {
			fmt.Println(host, path, matrix[host][path])
		}
	}

	// Reading is safe at every depth, even where nothing was ever stored:
	// each missing level yields a nil map, and reading a nil map is legal.
	fmt.Println(matrix["nowhere"]["nothing"]["404"], len(matrix["nowhere"]))
}
```

```
db1 /health map[500:1]
web1 /index.html map[200:2]
web1 /login map[302:1]
0 0
```

Three things are worth taking from that.

`if matrix[host] == nil` is the shorter guard, and it is available exactly because the value type is a map. The comma-ok form answers "was this key ever stored", which is a different question and three lines long; when the value type is a map or a slice, a missing key and a stored nil are the same thing to write into, so the nil test is both correct and shorter. Reach for comma-ok when the value type is a number or a string, where zero is a value someone might have stored on purpose.

The **innermost** type is where the whole declaration comes from. `++` says the leaves are numbers, `+=` on a fractional value says they are `float64`, `push` says they are slices. Every level above the leaf is a map of whatever the level below turned out to be, which is why one honest look at the innermost operation types the entire structure. `map[string]any` is what you get by refusing to take that look, and it costs an assertion at every step, on every line, forever.

Reading through the whole structure needs no guards. The last line asks for a count under two keys that were never stored and prints `0`: each missing level yields a nil map, reading a nil map is legal and yields the zero value, and only *writing* through one panics. Perl's read of the same path is the one that differs, because it quietly creates the intervening levels on the way through, and a hash that grew a key merely by being looked at has surprised Perl developers for thirty years.

## The mismatch

Why `m[k] = append(m[k], v)` works when `m[a][b] = v` panics (`nil-vs-undef`): the slice version *reads* `m[k]` (safe, yields nil), appends (nil-tolerant), and *assigns the result back to the top-level key* - every step is legal; the nested-map version tries to write through an inner nil map with no reassignment, which is the forbidden step. Corollary: the outer map itself must still be non-nil (`make` it - `nil-slices-vs-nil-maps`). Hash slices for *assignment* (`@h{qw(a b)} = (1,2)`) likewise become explicit loops or repeated assignments; there is no bulk syntax, and Go culture does not miss it. For nested structures, ask what the Perl was *for*: a two-level hash used as a set or counter keyed by two values is better as `map[hostPort]bool` - one allocation level, one lookup, comparable struct keys natively supported (any key type is fine if `==` works on it: strings, ints, arrays, pointer types, structs of comparable fields - but not slices, maps, or functions), and it replaces the old `$h{"$a$;$b"}` join-the-key hack exactly. Keep genuine nesting when the inner map is manipulated as a unit (handed out, iterated, deleted wholesale). One more absence: `values %h` for collecting slice contents has `maps.Values` (Go 1.23+, returns an iterator - `slices.Collect(maps.Values(m))` for a slice), again in random order.

Further reading: https://go.dev/blog/maps and https://go.dev/ref/spec#Map_types
