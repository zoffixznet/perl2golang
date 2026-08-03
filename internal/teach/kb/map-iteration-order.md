---
id: map-iteration-order
title: Map order is randomised per loop, on purpose
tags: [gotcha, maps, iteration, ordering]
perl_triggers: [keys, values, each, sort-keys, delete-in-loop]
severity: warning
prerequisites: [comma-ok-idiom, sort-slice]
---

You already know hashes are unordered — Perl has randomised per-process since 5.18 — but Go goes a step further that will break a specific class of ported code: iteration order is randomised *per range loop*, so two loops over the same untouched map in the same process can visit keys in different orders. Perl code that iterates `keys %h` twice and relies on getting the same sequence both times (building parallel arrays, pairing output across loops) is subtly broken in Go even though it happens to work in any single Perl process.

## The Perl you know

```perl
my %inv = (apples=>5, pears=>2, plums=>9, figs=>1, dates=>7, limes=>3);
say join ",", keys %inv for 1..2;
# apples,dates,plums,pears,limes,figs
# apples,dates,plums,pears,limes,figs   <- same order within one process

for my $k (sort keys %inv) { ... }      # the stable-output reflex you already have
```

## The Go you write

Compiled and run as shown — note run 1 versus runs 2 and 3:

```go
package main

import (
	"fmt"
	"maps"
	"slices"
)

func main() {
	inventory := map[string]int{
		"apples": 5, "pears": 2, "plums": 9, "figs": 1, "dates": 7, "limes": 3,
	}

	for run := 1; run <= 3; run++ {
		fmt.Printf("run %d: ", run)
		for k := range inventory {
			fmt.Print(k, " ")
		}
		fmt.Println()
	}

	// Stable output: sort the keys.
	for _, k := range slices.Sorted(maps.Keys(inventory)) {
		fmt.Printf("%s=%d ", k, inventory[k])
	}
	fmt.Println()
}
```

```text
run 1: figs dates limes apples pears plums 
run 2: apples pears plums figs dates limes 
run 3: apples pears plums figs dates limes 
apples=5 dates=7 figs=1 limes=3 pears=2 plums=9 
```

Randomised means randomised — identical consecutive orders can occur by chance, as runs 2 and 3 show; the differing run 1 is the guarantee you cannot rely on any of them. `slices.Sorted(maps.Keys(m))` (Go 1.23+) is the modern one-liner for the `sort keys %h` reflex; on older codebases you will see the collect-keys-then-`sort.Strings` loop spelled out.

Deleting while iterating, the other half of this topic, is explicitly legal and does what you hope — run as shown:

```go
package main

import "fmt"

func main() {
	sessions := map[string]bool{"a": true, "b": false, "c": true, "d": false}
	for id, active := range sessions {
		if !active {
			delete(sessions, id) // explicitly allowed by the spec
		}
	}
	fmt.Println(len(sessions), sessions)
}
```

```
2 map[a:true c:true]
```

The spec's exact contract: entries deleted during iteration will not be produced later; entries *added* during iteration may or may not be — so mutate-while-ranging is fine for deletion, hazardous for insertion. (Go 1.21+ also has `maps.DeleteFunc(m, f)` for exactly this pattern; `clear(m)` empties a map outright.)

## What happened to each

`each` is the other half of Perl's hash iteration, and it works differently
from `keys`: it hands back one pair per call, and it does so by keeping a
cursor *inside the hash itself*. That is why abandoning an `each` loop halfway
leaves the next one starting from the middle, why two `each` loops over the
same hash interfere, and why `keys %h` in the middle of one resets it. None of
that is documentation trivia; it is a class of bug that only shows up when the
data gets big enough for the early `last` to trigger.

Go has no equivalent and needs none. `for k, v := range m` gives both halves in
the loop header, keeps no state anywhere, and starts at the beginning every
single time.

Compiled and run as shown:

```go
package main

import (
	"fmt"
	"maps"
	"slices"
)

func main() {
	stock := map[string]int{"apples": 10, "pears": 4, "plums": 0}

	// Both halves of the pair come out of the loop header, and nothing is
	// remembered anywhere, so this loop always starts at the beginning.
	total := 0
	for _, v := range stock {
		total += v
	}
	fmt.Println("total:", total)

	// Two loops over the same map are independent. Abandoning the first
	// leaves nothing behind for the second to resume from.
	visited := 0
	for range stock {
		visited++
		break
	}
	for range stock {
		visited++
	}
	fmt.Println("visited:", visited)

	// When the order is part of the output, say so.
	for _, k := range slices.Sorted(maps.Keys(stock)) {
		fmt.Printf("[%s=%d]", k, stock[k])
	}
	fmt.Println()
}
```

```
total: 14
visited: 4
[apples=10][pears=4][plums=0]
```

`visited` is 4 because the second loop saw all three entries with nothing
carried over from the first. The Perl equivalent written with `each` would have
printed 3, and finding out why costs an afternoon.

One translation note. A `while (my ($k, $v) = each %h)` loop becomes a `range`
directly, and the only behaviour that goes missing is the resumption, which
almost no program wanted. A bare `each %h` outside a loop is a different
matter: it is a call that advances a cursor, and there is nothing in Go it can
become.

## The mismatch

The Go runtime randomises order specifically so nobody can ship code that accidentally depends on it — it is a deliberate compatibility-protection device, the same reasoning as Perl 5.18's hash randomisation but applied per iteration rather than per process. Practical audit list for ported code: any test asserting on the serialised form of a ranged map (fix: sort keys, or rely on `encoding/json`, which sorts map keys itself — `encoding-json`); any pair of loops assumed to align; any "first key" grab (`(keys %h)[0]`) — meaningless in Go, and its randomness will faithfully expose that it was meaningless in Perl too. When insertion order itself must be preserved, a map cannot do it: keep a companion `[]string` of keys in insertion order, the honest equivalent of `Tie::IxHash`.

Further reading: https://go.dev/blog/maps and https://go.dev/ref/spec#For_statements
