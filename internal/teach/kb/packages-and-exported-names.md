---
id: packages-and-exported-names
title: Capitalisation is the entire privacy system
tags: [orientation, packages, visibility]
perl_triggers: [package, exporter, export-ok, our, private-sub, package-var]
severity: info
prerequisites: [compile-time-mindset]
---

Go has no `Exporter`, no `@EXPORT_OK`, no `use Pkg qw(func)`, and no convention-only privacy: an identifier that starts with an upper-case letter is visible to importing packages, and one that starts with lower-case is compile-time inaccessible outside its package. That single rule replaces Perl's entire export machinery, and unlike Perl's leading-underscore convention, it is *enforced* — reaching for a "private" name is not rude, it is a build failure.

## The Perl you know

```perl
package Geo;
use Exporter 'import';
our @EXPORT_OK = qw(distance);

sub distance { sqrt(($_[2]-$_[0])**2 + ($_[3]-$_[1])**2) }
sub _clamp   { ... }   # "private" by convention only
1;
```

Nothing stops a caller from `Geo::_clamp(...)`. Privacy is a handshake agreement, and exports are a runtime negotiation via `import()`.

## The Go you write

A package `geo` inside module `example.com/pkgdemo`:

```go
package geo

import "math"

// Distance is exported: any importing package may call it.
func Distance(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x2-x1, y2-y1)
}

// clamp is unexported: only code inside package geo may call it.
func clamp(v, lo, hi float64) float64 {
	return math.Min(math.Max(v, lo), hi)
}
```

A `main` in the same module that calls both compiles only halfway:

```go-invalid
package main

import (
	"fmt"

	"example.com/pkgdemo/geo"
)

func main() {
	fmt.Println(geo.Distance(0, 0, 3, 4))
	fmt.Println(geo.clamp(5, 0, 1))
}
```

```
$ go build ./...
./main.go:11:18: undefined: geo.clamp
```

Delete the `geo.clamp` line and it builds and prints `5`. Note the caller always writes `geo.Distance` — the package name is a mandatory qualifier. There is no equivalent of importing `distance` into your own namespace; the closest thing (dot-imports) is effectively banned by convention.

## The mismatch

Everything you know about `@EXPORT`, `import()`, and `Sub::Exporter` simply evaporates — there is nothing to configure. The rule applies uniformly to functions, types, struct fields, methods, constants, and package-level variables, and the struct-field case is the one that bites: `encoding/json` cannot see lower-case fields, so a struct of unexported fields marshals to `{}` (see `struct-tags`). Also unlearn Perl's file-to-package looseness: in Go, one directory equals one package, every file in it declares the same `package` name, and identifiers are shared across all files of the package without any `use`. And note the flipped naming signal: in Perl, `_underscore` marks private; in Go, `Capital` marks *public*, so the default (what you type without thinking) is private — a better default than Perl's.

Further reading: https://go.dev/ref/spec#Exported_identifiers
