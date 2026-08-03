---
id: collections-hold-one-type
title: A slice holds one type, and []int is not []any
tags: [trap, types, slices, maps, generics]
perl_triggers: [mixed-array, array-of-any, hash-of-mixed, list-of-lists, arg-list]
severity: trap
prerequisites: [slices-not-arrays, type-assertions-and-switches]
---

A Perl array holds whatever you put in it, so `(1, "two", [3])` is an ordinary list and passing it anywhere is free. A Go slice has one element type, chosen at its declaration, and the trap is not that rule but its consequence: `[]int` and `[]any` are *unrelated types*, and Go will not convert between them even though every `int` is an `any`. The same holds for `map[string]string` against `map[string]any`, and for `[]int` against `[]float64`. The compiler's refusal reads like pedantry the first time; it is a statement about memory layout, and the only way across is a copy.

## The Perl you know

```perl
my @ports = (80, 443);
describe(@ports);            # any sub takes any list

my @mixed = (1, "two", 3);
my $total = 0;
$total += $_ for @mixed;     # 4: the string reads as 0
```

Nothing here has a type to disagree about, so nothing does.

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

// widen copies a slice of one type into one that holds anything.
func widen[T any](xs []T) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

// narrow copies the other way, asserting each element.
func narrow[T any](xs []any) []T {
	out := make([]T, len(xs))
	for i, x := range xs {
		out[i], _ = x.(T)
	}
	return out
}

func describe(vs []any) string { return fmt.Sprint(vs...) }

func main() {
	ports := []int{80, 443}

	// describe(ports) does not compile; the copy is the price of the change.
	fmt.Println(describe(widen(ports)))

	mixed := []any{1, "two", 3}
	back := narrow[int](mixed)
	fmt.Println(back, len(back))

	// The copy really is a copy: the two are independent afterwards.
	widened := widen(ports)
	widened[0] = 8080
	fmt.Println(ports[0], widened[0])
}
```

```
80 443
[1 0 3] 3
80 8080
```

The commented-out line is the whole lesson, and it is worth seeing it fail:

```go-invalid
package main

import "fmt"

func describe(vs []any) { fmt.Println(vs...) }

func main() {
	ports := []int{80, 443}
	describe(ports)
}
```

```
./sample.go:9:11: cannot use ports (variable of type []int) as []any value in argument to describe
```

Two things in the run are worth pausing on. `narrow[int]` turned `"two"` into `0`, not into an error: a failed assertion in the comma-ok form yields the zero value, so a genuinely mixed collection loses its odd elements quietly. And the explicit `[int]` is needed because the element type appears only in the result, where Go has nothing to infer it from.

## The mismatch

The reflex to build is: decide the element type at the declaration and keep it. Reaching for `[]any` "so it can hold anything" is the Perl habit, and it costs you the compiler's help at every use, an assertion at every read, and a copy at every boundary where something wants a concrete type. `[]any` is right for genuinely heterogeneous data — a decoded JSON document, a row of database values, `fmt.Println`'s own parameter — and wrong for a list that happens to have been built out of strings.

When the elements really differ but share behaviour, the Go answer is not `any` but an interface: declare `[]Shape` and let each element be whatever implements it (`implicit-interfaces`). When they differ only in numeric width, pick the wider one at the declaration rather than converting later, since `[]int` to `[]float64` is a copy too. When one function wants `[]int` and another wants `[]any`, change the declaration rather than converting at the call, because every conversion is a fresh slice and the two stop tracking each other from that moment on (`slice-aliasing-and-copy`).

One thing that does not need a copy: a *variadic* call. `fmt.Println(vs...)` requires `vs` to be `[]any` already, but `fmt.Println(a, b, c)` with three different types is fine, because each argument is converted to `any` on its own. That asymmetry catches everyone once.

Further reading: https://go.dev/doc/faq#convert_slice_of_interface
