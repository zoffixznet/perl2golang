---
id: var-vs-short-declaration
title: Declaring variables, := versus var, and the shadowing trap
tags: [gotcha, types, declarations, scoping]
perl_triggers: [my, our, state]
severity: warning
prerequisites: [static-types-and-zero-values]
---

Go has two ways to declare a variable, and picking between them is mechanical, not stylistic: `:=` declares-and-initialises with an inferred type and works only inside functions; `var` works everywhere and is the only option at package level or when you want the zero value. The trap hiding in this convenience is *shadowing*: `:=` inside an inner block silently creates a brand-new variable with the same name, and the classic Go bug - assigning to a shadowed `err` and then checking the outer one - is Perl's `my $x` inside a block writ large, except your instincts do not flag it because in Perl re-`my`-ing is a habit, not a hazard.

## The Perl you know

```perl
my $count = 1;
if (1) {
    my $count = 2;    # new lexical, shadows outer - same as Go
}
print "$count\n";     # 1
```

Perl warns about `"my" variable masks earlier declaration` in the same scope, but nested-scope shadowing is silent and usually harmless because you rarely re-declare accidentally - `$count = 2` (no `my`) is the natural mutation syntax.

## The Go you write

Compiled and run as shown:

```go
package main

import (
	"fmt"
	"strconv"
)

func main() {
	var port int
	if p, err := strconv.Atoi("8080"); err == nil {
		port = p
	}
	fmt.Println(port)

	count := 1
	if true {
		count := 2 // NEW variable: shadows the outer count
		_ = count
	}
	fmt.Println(count)
}
```

```
8080
1
```

The rules, compactly: `var name type` gives the zero value; `var name = value` infers; `name := value` is the in-function shorthand. Package level allows only `var` - this is a real syntax error:

```go-invalid
package main

x := 5

func main() {}
```

```
./decl_err.go:3:1: syntax error: non-declaration statement outside function body
```

And `:=` refuses to re-declare in the *same* scope unless at least one variable on the left is new:

```
./decl_err2.go:7:4: no new variables on left side of :=
```

That "at least one new" rule is why `v, err := f()` followed by `w, err := g()` is legal and idiomatic - the second `err` is *reused*, not redeclared.

## The one thing `:=` cannot infer from

`:=` reads the type off the value on its right, so it needs the value to have one. A bare `nil` does not: `nil` is not a value of some universal type, it is a *literal for the zero value of whichever pointer, map, slice, channel, function or interface type the context asks for*, and a short declaration provides no context at all.

```go-invalid
package main

func main() {
	x := nil
	_ = x
}
```

```
./sample.go:4:7: use of untyped nil in assignment
```

The fix is to say what the variable holds, and then the nil is implied rather than written:

```go
package main

import "fmt"

func main() {
	var missing any
	fmt.Println(missing == nil, missing)

	var name *string
	fmt.Println(name == nil)

	found := "here"
	name = &found
	fmt.Println(name == nil, *name)
}
```

```
true <nil>
true
false here
```

This is exactly where `my $x = undef;` lands, and where `my $x = some_sub();` lands when the sub ends in a bare `return;`. Perl is happy either way, because `undef` is a value that every scalar can hold. Go asks the question Perl never had to: *absent what?*

There is a second shape of the same surprise. A Perl sub always has a value, even if it is `undef`, so assigning from any call is legal. A Go function that declares no results has no value at all, and using it as one is a different error again:

```go-invalid
package main

func nothing() {}

func main() {
	x := nothing()
	_ = x
}
```

```
./sample.go:6:7: nothing() (no value) used as value
```

"No value" and "the nil value" are two different things in Go, and Perl spells both of them `undef`.

## The mismatch

In Go, `name := value` inside an `if`, `for`, or bare block always means a fresh variable, so `err := doThing()` where you meant `err = doThing()` compiles cleanly and quietly discards the error into a shadow - one character, no diagnostic, wrong program (though the shadow at least triggers "declared and not used" if you never read it in that block). Train the reflex: mutation is `=`, birth is `:=`, and when an inner block needs to set an outer variable, declare with `var` outside and assign with `=` inside, exactly as in the `port` example. One more habit inversion: Perl code `my`-declares everything defensively at first use; Go code prefers `:=` at first use too, but uses the `if v, err := f(); err != nil` form to keep the scope of temporaries as tight as one statement.

Further reading: https://go.dev/ref/spec#Short_variable_declarations
