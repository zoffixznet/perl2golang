---
id: constants-iota-named-types
title: Constants, iota enums, and giving a type a name
tags: [idiom, types, constants, iota, named-types]
perl_triggers: [use-constant, readonly, dualvar, magic-number, hash-as-enum]
severity: info
prerequisites: [explicit-conversions-no-coercion]
---

Where Perl fakes enums with `use constant` or a hash of magic numbers, Go has a first-class pattern: declare a *named type* over `int`, generate its values with `iota`, and hang methods off it — giving you compile-time protection Perl cannot offer, because a `LogLevel` and a plain `int` (or a `Celsius` and a `Fahrenheit`) are distinct types that refuse to mix. If you skip this idiom and port Perl constants as bare numbers, you discard the main thing Go's type system wanted to give you.

## The Perl you know

```perl
use constant { DEBUG => 0, INFO => 1, WARN => 2, ERROR => 3 };

my $level = INFO;
log_at(ERROR, "disk full");
log_at(42, "oops");          # nothing stops a nonsense level
my $temp_c = 100;
my $temp_f = $temp_c;        # unit confusion is invisible
```

Constants are just inlined subs returning numbers; nothing relates them to each other or restricts where they can be used.

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

type LogLevel int

const (
	Debug LogLevel = iota // 0
	Info                  // 1
	Warn                  // 2
	Error                 // 3
)

func (l LogLevel) String() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR"}[l]
}

type Celsius float64
type Fahrenheit float64

func (c Celsius) ToF() Fahrenheit { return Fahrenheit(c*9/5 + 32) }

const MaxPayload = 1 << 20 // untyped constant

func main() {
	fmt.Println(Info, Error)

	var c Celsius = 100
	fmt.Println(c.ToF())

	var n int64 = MaxPayload // untyped constants adapt to any numeric type
	var f float64 = MaxPayload
	fmt.Println(n, f)
}
```

```
INFO ERROR
212
1048576 1.048576e+06
```

`iota` counts from 0 within a `const` block, and the subsequent lines inherit the expression — so `Info` is implicitly `LogLevel = iota` again, now 1. The `String()` method is picked up by `fmt` automatically, which is why `Println(Info)` prints `INFO`, not `1`. And named types really are distinct — this is a compile error, exactly as observed:

```go-invalid
package main

type Celsius float64
type Fahrenheit float64

func main() {
	var c Celsius = 100
	var f Fahrenheit = c
	_ = f
}
```

```
./namedtype_err.go:8:21: cannot use c (variable of float64 type Celsius) as Fahrenheit value in variable declaration
```

Conversion is explicit and free at runtime: `Fahrenheit(c)` reinterprets, it does not convert units — which is exactly why `ToF()` exists.

## The mismatch

Three things to internalise. First, Go constants are compile-time only and can be *untyped*: `MaxPayload` above initialises an `int64` and a `float64` without conversion, which is the one deliberate soft spot in Go's otherwise rigid numerics (see `explicit-conversions-no-coercion`), and constants can be arbitrary-precision — `1 << 100` is a legal constant as long as you never store it in a variable that cannot hold it. There is no `Readonly` for runtime values: `const` cannot hold a slice, map, or anything computed at runtime; a package-level `var` plus discipline is the Go answer. Second, `iota` patterns scale beyond counting: `KB = 1 << (10 * (iota + 1))` generates size constants, and starting a block with `_ = iota` skips zero so that the zero value means "unset" — a deliberate collaboration with `static-types-and-zero-values`. Third, named types cost nothing and document everything; `type UserID int64` prevents the classic "passed the order ID where the user ID goes" bug at compile time, an entire test category Perl needs to write by hand.

Further reading: https://go.dev/ref/spec#Iota and https://go.dev/blog/constants
