---
id: strconv-parsing
title: strconv turns strings into numbers, and refuses to guess
tags: [idiom, strings, numbers, conversion]
perl_triggers: [string-to-number, sprintf-number, printf-number, hex, oct, looks-like-number]
severity: info
prerequisites: [explicit-conversions-no-coercion, errors-are-values]
---

Every place your Perl relied on a string quietly becoming a number - reading a config value, a CGI parameter, a column from a file - becomes an explicit `strconv` call in Go that returns a value *and an error*. The package is small and its naming is systematic, but two behaviours will surprise you: parsing is strict to the point of rejecting leading and trailing whitespace, and it never parses "as much as it can", so `"10 apples"` is an error rather than `10`. That strictness is the feature - it turns Perl's silent numification bugs into a value you must handle at the call site.

## The Perl you know

```perl
my $n = "42";
my $total = $n + 8;              # 50 - numification is invisible
my $bad = "10 apples" + 0;       # 10, with a warning you probably disabled
printf "%05.2f\n", 3.14159;      # 03.14
my $bytes = hex("ff");           # 255
my $mode  = oct("0755");         # 493
```

`Scalar::Util::looks_like_number` exists precisely because the language will not tell you whether a numification was meaningful.

## The Go you write

```go
package main

import (
	"errors"
	"fmt"
	"strconv"
)

func main() {
	n, err := strconv.Atoi("42")
	fmt.Println(n+8, err)

	// Strict: no partial parses, no surrounding space.
	_, err = strconv.Atoi("10 apples")
	fmt.Println(err)
	_, err = strconv.Atoi(" 42")
	fmt.Println(err)

	// Range errors are distinguishable from syntax errors.
	_, err = strconv.Atoi("99999999999999999999")
	fmt.Println(errors.Is(err, strconv.ErrRange), errors.Is(err, strconv.ErrSyntax))

	// ParseInt: explicit base and bit size. Base 0 honours 0x, 0o, 0b prefixes.
	b16, _ := strconv.ParseInt("ff", 16, 64)
	auto, _ := strconv.ParseInt("0755", 0, 64)
	fmt.Println(b16, auto)

	f, _ := strconv.ParseFloat("3.14159", 64)
	ok, _ := strconv.ParseBool("true") // accepts 1, t, T, TRUE, true, and friends
	fmt.Println(f, ok)

	// Number to string, both directions of the same package.
	fmt.Println(strconv.Itoa(255), strconv.FormatInt(255, 16), strconv.Quote("a\tb"))
	fmt.Printf("%05.2f|%x|%q\n", 3.14159, 255, "a\tb")
}
```

```
50 <nil>
strconv.Atoi: parsing "10 apples": invalid syntax
strconv.Atoi: parsing " 42": invalid syntax
true false
255 493
3.14159 true
255 ff "a\tb"
03.14|ff|"a\tb"
```

The error messages are unusually good: they name the function, quote the input, and say which rule was broken, so an error that reaches a log tells you exactly what the bad data was.

## The mismatch

Learn the naming scheme once and the package stops needing lookup: `Parse*` goes string→value (`ParseInt`, `ParseUint`, `ParseFloat`, `ParseBool`, `ParseComplex`), `Format*` goes value→string (`FormatInt`, `FormatFloat`, `FormatBool`), and `Atoi`/`Itoa` are the convenience pair for base-10 `int`. `ParseInt` always returns `int64`, so assigning to an `int` needs a conversion (`explicit-conversions-no-coercion`) - `Atoi` exists to save you that round trip. The bit-size argument is a range check, not a return type: `ParseInt(s, 10, 8)` still returns `int64` but fails if the value will not fit in an `int8`, which is a free validation step for ports of "this column must be a byte". Perl habits to retire: `hex($s)` → `strconv.ParseInt(s, 16, 64)`; `oct($s)` → `strconv.ParseInt(s, 0, 64)`, which reads the prefix and so handles `0x`, `0o`, `0b`, and bare octal `0755` the way `oct` did; `looks_like_number($s)` → simply parse it and check the error, since the parse is the test; `$x + 0` and `"$x"` as conversion idioms → nothing, because a typed value never needs coercing to be used. For formatting, `fmt.Sprintf` covers `sprintf` one-for-one (`%d`, `%s`, `%f`, `%05.2f`, `%x` all behave), with `%v` as the "print it sensibly" verb Perl never needed and `%q` producing a quoted, escaped Go string literal - useful in error messages, where showing the exact bad input matters. One trap when going the other way: `string(65)` is *not* `"65"`, it is `"A"`, because converting an integer to a string interprets it as a rune; use `strconv.Itoa`, and `go vet` will flag the mistake if you forget.

Further reading: https://pkg.go.dev/strconv and https://pkg.go.dev/fmt
