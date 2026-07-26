---
id: static-types-and-zero-values
title: Every variable has a type and is never uninitialised
tags: [orientation, types, zero-values]
perl_triggers: [my-declaration, undef, uninitialized-warning, defined]
severity: info
prerequisites: [compile-time-mindset]
---

In Perl, a freshly declared variable holds `undef` and its "type" is whatever you use it as next. In Go, declaration commits you to a type forever, and there is no uninitialised state to check for: every declared variable *immediately* holds a well-defined value called its zero value. This kills an entire class of `Use of uninitialized value` warnings, but it also means Go cannot distinguish "the port is 0" from "nobody set the port" — a distinction Perl code leans on constantly, and one you must now design for explicitly.

## The Perl you know

```perl
my $count;                    # undef until someone assigns
my %config;
print "port: $config{port}\n";   # warns: Use of uninitialized value
$count = "3 items";              # fine, it's a string now
$count += 1;                     # fine-ish, it's 4 now (with a warning)
```

A scalar is a chameleon, and `undef` is your universal "not set yet" marker, tested with `defined`.

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

type server struct {
	host    string
	port    int
	tls     bool
	tags    []string
	retries *int
}

func main() {
	var (
		n  int
		f  float64
		s  string
		b  bool
		p  *int
		xs []int
		m  map[string]int
	)
	fmt.Printf("int:     %d\n", n)
	fmt.Printf("float64: %g\n", f)
	fmt.Printf("string:  %q\n", s)
	fmt.Printf("bool:    %t\n", b)
	fmt.Printf("pointer: %v\n", p)
	fmt.Printf("slice:   %v (nil: %t)\n", xs, xs == nil)
	fmt.Printf("map:     %v (nil: %t)\n", m, m == nil)

	var srv server
	fmt.Printf("struct:  %+v\n", srv)
}
```

```
int:     0
float64: 0
string:  ""
bool:    false
pointer: <nil>
slice:   [] (nil: true)
map:     map[] (nil: true)
struct:  {host: port:0 tls:false tags:[] retries:<nil>}
```

Note the last line: a struct's zero value is the zero value of every field, recursively, with no constructor required. Well-designed Go types make that state directly usable — the documented mantra is "the zero value is ready to use" (`bytes.Buffer`, `sync.Mutex`, `strings.Builder` all work this way).

## The mismatch

Two habits need rewiring. First, `defined($x)` has no general translation, because a Go `int` is never undefined — it is `0`. When "unset" and "zero" genuinely differ, Go code makes absence explicit with a pointer field (`retries *int`, where `nil` means unset — see `nil-vs-undef`), the comma-ok map lookup (see `comma-ok-idiom`), or a separate boolean. Second, stop writing initialisation boilerplate: `var total int` is already `0`, `var names []string` is already appendable, `var buf strings.Builder` is already usable. Zero values are a design principle, not an accident of memory — Go deliberately defines them so that "I forgot to initialise" is not a bug category.

Further reading: https://go.dev/ref/spec#The_zero_value
