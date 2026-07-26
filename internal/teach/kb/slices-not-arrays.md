---
id: slices-not-arrays
title: Slices are views with capacity, arrays are values
tags: [gotcha, slices, arrays, append]
perl_triggers: [my-array, push, pop, shift, unshift, scalar-array, last-index, splice]
severity: warning
prerequisites: [static-types-and-zero-values]
---

Perl's `@array` maps to Go's *slice* (`[]int`), not Go's array (`[3]int`) — an array in Go has its length baked into its type, is copied wholesale on assignment, and you will rarely declare one on purpose. A slice is a three-word header — pointer into a backing array, length, capacity — and that "capacity" concept has no Perl equivalent but drives real behaviour: `append` mutates in place while there is room and *reallocates to somewhere new* when there is not, which is why `append`'s result must always be assigned back and why two slices can unexpectedly share storage (`slice-aliasing-and-copy`).

## The Perl you know

```perl
my @a = (1, 2, 3);
my @b = @a;          # copies the elements (lists copy on assignment)
push @a, 4;          # grows invisibly, no result to assign
my $n = scalar @a;   # length; capacity is not a concept you have
```

Perl arrays grow implicitly and assignment copies contents — closer, ironically, to Go's *arrays* than to its slices.

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

func main() {
	// An array: fixed length, value semantics.
	a := [3]int{1, 2, 3}
	b := a // copies all three elements
	b[0] = 99
	fmt.Println(a, b)

	// A slice: a small header (pointer, length, capacity) over a backing array.
	s := []int{1, 2, 3}
	t := s // copies the header, SHARES the elements
	t[0] = 99
	fmt.Println(s, t)

	// append grows capacity in jumps, reallocating when full:
	var xs []int
	for i := 0; i < 9; i++ {
		xs = append(xs, i)
		fmt.Printf("len=%d cap=%d\n", len(xs), cap(xs))
	}
}
```

```
[1 2 3] [99 2 3]
[99 2 3] [99 2 3]
len=1 cap=4
len=2 cap=4
len=3 cap=4
len=4 cap=4
len=5 cap=8
len=6 cap=8
len=7 cap=8
len=8 cap=8
len=9 cap=16
```

(The exact growth pattern is a runtime implementation detail — never depend on it; depend only on "amortised constant append".) Forgetting to use `append`'s result does not even compile:

```go-invalid
package main

func main() {
	s := []int{1}
	append(s, 2)
}
```

```
./appenderr.go:5:2: append(s, 2) (value of type []int) is not used
```

## The mismatch

The porting table: `push @a, $x` → `a = append(a, x)` (assignment mandatory — within capacity `append` writes in place and returns the same header; past capacity it copies everything to a bigger array and returns a *different* header, and code keeping the old one holds stale data); `pop` → `x := a[len(a)-1]; a = a[:len(a)-1]`; `shift`/`unshift` → re-slicing `a[1:]` or `append([]T{x}, a...)`, both of which should make you pause, because O(n) front operations that Perl hides are visible in Go — a genuine queue wants a different structure. `scalar @a` and `$#a` → `len(a)` and `len(a)-1`; there is no separate "last index" spelling and *negative indices do not exist* — `a[len(a)-1]`, never `a[-1]`, which is a compile error for constants and a runtime panic otherwise. `splice` has no single equivalent; the `slices` package (`slices.Insert`, `slices.Delete`) covers most uses. Pre-size with `make([]T, 0, n)` when you know `n` — the Perl habit of just pushing is fine, but this is the cheap optimisation Go reviewers expect in hot paths. Finally, when you see `[3]int` in real code it is usually deliberate value semantics or a fixed-size key (`maps-of-slices`); default to slices everywhere else.

Further reading: https://go.dev/blog/slices-intro
