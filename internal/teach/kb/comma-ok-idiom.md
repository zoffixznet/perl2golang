---
id: comma-ok-idiom
title: Comma-ok replaces exists, and defined has no seat at the table
tags: [idiom, maps, nil]
perl_triggers: [exists, delete-hash-elem, defined-hash-elem, defined-or, defined-or-assign, or-assign]
severity: info
prerequisites: [static-types-and-zero-values, nil-slices-vs-nil-maps]
---

A Go map lookup always succeeds: `m[k]` on a missing key quietly returns the zero value, so `visits["bob"]` is `0` whether bob visited zero times or was never seen at all. The comma-ok form — `v, ok := m[k]` — is the *only* way to tell those apart, and it replaces `exists`. What it does not replace is `defined`: Perl's three-state world (absent / present-but-undef / present-with-value) collapses to two states in Go, and if ported code relied on the middle state, you must redesign, not transliterate.

## The Perl you know

```perl
my %h = (alice => 0, carol => undef);
# what perl prints:
# alice  exists:1 defined:1
# carol  exists:1 defined:0    <- the middle state Go doesn't have
# dave   exists:0 defined:0

$h{eve} //= 'default';    # set only if not defined
delete $h{alice};
```

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

func main() {
	visits := map[string]int{
		"alice": 0, // present, with a zero value
	}

	fmt.Println(visits["alice"]) // 0
	fmt.Println(visits["bob"])   // also 0: absence looks identical

	v, ok := visits["alice"]
	fmt.Println(v, ok)
	v, ok = visits["bob"]
	fmt.Println(v, ok)

	if _, ok := visits["bob"]; !ok {
		fmt.Println("bob has never visited")
	}

	delete(visits, "alice")
	_, ok = visits["alice"]
	fmt.Println(ok)
}
```

```
0
0
0 true
0 false
bob has never visited
false
```

`delete(m, k)` is the `delete $h{k}` equivalent — a no-op on missing keys, no return value worth having. The `if _, ok := ...; !ok` line shows the idiomatic one-statement scoping from `var-vs-short-declaration`.

## Carrying "there was nothing" out of a function

Inside one function the comma-ok form answers the question. The harder case is a lookup wrapped in a sub, because Perl's answer travels out on its own: `return $port{$name}` hands back undef for a key that is not there, and the caller's `defined` test picks it up. A Go signature has to say so, and if it does not, the information is gone at the boundary and no care at the call site can recover it.

There are two spellings, and the choice is a design decision rather than a mechanical one:

```go
package main

import "fmt"

var port = map[string]int{"http": 80, "https": 443, "echo": 0}

// portOf is the comma-ok shape, and it composes with the map read that
// produced it: the second result travels all the way out to the caller.
func portOf(name string) (int, bool) {
	p, ok := port[name]
	return p, ok
}

// portPtr is the other spelling. One value, nil for absent, and a dereference
// at every use. It reads better when the answer is passed on rather than
// tested here.
func portPtr(name string) *int {
	p, ok := port[name]
	if !ok {
		return nil
	}
	return &p
}

func main() {
	for _, name := range []string{"http", "echo", "gopher"} {
		if p, ok := portOf(name); ok {
			fmt.Printf("%-7s port %d\n", name, p)
		} else {
			fmt.Printf("%-7s not listed\n", name)
		}
	}
	fmt.Println(portPtr("echo") != nil, portPtr("gopher") != nil)
}
```

```
http    port 80
echo    port 0
gopher  not listed
true false
```

Notice what the entry with a real 0 in it is doing there. `echo` has port 0, and every version of this that returns a bare `int` reports it as missing. That is the bug the comma-ok form exists to prevent, and it is invisible until the day a legitimate zero turns up in the data.

Prefer `(T, bool)` when the caller will test the answer immediately, which is most of the time: it is what the standard library does, it costs no allocation, and the `if v, ok := f(); ok` line reads as one thought. Prefer `*T` when the value is going to be stored or passed along still-maybe-absent, because a pair does not fit in a struct field or a slice element without inventing a type for it. And prefer `(T, error)` over both when the caller deserves to know *why* there was no answer.

## The mismatch

Mappings to retrain: `exists $h{k}` → `_, ok := m[k]`; `exists $ENV{X}` → `_, ok := os.LookupEnv("X")`, because `os.Getenv` returns `""` for unset and set-to-empty alike; `defined $h{k}` → *does not exist*; `$h{k} // $default` → `if v, ok := m[k]; ok { use v } else { use default }` (there is no `//` operator, and no `||`-returns-value either — Go's `||` yields only bool, so the entire `$x || $y` default-value idiom is dead; write the if-statement). When Perl code genuinely uses all three states — a config hash where `key => undef` means "explicitly disabled" as distinct from "not mentioned" — the honest Go translation is `map[string]*string` (nil value = present-but-valueless) or a small struct value with a validity flag. The comma-ok shape is not map-specific; it is a language-wide convention you will meet three more times: type assertions `v, ok := x.(T)` (`type-assertions-and-switches`), channel receives `v, ok := <-ch` (`channels-and-select`), and it is deliberately echoed by `v, err :=` returns. One caution: comma-ok is a special *assignment form*, not an expression — you cannot pass `m[k]`'s ok-ness inline to a function; it must land in variables first.

Further reading: https://go.dev/blog/maps and https://go.dev/ref/spec#Index_expressions
