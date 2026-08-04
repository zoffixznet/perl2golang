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

## When context crosses a function boundary

The sharpest version of all this is a sub that returns a match:

```perl
sub parts { my ($t) = @_; return $t =~ /^(\w+)\s+(\d{4})$/ }

my ($name, $year) = parts($row);      # the two capture groups
if (parts($row))     { ... }          # a truth value
```

The parentheses that say "list" are at the call site, not at the match, so the same `return` yields two strings to one caller and a yes-or-no to the other, and the sub never learns which. That is context reaching across a function boundary, and it is the one thing in Perl that has no Go spelling at all: a Go function returns what it returns.

The signature to pick is the one that keeps both answers rather than the one that matches either caller exactly. Return the groups, and return `nil` when nothing matched:

```go
package main

import (
	"fmt"
	"regexp"
)

var nameYear = regexp.MustCompile(`^(\w+)\s+(\d{4})$`)

// parts returns the capture groups, or nil when the pattern did not match.
// One signature covers both the readings Perl left to the call site: nil is
// empty when the caller wants a list, and false when it wants a test.
func parts(text string) []string {
	m := nameYear.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	return m[1:]
}

func main() {
	for _, row := range []string{"ada 1815", "no year here"} {
		got := parts(row)

		// The list reading.
		name, year := "(none)", "(none)"
		if len(got) == 2 {
			name, year = got[0], got[1]
		}
		fmt.Printf("%-6s %s\n", name, year)

		// The test reading, out of the same value.
		if len(got) > 0 {
			fmt.Printf("  %q parsed\n", row)
		} else {
			fmt.Printf("  %q not parsed\n", row)
		}
	}
}
```

```
ada    1815
  "ada 1815" parsed
(none) (none)
  "no year here" not parsed
```

A nil slice is doing a lot of work there, and it is worth being explicit about why it can: `len(nil)` is 0, ranging over nil iterates zero times, and appending to nil allocates. So "no match" needs no sentinel and no second result. That is the same reason Go code returns nil slices freely where other languages return empty ones.

The reading this does *not* cover is `my $one = parts($row)`, where Perl puts the match in scalar context through the return and yields 1 or the empty string. There is no signature that gives the groups to one caller and a boolean to another, so the choice has to be made once, in the function, and stated in its name and its doc comment. If both readings are genuinely wanted, that is two functions: `parts` and `hasParts`. Two names at two call sites is more code and less to remember.

## Lists are flat, and Go's are not

The other half of context is flattening, and it is the half that changes how many things you end up with. A Perl list inside a list is not a nested thing: it is more elements. `map { ($a, $b) } @rows` gives twice as many results as it has rows, `( 'start', @head, 'end' )` is as long as `@head` plus two, and `( 1, 2 ) x 3` is six elements. Go has no such rule anywhere, and the difference is spelled with three dots:

```go
package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

func main() {
	rows := []string{"north,widget,12", "south,gizmo,7"}

	// map { ($f[0], $f[2]) } @rows: two results per row, not one holding two.
	var flat []string
	for _, row := range rows {
		f := strings.Split(row, ",")
		flat = append(flat, f[0], f[2])
	}
	fmt.Println(len(flat), flat)

	// map { [ $_, length $_ ] } @rows: a reference is one value, so this is
	// one result per row, and the type says which of the two you have.
	var pairs [][]string
	for _, row := range rows {
		pairs = append(pairs, []string{row, fmt.Sprint(len(row))})
	}
	fmt.Println(len(pairs))

	// map { @{ $h{$_} } } sort keys %h: one level flattened out.
	byRegion := map[string][]string{"north": {"a", "b"}, "south": {"c"}}
	var all []string
	for _, k := range slices.Sorted(maps.Keys(byRegion)) {
		all = append(all, byRegion[k]...)
	}
	fmt.Println(len(all), all)

	// ( %defaults, %site ): a clone and a copy, and the shallowness is visible.
	defaults := map[string]string{"host": "localhost", "port": "8080"}
	site := map[string]string{"port": "443"}
	conf := maps.Clone(defaults)
	maps.Copy(conf, site)
	fmt.Println(conf["host"], conf["port"])
}
```

```
4 [north 12 south 7]
2
3 [a b c]
localhost 443
```

The good news is that Go tells you when you get this wrong, most of the time. `[]string` and `[][]string` are different types, so a block that was meant to flatten and did not is usually a compile error rather than a wrong answer. The exception, and it is the one to watch, is when the element type is `any`: then both shapes compile, and the only symptom is a count that is too small.

Two conveniences worth knowing once you are writing Go rather than translating it. `slices.Concat(a, b, c)` is the list comma over several slices, and `slices.Repeat(s, n)` is the list form of `x`. And `maps.Clone` followed by `maps.Copy` is the merged hash, in two lines that make it plain the copy is shallow: the values are shared, so a nested map inside one is the same nested map inside the other.

## The mismatch

Context is not a feature Go left out, it is a question Go never asks, so the porting work is to decide once, at each site, what the Perl meant and write that down. Practical audit list for translated code: every `my $n = @a` is `len(a)`; every `.` or arithmetic operator with an array on either side is a `len` too, because those operators impose scalar context and it is easy to read the Perl as concatenating the elements; `my ($x) = f()` is `f()[0]` with an emptiness check, while `my $x = f()` is `len(f())`; `(LIST)[i]` needs the list named before it can be indexed; `my $n = () = LIST` is `len` of the list and nothing more exotic; and `wantarray` is a design change, not a translation. The compiler catches the ones where the types disagree and says nothing about the ones where they happen to line up, which is why the `len` cases are worth going through by hand.

Further reading: https://perldoc.perl.org/perldata#Context and https://go.dev/ref/spec#Length_and_capacity
