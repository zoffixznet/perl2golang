---
id: range-is-not-foreach
title: range gives you the index first, and the element is a copy
tags: [gotcha, slices, loops, range]
perl_triggers: [foreach, grep, map-block, topic-modification, while-each]
severity: warning
prerequisites: [slices-not-arrays]
---

Two habits from `foreach` will produce wrong Go on day one. First: `for x := range items` binds the *index*, not the element — the direct transliteration of `for my $x (@list)` quietly iterates over `0, 1, 2, ...` instead of your data, and if the elements are ints it even type-checks. Second: the two-variable form's element is a *copy*, so assigning to it modifies nothing — where Perl's `foreach` famously aliases `$_` (and the loop variable) to the real element, making mutate-in-loop a standard Perl idiom that ports to a silent no-op.

## The Perl you know

```perl
my @fruits = qw(apple banana cherry);
for my $f (@fruits) {
    $f = "plum";        # ALIASED: @fruits is now plum, plum, plum
}
my @long = grep { length > 6 } @fruits;
my @upper = map { uc } @fruits;
```

The loop variable is the element; `grep` and `map` are builtins.

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

func main() {
	fruits := []string{"apple", "banana", "cherry"}

	// One-variable range gives the INDEX, not the element:
	for f := range fruits {
		fmt.Println(f)
	}

	// Index and element:
	for i, f := range fruits {
		fmt.Println(i, f)
	}

	// The loop variable is a copy: this modifies nothing.
	for _, f := range fruits {
		f = "PLUM"
		_ = f
	}
	fmt.Println(fruits)

	// To modify elements, index into the slice:
	for i := range fruits {
		fruits[i] = "plum-" + fruits[i]
	}
	fmt.Println(fruits)

	// No grep/map builtins; the loop IS the idiom:
	var long []string
	for _, f := range fruits {
		if len(f) > 10 {
			long = append(long, f)
		}
	}
	fmt.Println(long)

	// range over an int (Go 1.22+):
	for i := range 3 {
		fmt.Print(i, " ")
	}
	fmt.Println()
}
```

```
0
1
2
0 apple
1 banana
2 cherry
[apple banana cherry]
[plum-apple plum-banana plum-cherry]
[plum-banana plum-cherry]
0 1 2 
```

## The mismatch

The mechanical translations: `for my $f (@list)` → `for _, f := range list` (the `_` discards the index you did not ask for — writing `for f := range list` is the classic conversion bug); `for my $i (0..$#list)` → `for i := range list`; `for (1..10)` → `for i := 1; i <= 10; i++` or `for range 10` when the counter is unused (Go's only loop keyword is `for`; it plays `while` as `for cond {}` and `until`/`loop` as `for {}`). Mutation in place is always by index: `fruits[i] = ...`. For `grep`/`map`/`first`, the append-loop above is the culturally accepted answer — Go deliberately shipped no map/grep over slices even after generics made it possible, though `slices.ContainsFunc`, `slices.IndexFunc`, and `slices.DeleteFunc` cover common `grep`-adjacent cases; chains of transformations become sequential loops, more lines and measurably clearer stack traces. Ranging over a map gives key (one variable) or key, value (two) in random order (`map-iteration-order`); over a string, byte-offset and rune (`strings-are-bytes`); there is no `each`-style stateful iterator to leak state between loops.

Further reading: https://go.dev/ref/spec#For_statements
