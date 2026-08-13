---
id: compile-time-mindset
title: The compiler is the first test suite
tags: [orientation, compiler, tooling]
perl_triggers: [use-strict, use-warnings, perl-c, undefined-subroutine, cant-locate-method]
severity: info
prerequisites: []
---

Errors you are used to discovering in production at 3 a.m. - a typoed function name, a call into a module that never loaded, a variable you meant to use but did not - do not exist in a running Go program, because the program never runs until they are gone. The flip side: Go refuses to compile things Perl happily tolerates, including *unused* variables and imports, so your first hour with Go will feel like arguing with a linter that has root. It is not being pedantic for style; it is the language's replacement for `use strict`, `use warnings`, `perl -c`, and a chunk of your test suite, and it is non-negotiable.

## The Perl you know

`perl -c` checks syntax, not existence. A typoed call is a runtime bomb that only detonates when that line is reached:

```perl
use strict;
use warnings;

print "deploying\n";
if (0) {
    totally_missing_sub();   # never reached, never noticed
}
```

```
$ perl -c script.pl
script.pl syntax OK
$ perl script.pl
deploying
```

The dead branch could sit there for years. `strict` catches undeclared *variables*, but subs, methods, and most cross-module mistakes surface only at runtime.

## The Go you write

The same category of mistake never produces a binary. This intentionally does not compile:

```go-invalid
package main

import "strings"

func main() {
	s := strings.ToUppercase("hi")
	_ = s
}
```

```
$ go run typo.go
./typo.go:6:15: undefined: strings.ToUppercase
```

And Go goes further than any Perl pragma: unused things are hard errors, not warnings. Both of these fail to compile, on purpose:

```go-invalid
package main

import "fmt"

func main() {
	debug := true
	fmt.Println("deploying")
}
```

```
./unused.go:6:2: declared and not used: debug
```

```go-invalid
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello")
}
```

```
./unusedimport.go:5:2: "os" imported and not used
```

When you genuinely need to keep a value around while debugging, assign it to the blank identifier: `_ = debug`. Editors running `goimports` add and remove imports automatically, so in practice the unused-import error mostly appears when editing by hand.

## The mismatch

Perl's model is "compile as little as possible, resolve at runtime, trust the tests" - that is what makes `AUTOLOAD`, `can()`, string eval, and symbol-table surgery possible. Go's model is "resolve everything before the program exists". You lose runtime flexibility (there is no `eval "..."`, no monkey-patching, no loading code by computed name) and gain a guarantee: if it compiled, every identifier in it resolves. Retrain one reflex: the edit-save-run loop becomes edit-save-*compile*-run, and a compile error is the tool doing its job, not an obstacle. Unused-variable errors will annoy you for a week; they exist because an unused variable is very often a bug (you computed the wrong thing, or forgot to use the right one), and Go's authors decided a warning nobody reads is worth less than an error everybody fixes.

Further reading: https://go.dev/doc/faq#unused_variables_and_imports
