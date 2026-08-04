---
id: range-is-not-foreach
title: range gives you the index first, and the element is a copy
tags: [gotcha, slices, loops, range]
perl_triggers: [foreach, grep, map-block, topic-modification, while-each, redo, loop-label, next-label, last-label]
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

## Draining a list, which is not a range loop at all

`while (defined(my $job = shift @queue))` looks like iteration and is not: it empties the list as it goes, and the `defined` is what keeps a queue holding a 0 from stopping early. Go writes it with the length as the condition, and taking the element becomes two plain statements at the top of the body:

```go
package main

import "fmt"

func main() {
	// while (defined(my $job = shift @queue)) -- the question is whether
	// there was an element, so a queue holding a 0 does not stop it.
	queue := []int{3, 0, 7}
	for len(queue) > 0 {
		job := queue[0]
		queue = queue[1:]
		fmt.Printf("took %d, %d left\n", job, len(queue))
	}

	// A worklist that grows while it drains. range cannot do this: it fixes
	// the length before the first iteration.
	work := []string{"a"}
	children := map[string][]string{"a": {"b", "c"}, "b": {"d"}}
	seen := map[string]bool{}
	var order []string
	for len(work) > 0 {
		node := work[0]
		work = work[1:]
		if seen[node] {
			continue
		}
		seen[node] = true
		order = append(order, node)
		work = append(work, children[node]...)
	}
	fmt.Println(order)
}
```

```
took 3, 2 left
took 0, 1 left
took 7, 0 left
[a b c d]
```

The second loop is why the idiom exists and why `range` cannot replace it: `range` evaluates the length once, before the first iteration, so appending to the slice inside the loop has no effect on how many times it runs. A worklist that grows as it is walked needs the length checked every time round, which is what `for len(work) > 0` does.

`queue = queue[1:]` moves the window forward without copying anything, so draining from the front is as cheap as draining from the back. What it does not do is release the elements already passed: the backing array is still reachable through the original allocation until the whole slice goes, which only matters when the elements are large and the queue is long-lived.

## The mismatch

The mechanical translations: `for my $f (@list)` → `for _, f := range list` (the `_` discards the index you did not ask for — writing `for f := range list` is the classic conversion bug); `for my $i (0..$#list)` → `for i := range list`; `for (1..10)` → `for i := 1; i <= 10; i++` or `for range 10` when the counter is unused (Go's only loop keyword is `for`; it plays `while` as `for cond {}` and `until`/`loop` as `for {}`). Mutation in place is always by index: `fruits[i] = ...`. For `grep`/`map`/`first`, the append-loop above is the culturally accepted answer — Go deliberately shipped no map/grep over slices even after generics made it possible, though `slices.ContainsFunc`, `slices.IndexFunc`, and `slices.DeleteFunc` cover common `grep`-adjacent cases; chains of transformations become sequential loops, more lines and measurably clearer stack traces. Ranging over a map gives key (one variable) or key, value (two) in random order (`map-iteration-order`); over a string, byte-offset and rune (`strings-are-bytes`); there is no `each`-style stateful iterator to leak state between loops.

## The third keyword, and the shape it forces

Perl has three loop keywords and Go has two. `next` is `continue`, `last` is `break`, and `redo` has no counterpart at all: it re-runs the body without advancing to the next element or re-testing the condition. The retry idiom it exists for is common enough to be worth knowing the translation for.

The body goes inside a loop of its own, and once it is wrapped an unlabelled `continue` or `break` means the *inner* loop, so any `next` or `last` in the same body has to name the outer one:

```go
package main

import "fmt"

func run(jobs []string) {
	attempts := map[string]int{}
eachJob:
	for _, job := range jobs {
		for { // redo continues this one
			attempts[job]++
			if job == "beta" && attempts[job] < 3 {
				continue // this is redo
			}
			if job == "skipme" {
				continue eachJob // this is next
			}
			if job == "stop" {
				break eachJob // and this is last
			}
			fmt.Println(job, attempts[job])
			break // what makes the inner loop run once by default
		}
	}
}

func main() { run([]string{"alpha", "beta", "skipme", "gamma", "stop", "never"}) }
```

```
alpha 1
beta 3
gamma 1
```

A Go label goes on the line above the loop, is written `Name:`, and can only appear after `break`, `continue` or `goto`. Go rejects a label nothing branches to, which is a small mercy: a wrapped body that never uses `next` or `last` needs no label at all.

Having written it out once, it is usually worth rewriting by hand. A retry is clearer as a counted inner loop (`for attempt := 1; attempt <= 3; attempt++`) than as a conditional `continue`, because the counter is then visible in the header instead of hidden in a map. The mechanical translation is what keeps the behaviour; the rewrite is what makes it Go.

Further reading: https://go.dev/ref/spec#For_statements
