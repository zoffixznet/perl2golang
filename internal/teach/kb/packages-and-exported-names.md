---
id: packages-and-exported-names
title: Capitalisation is the entire privacy system
tags: [orientation, packages, visibility]
perl_triggers: [package, exporter, export-ok, our, private-sub, package-var]
severity: info
prerequisites: [compile-time-mindset]
---

Go has no `Exporter`, no `@EXPORT_OK`, no `use Pkg qw(func)`, and no convention-only privacy: an identifier that starts with an upper-case letter is visible to importing packages, and one that starts with lower-case is compile-time inaccessible outside its package. That single rule replaces Perl's entire export machinery, and unlike Perl's leading-underscore convention, it is *enforced* - reaching for a "private" name is not rude, it is a build failure.

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

Delete the `geo.clamp` line and it builds and prints `5`. Note the caller always writes `geo.Distance` - the package name is a mandatory qualifier. There is no equivalent of importing `distance` into your own namespace; the closest thing (dot-imports) is effectively banned by convention.

## A package that exports is a namespace, not a class

Perl uses one keyword for two different things. `package Foo;` with `bless` in it is a class, and its subs are methods reached through an object. `package Foo;` with `our @EXPORT_OK = qw(...)` in it is a namespace, and its subs are functions the caller imports by name. Nothing in the syntax separates them, and the bodies can look identical:

```perl
sub summarize {
    my ($freq) = @_;                 # a hash reference, not an object
    my @words = sort { $freq->{$b} <=> $freq->{$a} } keys %$freq;
    ...
}
```

That first line and that arrow are exactly what a method looks like. The thing that settles it is the export list: nobody exports a method, because a method is found through the object rather than by name. In Go the two land in completely different places - one becomes a method on a struct type, the other an ordinary function in a package - and getting it wrong is not a style problem, it is a call that does not compile.

The lesson for reading Perl generally: `@EXPORT`, `@EXPORT_OK` and `use Exporter` are the marks of a module meant to be *called*, and `bless`, `->new` and `$self` are the marks of one meant to be *instantiated*. A module with both is doing both, and its Go translation will be a type plus a few package-level functions, which is a perfectly ordinary Go package.

## The mismatch

Everything you know about `@EXPORT`, `import()`, and `Sub::Exporter` simply evaporates - there is nothing to configure. The rule applies uniformly to functions, types, struct fields, methods, constants, and package-level variables, and the struct-field case is the one that bites: `encoding/json` cannot see lower-case fields, so a struct of unexported fields marshals to `{}` (see `struct-tags`). Also unlearn Perl's file-to-package looseness: in Go, one directory equals one package, every file in it declares the same `package` name, and identifiers are shared across all files of the package without any `use`. And note the flipped naming signal: in Perl, `_underscore` marks private; in Go, `Capital` marks *public*, so the default (what you type without thinking) is private - a better default than Perl's.

One naming trap deserves its own sentence, because Perl habits walk straight
into it: a helper sub named `fmt`, `json` or `sort` reads naturally in Perl
and collides in Go. An import is file-scoped, but a package-level identifier
is package-scoped, so `func fmt(...)` in one file breaks every file in the
package that says `import "fmt"` - the error is "fmt already declared through
import", pointing at the other file. There is no renaming an import out of
the way in the file that declares the function; the function is the one that
has to move. Pick names that do not shadow the standard library packages you
use, the same way you already avoided naming a Perl sub `length`.

Further reading: https://go.dev/ref/spec#Exported_identifiers
