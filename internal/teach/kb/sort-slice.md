---
id: sort-slice
title: Sorting is a function call, and the default is numeric-aware
tags: [idiom, slices, sorting, ordering]
perl_triggers: [sort, sort-block, sort-numeric, sort-keys, reverse-sort, schwartzian-transform, cmp-operator, spaceship-operator]
severity: info
prerequisites: [slices-not-arrays]
---

Perl's `sort` is a builtin that returns a new list and defaults to string comparison, which is why `sort { $a <=> $b }` is muscle memory for every Perl programmer alive. Go's sorting lives in two packages, sorts *in place*, and is type-directed: `slices.Sort(xs)` needs no comparator at all because the element type already says whether these are numbers or strings. The two things to unlearn are the return value (there is none — the input slice is rearranged, so `sorted := slices.Sort(xs)` does not compile) and `$a`/`$b` (a Go comparator is a function parameter list, not a pair of package globals).

## The Perl you know

```perl
my @nums = (10, 9, 100, 1);
my @lex  = sort @nums;                  # 1, 10, 100, 9  — string order by default
my @num  = sort { $a <=> $b } @nums;    # 1, 9, 10, 100
my @desc = sort { $b <=> $a } @nums;

my @by_len = sort { length($a) <=> length($b) or $a cmp $b } @words;
my @keys   = sort keys %inventory;      # the stable-output reflex
```

`sort` returns a new list, leaves the original alone, and `$a`/`$b` arrive as package variables the block reads without declaring them.

## The Go you write

```go
package main

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type user struct {
	Name string
	Age  int
}

func main() {
	nums := []int{10, 9, 100, 1}
	slices.Sort(nums) // in place, numeric because the element type is int
	fmt.Println(nums)

	words := []string{"pear", "fig", "apple"}
	slices.Sort(words)
	fmt.Println(words)

	// Descending: reverse the comparison, not the slice.
	slices.SortFunc(nums, func(a, b int) int { return cmp.Compare(b, a) })
	fmt.Println(nums)

	// A comparator returns negative, zero, or positive — the <=> operator, spelled out.
	users := []user{{"raj", 41}, {"jane", 29}, {"ines", 41}}
	slices.SortFunc(users, func(a, b user) int {
		if c := cmp.Compare(b.Age, a.Age); c != 0 { // age descending
			return c
		}
		return strings.Compare(a.Name, b.Name) // then name ascending
	})
	fmt.Println(users)

	// sort keys %h, in one line.
	inventory := map[string]int{"plums": 9, "apples": 5, "figs": 1}
	for _, k := range slices.Sorted(maps.Keys(inventory)) {
		fmt.Printf("%s=%d ", k, inventory[k])
	}
	fmt.Println()

	// Searching a sorted slice: binary search, comma-ok style.
	i, found := slices.BinarySearch(words, "fig")
	fmt.Println(i, found)
}
```

```
[1 9 10 100]
[apple fig pear]
[100 10 9 1]
[{ines 41} {raj 41} {jane 29}]
apples=5 figs=1 plums=9 
1 true
```

`cmp.Compare(a, b)` is the direct translation of `<=>` and of `cmp`: it returns -1, 0, or +1, and works for any ordered type. Swapping the arguments reverses the order, which is cheaper and clearer than sorting and then reversing.

## The mismatch

The porting table is short. `sort @xs` → `slices.Sort(xs)` **plus a copy first if the caller still needs the original**, because Go sorts in place: `sorted := slices.Clone(xs); slices.Sort(sorted)`. `sort { $a <=> $b }` → nothing, the default already compares numerically for numeric element types; the block only survives translation when it encodes a real rule (a field, a case-insensitive key, a multi-level tiebreak). `sort { $a cmp $b }` → also nothing for `[]string`, though note Go compares by *byte* value, so uppercase sorts before lowercase and non-ASCII sorts by UTF-8 order — locale-aware collation is not in the standard library, and `golang.org/x/text/collate` is where you go when it matters. Two API generations coexist in real code: the older `sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })` takes a *less* function over indexes, while the modern `slices.SortFunc` (Go 1.21+) takes a *comparison* function over elements — read the signature before writing the body, because returning a bool where an int belongs is a type error and, worse, returning `a < b` from a comparator returns the wrong thing for equal elements. Stability is opt-in in both (`sort.SliceStable`, `slices.SortStableFunc`); the default sort is not stable, so equal elements may be reordered where Perl's merge sort would have kept them. Finally, the Schwartzian transform has no reason to survive: `slices.SortFunc` calls your comparator directly, so if the key is expensive, precompute a `[]struct{key K; val V}` and sort that — the same idea, written out.

Further reading: https://pkg.go.dev/slices#SortFunc and https://pkg.go.dev/cmp#Compare
