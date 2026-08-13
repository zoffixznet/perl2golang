---
id: strings-are-bytes
title: A Go string is bytes, not characters, and it never changes
tags: [trap, strings, unicode, runes, bytes]
perl_triggers: [use-utf8, length, substr, reverse-string, lc, uc, string-append, index]
severity: trap
prerequisites: [explicit-conversions-no-coercion]
---

Under `use utf8`, Perl gives you character semantics: `length("naïve café")` is 10 and `substr` never cuts a character in half. A Go string is an immutable sequence of *bytes* that by convention holds UTF-8, and every operation - `len`, indexing, slicing - works on bytes, so the same text has `len` 12, `s[3]` is the number `175`, and `s[:3]` hands you half of an `ï`. Nothing warns you. Code ported with Perl's length/substr instincts intact will corrupt non-ASCII text while passing every ASCII-only test you write.

## The Perl you know

```perl
use utf8;
my $s = "naïve café";
say length $s;                 # 10 - characters
{ use bytes; say length $s; }  # 12 - bytes, only if you ask
```

The output is `10`, then `12`. Perl decodes source and data into an internal character representation; byte-level access is the special case you opt into.

## The Go you write

Compiled and run as shown:

```go
package main

import (
	"fmt"
	"unicode/utf8"
)

func main() {
	s := "naïve café"

	fmt.Println(len(s))                    // bytes
	fmt.Println(utf8.RuneCountInString(s)) // characters (runes)

	fmt.Println(s[3])         // indexing yields a byte (uint8), here mid-rune
	fmt.Printf("%q\n", s[:3]) // slicing can cut a rune in half

	for i, r := range "aïb" {
		fmt.Printf("byte offset %d: %c\n", i, r)
	}

	runes := []rune(s)
	fmt.Println(len(runes), string(runes[:3]))
}
```

```
12
10
175
"na\xc3"
byte offset 0: a
byte offset 1: ï
byte offset 3: b
10 naï
```

Read that carefully: `range` over a string is the one construct that decodes UTF-8 - it yields *rune* values at their *byte* offsets, which is why the loop indexes jump 0, 1, 3. A `rune` is just `int32` (a Unicode code point); a `byte` is `uint8`. For character-safe slicing, convert to `[]rune` first, as the last line does.

Strings are also immutable, and the compiler says so:

```go-invalid
package main

func main() {
	s := "hello"
	s[0] = 'H'
}
```

```
./strimm_err.go:5:2: cannot assign to s[0] (neither addressable nor a map index expression)
```

So Perl's `$out .= $chunk` loop, which mutates in place, becomes `strings.Builder` - repeated `out += chunk` on a Go string allocates a fresh copy every pass and is quadratic:

```go
package main

import (
	"fmt"
	"strings"
)

func main() {
	var b strings.Builder // zero value is ready to use
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&b, "row %d;", i)
	}
	fmt.Println(b.String())
}
```

```
row 0;row 1;row 2;
```

## The mismatch

There is no `use utf8` switch to flip because there is no decode step: Go source is UTF-8, string literals are UTF-8 bytes, and the runtime never converts - a string can even hold arbitrary binary garbage, which is also how you should read the absence of a separate "byte string" type. Translate deliberately: Perl `length` → `utf8.RuneCountInString` for characters or `len` for bytes, chosen per call site; `substr` on text → `[]rune` conversion or `strings`-package searching by byte offset; `reverse(scalar)` → reverse a `[]rune`, never the bytes; case-folding → `strings.ToUpper`, which is rune-aware. The mercy is that byte-oriented processing of UTF-8 is usually *safe* when you search for ASCII delimiters (a multi-byte rune never contains an ASCII byte), which is why most Go code handles `len`/indexing bytes without incident - the trap is fixed-width slicing and per-"character" indexing, exactly the operations `substr` habits produce.

Further reading: https://go.dev/blog/strings
