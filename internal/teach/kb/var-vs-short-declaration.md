---
id: var-vs-short-declaration
title: Declaring variables, := versus var, and the shadowing trap
tags: [gotcha, types, declarations, scoping]
perl_triggers: [my, our, state]
severity: warning
prerequisites: [static-types-and-zero-values]
---

Go has two ways to declare a variable, and picking between them is mechanical, not stylistic: `:=` declares-and-initialises with an inferred type and works only inside functions; `var` works everywhere and is the only option at package level or when you want the zero value. The trap hiding in this convenience is *shadowing*: `:=` inside an inner block silently creates a brand-new variable with the same name, and the classic Go bug — assigning to a shadowed `err` and then checking the outer one — is Perl's `my $x` inside a block writ large, except your instincts do not flag it because in Perl re-`my`-ing is a habit, not a hazard.

## The Perl you know

```perl
my $count = 1;
if (1) {
    my $count = 2;    # new lexical, shadows outer — same as Go
}
print "$count\n";     # 1
```

Perl warns about `"my" variable masks earlier declaration` in the same scope, but nested-scope shadowing is silent and usually harmless because you rarely re-declare accidentally — `$count = 2` (no `my`) is the natural mutation syntax.

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

The rules, compactly: `var name type` gives the zero value; `var name = value` infers; `name := value` is the in-function shorthand. Package level allows only `var` — this is a real syntax error:

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

That "at least one new" rule is why `v, err := f()` followed by `w, err := g()` is legal and idiomatic — the second `err` is *reused*, not redeclared.

## The mismatch

In Go, `name := value` inside an `if`, `for`, or bare block always means a fresh variable, so `err := doThing()` where you meant `err = doThing()` compiles cleanly and quietly discards the error into a shadow — one character, no diagnostic, wrong program (though the shadow at least triggers "declared and not used" if you never read it in that block). Train the reflex: mutation is `=`, birth is `:=`, and when an inner block needs to set an outer variable, declare with `var` outside and assign with `=` inside, exactly as in the `port` example. One more habit inversion: Perl code `my`-declares everything defensively at first use; Go code prefers `:=` at first use too, but uses the `if v, err := f(); err != nil` form to keep the scope of temporaries as tight as one statement.

Further reading: https://go.dev/ref/spec#Short_variable_declarations
