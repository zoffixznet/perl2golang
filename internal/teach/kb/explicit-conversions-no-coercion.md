---
id: explicit-conversions-no-coercion
title: No coercion, ever - numbers and strings never blur
tags: [gotcha, types, numbers, conversion]
perl_triggers: [numeric-context, string-as-number, int-builtin, zero-but-true, numeric-comparison, string-comparison]
severity: warning
prerequisites: [static-types-and-zero-values]
---

Perl's scalar is simultaneously a number and a string and converts on demand; Go's compiler treats `"5" + 2` and even `int * float64` as type errors, so every conversion in your program is one you wrote by hand. This deletes an entire genre of Perl bugs (`"10 apples" + 2` silently becoming `12`) and introduces a new one: integer division truncating where Perl would have given you a float, in code that looks numerically innocent.

## The Perl you know

```perl
say "5" + 2;           # 7    — string numified
say "10 apples" + 2;   # 12   — leading digits win (warns under warnings)
say 7 / 2;             # 3.5  — division is floating-point, always
my $n = 3; my $r = 1.5;
say $n * $r;           # 4.5  — int/float distinction doesn't exist
```

Verified output: `7`, `12`, `3.5`, `4.5`. One scalar type, one numeric tower, `==` for numbers and `eq` for strings doing the disambiguation work.

## The Go you write

Both of these are intentional compile errors:

```go-invalid
package main

func main() {
	n := "5" + 2
	_ = n
}
```

```
./conv_err.go:4:7: invalid operation: "5" + 2 (mismatched types untyped string and untyped int)
```

```go-invalid
package main

import "fmt"

func main() {
	var count int = 3
	var ratio float64 = 1.5
	fmt.Println(count * ratio)
}
```

```
./conv_err2.go:8:14: invalid operation: count * ratio (mismatched types int and float64)
```

The working versions, compiled and run:

```go
package main

import (
	"fmt"
	"strconv"
)

func main() {
	var count int = 3
	var ratio float64 = 1.5
	fmt.Println(float64(count) * ratio)

	fmt.Println(7 / 2)
	fmt.Println(7.0 / 2)
	fmt.Println(float64(7) / 2)

	f := 2.99
	fmt.Println(int(f), int(-f)) // truncates toward zero, silently

	n, err := strconv.Atoi("42")
	fmt.Println(n, err)

	var i32 int32 = 1000
	var i64 int64 = 1000
	fmt.Println(int64(i32) == i64)
}
```

```
4.5
3
3.5
3.5
2 -2
42 <nil>
true
```

## The mismatch

Four rules replace the coercion instinct. One: `T(v)` is the universal conversion syntax, and *even `int` to `int64` requires it* — same-shaped integer types are still distinct types, which matters constantly because `len()` returns `int` while `time.Duration` and file sizes are `int64`. Prefer plain `int` (64-bit on modern platforms) unless an API or a serialisation format dictates otherwise. Two: `/` on two integers is integer division — `7 / 2` is `3`, and Perl's answer needs `float64(a) / float64(b)`; this is the single most common numeric porting bug. Three: string-to-number never happens implicitly and never partially — `strconv.Atoi("10 apples")` returns an *error*, not `10` (see `strconv-parsing`), and `==` works for both numbers and strings because types, not operators, disambiguate — the `==`/`eq` split is gone. Four: untyped *constants* are the one place Go feels Perl-flexible — `7.0 / 2` works, and `const k = 1 << 20` can initialise an `int64` or a `float64` — but a constant that cannot be represented exactly is a compile error (`int(2.99)` with a constant literal refuses to compile), so the flexibility never becomes coercion.

Further reading: https://go.dev/ref/spec#Conversions and https://go.dev/blog/constants
