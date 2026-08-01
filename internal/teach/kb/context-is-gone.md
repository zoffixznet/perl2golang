---
id: context-is-gone
title: Context is gone, so every expression means one thing
tags: [core, context, lists, arrays, mindset]
perl_triggers: [scalar-array, numeric-context, list-assignment, list-slice, paren-grouping, comma-operator, wantarray, context-sensitive-return]
severity: info
prerequisites: [static-types-and-zero-values, slices-not-arrays]
---

Context is the piece of Perl with no Go counterpart at all, and it is the one you will miss last and trip over first. In Perl an expression does not have a value until you know where it sits: `@items` is four things on the right of `my @copy` and the number 4 on the right of `my $n`. Go has no such rule. An expression has a type, that type is fixed at compile time, and a slice never quietly becomes its own length. So every place your Perl was leaning on context, the Go says out loud which of the two meanings it wanted, and nine times out of ten that word is `len`.

## The Perl you know

```perl
my @items = ("pen", "cup", "map", "key");

my $count = @items;             # scalar context: 4
my @copy  = @items;             # list context: four strings
print "there are " . @items . " items\n";   # . imposes scalar context: 4

my $q = ($n - $m) / $d;         # parentheses that only group
my @three = (1, 2, 3);          # parentheses that build a list

my ($first) = @items;           # list assignment: "pen"
my $first   = @items;           # scalar assignment: 4

my $fields = () = split /:/, $line;   # the counting idiom
```

The last five lines are the whole difficulty in miniature: the same characters, `( )`, mean grouping or a list depending only on what surrounds them, and `my ($x) = ...` and `my $x = ...` differ by two characters and give completely different answers.

## The Go you write

```go
package main

import (
	"fmt"
	"strings"
)

func main() {
	items := []string{"pen", "cup", "map", "key"}

	// my $count = @items;      an array where a number is wanted
	count := len(items)
	fmt.Printf("there are %d items\n", count)

	// my $q = ($n - $m) / $d;  parentheses that only group
	n, m, d := 13, 3, 5
	q := float64(n-m) / float64(d)
	fmt.Printf("q = %g\n", q)

	// my @fields = split /:/, $line;   and then its count, separately
	fields := strings.Split("root:x:0:0", ":")
	fmt.Printf("%d fields, third is %q\n", len(fields), fields[2])

	// my ($first) = @items;    versus   my $first = @items;
	first := items[0]
	fmt.Printf("first = %s, count again = %d\n", first, len(items))
}
```

```text
there are 4 items
q = 2
4 fields, third is "0"
first = pen, count again = 4
```

Notice what happened to the parentheses. In `float64(n-m)` they group, exactly as they did in Perl, and nothing else. There is no other reading available: Go has no syntax that turns `(n - m)` into a one-element anything, so the ambiguity you were carrying disappears rather than being resolved.

A list slice is the case that most often survives translation looking correct and behaving wrongly. Perl's `(sort @names)[0]` is a single value; the list it indexes never gets a name. Go indexes something it already has, so the list becomes a variable first:

```go
package main

import (
	"fmt"
	"sort"
	"strings"
)

// lines returns the parts. A caller who wants the count calls len on the
// result, which is the only thing Perl's scalar context was ever saying.
func lines(text string) []string {
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

func main() {
	text := "alpha\nbeta\ngamma\n"

	parts := lines(text)
	fmt.Println(len(parts), parts[0])

	// (sort @names)[0] has to become a named list and an index.
	names := []string{"delta", "alpha", "charlie"}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	lowest := sorted[0]
	highest := sorted[len(sorted)-1]
	fmt.Println(lowest, highest)
}
```

```text
3 alpha
alpha delta
```

Two habits fall out of that. Negative indices are not indices in Go: `[-1]` is written `[len(xs)-1]`, and it panics on an empty slice where Perl would have handed you `undef`, so the emptiness has to be either checked or known. And a function returning a list returns exactly one thing, a slice, whatever the caller wanted; the caller asks for `len` when it wanted a count. Perl's `wantarray`, which let one sub answer differently depending on how it was called, has no expression in Go at all: it becomes two functions with two names, and the call sites choose.

## The mismatch

Context is not a feature Go left out, it is a question Go never asks, so the porting work is to decide once, at each site, what the Perl meant and write that down. Practical audit list for translated code: every `my $n = @a` is `len(a)`; every `.` or arithmetic operator with an array on either side is a `len` too, because those operators impose scalar context and it is easy to read the Perl as concatenating the elements; `my ($x) = f()` is `f()[0]` with an emptiness check, while `my $x = f()` is `len(f())`; `(LIST)[i]` needs the list named before it can be indexed; `my $n = () = LIST` is `len` of the list and nothing more exotic; and `wantarray` is a design change, not a translation. The compiler catches the ones where the types disagree and says nothing about the ones where they happen to line up, which is why the `len` cases are worth going through by hand.

Further reading: https://perldoc.perl.org/perldata#Context and https://go.dev/ref/spec#Length_and_capacity
