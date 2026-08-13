---
id: slice-surgery
title: Removing, inserting and replacing inside a slice
tags: [idiom, slices, stdlib]
perl_triggers: [splice, shift, unshift, pop, push, array-truncate, delete-array-element]
severity: info
prerequisites: [slices-not-arrays, slice-aliasing-and-copy]
---

`splice` is Perl's one tool for editing the middle of an array: it removes a run, puts something else in its place, changes the array's length, and hands back what it took, all in a single call. Go's `slices` package splits that into three functions, none of which returns the removed part, and adds one rule that catches every newcomer: a function handed a slice can change its elements but cannot change its caller's length. Learn the three functions and that rule and every `splice` you meet has an obvious shape.

## The Perl you know

```perl
my @queue = qw(a b c d e);

my @taken = splice(@queue, 0, 2);       # remove and keep what came out
splice(@queue, 1, 0, 'x', 'y');         # insert without removing
splice(@queue, 0, 2, 'one');            # replace two with one
splice(@queue, 3) if @queue > 3;        # truncate
```

The offsets are forgiving: negative counts from the end, past the end lands at the end, and nothing dies.

## The Go you write

Compiled and run as shown:

```go
package main

import (
	"fmt"
	"slices"
)

// take removes count elements from *xs at offset and returns them, which is
// the one operation the standard library does not have.
func take[T any](xs *[]T, offset, count int) []T {
	list := *xs
	removed := slices.Clone(list[offset : offset+count])
	*xs = slices.Delete(list, offset, offset+count)
	return removed
}

func main() {
	queue := []string{"a", "b", "c", "d", "e"}

	// Remove: the third argument is the end, not a length.
	queue = slices.Delete(queue, 1, 3)
	fmt.Println(queue)

	// Insert: at an index, any number of values.
	queue = slices.Insert(queue, 1, "x", "y")
	fmt.Println(queue)

	// Replace: a run of any length by a run of any other length.
	queue = slices.Replace(queue, 0, 2, "one")
	fmt.Println(queue)

	// Truncate: reslicing, which keeps the underlying array.
	queue = queue[:2]
	fmt.Println(queue, len(queue), cap(queue))

	// And the one that hands back what it removed.
	rest := []int{1, 2, 3, 4, 5}
	got := take(&rest, 1, 2)
	fmt.Println(got, rest)
}
```

```
[a d e]
[a x y d e]
[one y d e]
[one y] 2 5
[2 3] [1 4 5]
```

Four things in there are worth naming.

**Every one of them returns a new slice, and you must assign it back.** `slices.Delete(queue, 1, 3)` on its own compiles, does part of the work, and leaves `queue` with its original length, which is one of the quietest bugs in the language. `go vet` catches the standard-library cases; the habit of writing `x = f(x, ...)` catches the rest.

**The bounds are a half-open range, not an offset and a length.** `slices.Delete(q, 1, 3)` removes two elements, at indices 1 and 2. `splice(@q, 1, 3)` removes three. Translating one to the other is `end = offset + length`, every time.

**They panic on bad bounds.** There is no negative index, no clipping at the end, and no forgiveness: `q[len(q)+1]` is a runtime panic, not an empty result. Perl's tolerance has to become explicit arithmetic before the call.

**`take` needs a pointer, and that is not fussiness.** A slice value is a header - a pointer to an array, a length, and a capacity - passed by value like everything else in Go. A function can write through the pointer into the same elements, which is why `func clear(xs []int)` works, but assigning a shorter slice to its own parameter changes nothing the caller can see. When the length has to change, either return the new slice or take a `*[]T`.

## The mismatch

The three simple cases have simpler answers than the general ones, and reaching for `slices` when you do not need it is its own noise. `pop` is `x, queue = queue[len(queue)-1], queue[:len(queue)-1]`. `shift` is `x, queue = queue[0], queue[1:]`, which costs nothing and keeps the underlying array. `push` is `append`. Truncating is `queue[:n]`.

Two aliasing traps come with that convenience, both of them from `slice-aliasing-and-copy`. Reslicing does not free anything: `queue = queue[:2]` keeps the whole original array alive, elements two onward included, which matters when the elements are large or hold pointers. And `slices.Delete` shifts the tail down inside the existing array, so any other slice that was looking at the same array sees the change, and the elements past the new end are left as they were unless you zero them yourself.

Finally, the honest reason the helper above exists: nothing in the standard library both edits and reports. If you find yourself needing the removed part, write the four lines once and give them a name, rather than clone-and-delete at every call site.

Further reading: https://pkg.go.dev/slices#Delete
