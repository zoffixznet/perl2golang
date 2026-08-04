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

## Growing to fit a write, which Perl did without being asked

`$a[5] = 'f'` on a three-element array is not an error in Perl: the array becomes six long and indices 3 and 4 hold undef. The equivalent Go line panics, because a slice's length is part of what it is and an index outside it is out of range. Making the room is a step of its own, and the same is true of `$#a = N`, which sets the length in both directions where reslicing alone can only reach as far as the capacity happens to go:

```go
package main

import "fmt"

// grow returns xs made at least n long, with anything it adds left at the
// zero value of T.
func grow[T any](xs []T, n int) []T {
	if n <= len(xs) {
		return xs
	}
	return append(xs, make([]T, n-len(xs))...)
}

// at reads xs[i], or the zero value of T when there is no such element.
func at[T any](xs []T, i int) T {
	if i < 0 || i >= len(xs) {
		var missing T
		return missing
	}
	return xs[i]
}

func main() {
	slot := []string{"a", "b", "c"}

	// $slot[5] = 'f' -- the room has to be made first.
	slot = grow(slot, 6)
	slot[5] = "f"
	fmt.Println(len(slot), slot)

	// $slot[20] -- out of range, and reading it changes nothing.
	fmt.Printf("%q %d\n", at(slot, 20), len(slot))

	// $slot[-1] -- a negative index is not an index at all here.
	fmt.Println(slot[len(slot)-1])

	// $#slot = 1 truncates; $#slot = 3 re-extends. One expression does both,
	// because the growth runs first and the reslice then fixes the length.
	slot = grow(slot, 2)[:2]
	fmt.Println(len(slot), slot)
	slot = grow(slot, 4)[:4]
	fmt.Println(len(slot), slot)
}
```

```
6 [a b c   f]
"" 6
f
2 [a b]
4 [a b  ]
```

Three things in that sample are worth keeping. `at` exists because reading past the end is undef in Perl and a panic here, and a program that dies on a short line teaches nothing about the lines below it; where the element is known to be there, `xs[i]` says so and is the better line. A negative index is not a Perl-style count from the end but a compile error for a constant and a panic for a variable, so `xs[len(xs)-1]` is the spelling, on the left of an assignment as much as on the right. And the gaps a growth opens hold the zero value, which is 0 or the empty string and not "nothing": where the difference matters, the element type has to be `[]*T` so that a gap is nil (`nil-vs-undef`).

The bigger question the growth raises is whether the data structure is right. An array written at arbitrary computed indices is a sparse table, and in Go that is usually `map[int]T`: no growth, no gaps, no panic, and the two-result read answers exactly what `defined` was asking (`comma-ok-idiom`).

## Two dimensions, where the growth happens at every level

`my @d; $d[$i][$j] = 0;` is the shape every dynamic-programming table, grid and matrix in Perl is written in, and it is doing three invisible things: extending `@d` to reach `$i`, putting a fresh array reference there, and extending *that* to reach `$j`. Go's `[][]int` starts nil at both levels, so all three have to be written, outermost first. The growth is assigned back at each level for the same reason `append`'s result is: growing may have to move the data.

```go
package main

import "fmt"

func grow[T any](xs []T, n int) []T {
	if n <= len(xs) {
		return xs
	}
	return append(xs, make([]T, n-len(xs))...)
}

func main() {
	a, b := []rune("kitten"), []rune("sitting")

	// my @d; $d[$i][$j] = ... -- both levels are made by the write.
	var d [][]int
	for i := 0; i <= len(a); i++ {
		d = grow(d, i+1)
		d[i] = grow(d[i], 1)
		d[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		d[0] = grow(d[0], j+1)
		d[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			best := d[i-1][j] + 1
			d[i] = grow(d[i], j+1)
			if d[i][j-1]+1 < best {
				best = d[i][j-1] + 1
			}
			if d[i-1][j-1]+cost < best {
				best = d[i-1][j-1] + cost
			}
			d[i][j] = best
		}
	}
	fmt.Println(d[len(a)][len(b)], len(d), len(d[0]))
}
```

```
3 7 8
```

Notice which reads in that loop are plain and which are not. `d[i-1][j]` needs no help: `i` counts from 1 to `len(a)` and the outer slice was grown to `len(a)+1` on the way in, so the element is certainly there. That is the general rule, and it is worth stating as a rule because it decides how the whole program reads: **an index is safe exactly when something in view bounds it.** A loop over `0 .. $#a` bounds its variable by `a`'s length, and so does `1 .. @a` with `$a[$i-1]`; an index that came from arithmetic, from input, or from a different array's length bounds nothing, and that read needs `at`. A converter that cannot tell the two apart has to choose between panicking programs and a call wrapped round every index expression, and neither is what a reader wants.

When the size is known before the loop, none of this is needed and the table should simply be made: `d := make([][]int, len(a)+1)` followed by a loop setting each `d[i] = make([]int, len(b)+1)`. That is the Go a reviewer expects, and the growth above is what a converter emits when the source never says how big the table is.

One more thing Perl hid in that loop: `0 .. @a` puts an array in *numeric context*, where it is the element count. It is `len(a)` and nothing else, which is the same answer `scalar @a` gives and the same one `if (@list > 3)` was asking for.

## The mismatch

The porting table: `push @a, $x` → `a = append(a, x)` (assignment mandatory — within capacity `append` writes in place and returns the same header; past capacity it copies everything to a bigger array and returns a *different* header, and code keeping the old one holds stale data); `pop` → `x := a[len(a)-1]; a = a[:len(a)-1]`; `shift`/`unshift` → re-slicing `a[1:]` or `append([]T{x}, a...)`, both of which should make you pause, because O(n) front operations that Perl hides are visible in Go — a genuine queue wants a different structure. `scalar @a` and `$#a` → `len(a)` and `len(a)-1`; there is no separate "last index" spelling and *negative indices do not exist* — `a[len(a)-1]`, never `a[-1]`, which is a compile error for constants and a runtime panic otherwise. `splice` has no single equivalent; the `slices` package (`slices.Insert`, `slices.Delete`) covers most uses. Pre-size with `make([]T, 0, n)` when you know `n` — the Perl habit of just pushing is fine, but this is the cheap optimisation Go reviewers expect in hot paths. Finally, when you see `[3]int` in real code it is usually deliberate value semantics or a fixed-size key (`maps-of-slices`); default to slices everywhere else.

Further reading: https://go.dev/blog/slices-intro
