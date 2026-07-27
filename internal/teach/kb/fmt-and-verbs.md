---
id: fmt-and-verbs
title: printf survives the port, but the verbs are typed now
tags: [idiom, formatting, stdlib, strings]
perl_triggers: [printf, sprintf, print, say, string-interpolation, here-doc-interpolation, output-field-separator, output-record-separator, number-stringification]
severity: warning
prerequisites: [explicit-conversions-no-coercion, strings-are-bytes]
---

`printf` is the one Perl builtin that arrives in Go almost intact: same `%`, same flags, same width and precision, same `*` for a runtime width. What changes is that the verbs are now checked against the argument's *type*, and `%s` no longer means "whatever this is, as text". Perl's `%s` stringifies anything you hand it because every scalar already knows how to be a string; Go's `%s` on an `int` produces the literal text `%!s(int=42)` in your output, and the program keeps running. Nothing crashes, nothing is logged, and the damage shows up in a report a week later. That is why `go vet` (`vet-and-staticcheck`) checks format strings, and why running it is not optional.

## The Perl you know

```perl
printf "%-10s %5.2f %d\n", $name, $price, $qty;
my $row = sprintf "%04d", $id;

print "user $name has $count items\n";     # interpolation
print STDERR "cannot open $path: $!\n";
printf "%s\n", 42;                          # 42: everything stringifies
printf "%d\n", "42 apples";                 # 42: everything numifies too
printf "%g\n", 1234567.0;                   # 1.23457e+06

{
    local $, = "-";                          # output field separator
    local $\ = "\n";                         # output record separator
    print "a", "b";                          # a-b\n
}
```

One family of functions (`print`, `printf`, `sprintf`, `say`), one set of separators, and a scalar that answers to any verb you ask of it.

## The Go you write

```go
package main

import (
	"fmt"
	"os"
	"strings"
)

type item struct {
	Name string
	Qty  int
}

func main() {
	// The familiar half: flags, width, precision, and * all behave.
	fmt.Printf("%-10s|%5.2f|%04d|%*d|\n", "widget", 19.5, 7, 6, 42)

	// The unfamiliar half: a verb that does not match its argument is
	// printed as an error inside the output rather than raised.
	fmt.Printf("%s %d\n", 42, 42.9)

	// %v is the verb Perl never needed: "print this in its natural form".
	// %+v adds field names, %#v prints Go syntax, %T prints the type.
	it := item{"bolt", 12}
	fmt.Printf("%v | %+v | %#v | %T\n", it, it, it, it)
	fmt.Printf("%v %v %v\n", []int{1, 2, 3}, map[string]int{"b": 2, "a": 1}, nil)

	// %q quotes and escapes, which is how you make whitespace bugs visible.
	fmt.Printf("%q %q\n", "two words\n", strings.Fields("two words"))

	// Print and Println are not the same function with a newline bolted on.
	fmt.Println("a", 1, "b")   // spaces between every operand
	fmt.Print("a", 1, "b", "\n") // spaces only between two non-strings
	fmt.Print("a", 1, 2, "b", "\n")

	// Writing somewhere other than stdout takes an io.Writer, not a handle.
	fmt.Fprintf(os.Stderr, "cannot open %s: %v\n", "/etc/shadow", os.ErrPermission)

	// Sprintf builds a string; there is no interpolation to fall back on.
	fmt.Println(fmt.Sprintf("user %s has %d items", "jane", 3))
}
```

```
widget    |19.50|0007|    42|
%!s(int=42) %!d(float64=42.9)
{bolt 12} | {Name:bolt Qty:12} | main.item{Name:"bolt", Qty:12} | main.item
[1 2 3] map[a:1 b:2] <nil>
"two words\n" ["two" "words"]
a 1 b
a1b
a1 2b
cannot open /etc/shadow: permission denied
user jane has 3 items
```

The `cannot open` line is on standard error and the rest is on standard output; a terminal shows both, `go run . > out.txt` separates them.

Two of those lines repay a second look. `map[a:1 b:2]` is sorted: `fmt` sorts map keys before printing, so `%v` on a map is deterministic even though ranging over it is not (`map-iteration-order`). And `fmt.Print` inserts a space only between operands that are *both* non-strings, which is a rule nobody remembers correctly; if you care about the spacing, use `Printf` and say what you mean.

## The mismatch

The verb table, where Perl and Go disagree. `%s` in Go means "a string, a `[]byte`, or something with a `String() string` method" and nothing else; give it a number and you get `%!s(int=42)` embedded in your output. `%d` given a float is `%!d(float64=42.9)`, where Perl would have truncated to `42`, so every `printf "%d", $x` over a possibly fractional value needs a decision at translation time: `int(x)` truncates like Perl, `%.0f` rounds half to even. `%v` has no Perl equivalent and is the verb you will use most, because it prints any value in a sensible default form; `%+v` on a struct adds the field names and is the closest thing to `Data::Dumper` for a quick look, with `%#v` giving re-pastable Go syntax. Perl's `%v` is a different thing entirely (a version-vector flag: `sprintf "%vd", v1.22.333` yields `1.22.333`), so a format string carrying `%vd` is one of the few that means something in both languages and something different in each.

`%g` deserves its own warning. Perl inherits C's rule of six significant digits, so `printf "%g", 1234567.0` prints `1.23457e+06`; Go's `%g` prints the shortest text that reads back as the same float64, giving `1.234567e+06`. Byte-identical output therefore needs `%.6g` in Go, which is what a faithful conversion emits. The same shortest-round-trip rule is why `fmt.Println(0.1 + 0.2)` prints `0.30000000000000004` where Perl prints `0.3`: Perl stringifies through fifteen significant digits and trims, Go does not round for you.

The rest is smaller but adds up. There is no `$,`, no `$\`, and no `$"`: separators are explicit, `strings.Join` for lists, `Println` when you want the newline. `print` in Perl takes a list; `fmt.Print` takes `...any`, which is why `fmt.Print("n=", n, "\n")` compiles but reads badly next to `fmt.Printf("n=%d\n", n)`. Argument counts are checked at run time and reported in the output: too few arguments yields `%!d(MISSING)`, too many appends `%!(EXTRA int=2)` after everything else, including after your newline. `%w` exists only in `fmt.Errorf`, where it wraps an error rather than formatting it (`error-wrapping`); using it in `Printf` produces `%!w(...)`. And the whole family has a `Sprint`/`Fprint` sibling: `Sprintf` returns a string, `Fprintf` writes to any `io.Writer` (`io-reader-writer`), which is how a function that "prints" becomes testable.

Further reading: https://pkg.go.dev/fmt
