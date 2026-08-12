---
id: slice-aliasing-and-copy
title: Two slices, one backing array - the aliasing bug
tags: [trap, slices, aliasing, copy]
perl_triggers: [array-slice, array-copy, splice, arrayref-passing, list-assignment, hash-copy, hash-merge]
severity: trap
prerequisites: [slices-not-arrays]
---

`s[1:3]` does not copy anything: it returns a new slice header pointing into the *same* backing array, so writes through the sub-slice mutate the original — and, far worse, an `append` to the sub-slice that still fits within shared capacity silently overwrites elements of the parent that the sub-slice never contained. Perl's `@a[1..3]` returns fresh copies, so fifteen years of instinct says slicing is safe. In Go it is the language's premier data-corruption bug: no panic, no warning, just a value changing at a distance, often discovered three functions away.

## The Perl you know

```perl
my @lines = qw(boot auth disk net cron);
my @head  = @lines[0..1];   # COPIES: modifying @head never touches @lines
$head[0] = "BOOT";
# @lines is still (boot auth disk net cron)
```

List slices and `[@array]` copies mean aliasing requires deliberate reference-taking.

## The Go you write

Compiled and run as shown — read the third line of output twice:

```go
package main

import "fmt"

func main() {
	logLines := []string{"boot", "auth", "disk", "net", "cron"}

	head := logLines[0:2] // shares the backing array
	head[0] = "BOOT"
	fmt.Println(logLines) // parent modified

	// The classic append-into-parent bug:
	first3 := logLines[0:3] // len 3, but cap 5: room to grow IN PLACE
	fmt.Println(len(first3), cap(first3))
	first3 = append(first3, "EXTRA") // fits within cap: overwrites logLines[3]
	fmt.Println(logLines)

	// Fix 1: the three-index slice caps capacity at the slice's end.
	safe := logLines[0:3:3]
	safe = append(safe, "extra") // cap exceeded: reallocates, parent untouched
	fmt.Println(logLines)
	fmt.Println(safe)

	// Fix 2: explicit copy.
	dup := make([]string, len(logLines))
	copy(dup, logLines)
	dup[0] = "copied"
	fmt.Println(logLines[0], dup[0])
}
```

```
[BOOT auth disk net cron]
3 5
[BOOT auth disk EXTRA cron]
[BOOT auth disk EXTRA cron]
[BOOT auth disk extra]
BOOT copied
```

`"net"` is gone — replaced by `"EXTRA"` through a slice that ended two elements earlier. The three-index form `s[low:high:max]` sets capacity to `max-low`, forcing any future `append` to reallocate instead of trespassing. `slices.Clone(s)` (Go 1.21+) is the modern one-call spelling of Fix 2.

## The mismatch

Rewire the instinct completely: in Perl, slicing copies and you take references to share; in Go, slicing *shares* and you copy to isolate. Audit three patterns when porting. One: any function that receives a slice and appends to it may be corrupting the caller's data if the caller later uses the original — return the appended slice or copy first. Two: "keep the first N, drop the rest" (`@kept = @lines[0..$n]`) as `kept := lines[:n]` pins the *entire* original backing array in memory and stays writable through future appends — for long-lived results, clone. Three: `copy(dst, src)` copies only `min(len(dst), len(src))` elements and returns that count — copying into an empty (`len` 0) destination copies *nothing*, a quiet off-by-everything for people who expect `dst` to grow; `make` the destination with the right length first, as above. The compensation for all this danger: sub-slicing is O(1), which is why Go parsers and tokenisers slice with abandon — safely, because they treat slices as read-only views. Adopt that discipline: share for reading, clone for keeping or writing.

## The other direction: a sub that grows the caller's array

Perl's `sub add { my ($list, $v) = @_; push @$list, $v }` reaches the caller's array, because the reference *is* the array. The same function in Go receives a copy of the slice header, so the element it appends belongs to the function alone and the caller sees nothing. Nothing warns, the code compiles, and the caller's array is simply shorter than it should be. This is the most common Perl sub idiom there is, and it is the one place where "slices share their data" stops being true.

```go
package main

import "fmt"

// grows takes a copy of the slice header: the elements are shared, the length
// is not, so the caller never sees the element this adds.
func grows(xs []string) {
	xs = append(xs, "new")
	_ = xs
}

// returnsGrown is the shape to reach for first, and the one a Go developer
// expects to read.
func returnsGrown(xs []string) []string {
	return append(xs, "new")
}

// takesPointer is the shape when growing the caller's slice is the whole job.
func takesPointer(xs *[]string) {
	*xs = append(*xs, "new")
}

// writeFirst changes an element the slice already has, which does reach the
// caller. That is the half that makes the other half surprising.
func writeFirst(xs []string) {
	if len(xs) > 0 {
		xs[0] = "CHANGED"
	}
}

func main() {
	a := []string{"one"}
	grows(a)
	fmt.Println("after grows:      ", a)

	a = returnsGrown(a)
	fmt.Println("after returnsGrown:", a)

	takesPointer(&a)
	fmt.Println("after takesPointer:", a)

	writeFirst(a)
	fmt.Println("after writeFirst: ", a)
}
```

```
after grows:       [one]
after returnsGrown: [one new]
after takesPointer: [one new new]
after writeFirst:  [CHANGED new new]
```

Prefer the returning form. `*[]T` is correct and is what the standard library reaches for only rarely, because a function that both takes and returns the slice reads at the call site as what it is: `list = add(list, v)` says the list changed, where `add(&list, v)` says it only if you know the signature. The pointer form earns its place when the slice is a field being appended to from several methods, and at that point the honest move is usually a small type with an `Add` method on a pointer receiver.

## The plainest case, and the one that gets ported first

Before any of the slicing subtleties, there is the assignment:

```perl
my @copy = @original;    # a copy. Writing to @copy leaves @original alone.
my %adjusted = %prices;  # also a copy, and also shallow.
```

Written straight across, `copy := original` and `adjusted := prices` are not copies at all. A slice assignment copies the three-word header and leaves both names pointing at one backing array; a map assignment copies a pointer, because a Go map *is* a reference. In both cases a write through either name is visible through the other, immediately, with nothing said about it. It is the same bug as the slicing one and it arrives earlier, because assigning one array to another is the first thing a ported script does.

The fix is one call each, and both are shallow:

```go
package main

import (
	"fmt"
	"maps"
	"slices"
)

func main() {
	original := []string{"apple", "banana"}
	prices := map[string]int{"apple": 1}

	kept := slices.Clone(original)
	adjusted := maps.Clone(prices)

	kept[0] = "APPLE"
	adjusted["apple"] = 99
	fmt.Println(original[0], kept[0], prices["apple"], adjusted["apple"])
}
```

```
apple APPLE 1 99
```

Shallow matters. `slices.Clone` of a `[]*Job` gives you a new slice of the *same* jobs, so writing `copy[0].Name = "x"` still changes what the original's first element points at. Perl's `my @copy = @original` had exactly the same property for a list of references, so this is one place the instinct carries over intact: copying the container is not copying what is in it.

A related shape worth recognising: `my %merged = (%base, extra => 1)` has no Go spelling as a literal, because Go has no splicing into a composite. It becomes a clone followed by ordinary assignment, which is two lines and makes the shallowness visible:

```go
package main

import (
	"fmt"
	"maps"
	"slices"
)

func main() {
	base := map[string]int{"a": 1, "b": 2}
	merged := maps.Clone(base)
	merged["extra"] = 3
	fmt.Println(slices.Sorted(maps.Keys(merged)))
}
```

```
[a b extra]
```

Further reading: https://go.dev/blog/slices-intro and https://pkg.go.dev/slices#Clone
