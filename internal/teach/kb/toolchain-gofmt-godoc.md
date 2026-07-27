---
id: toolchain-gofmt-godoc
title: One binary drives everything, and gofmt ended the style wars
tags: [orientation, tooling, gofmt, godoc]
perl_triggers: [perltidy, perltidyrc, perldoc, prove, shebang]
severity: info
prerequisites: [compile-time-mindset]
---

Every Go workflow runs through one binary: `go build` compiles, `go run` compiles-and-executes, `go test` finds and runs tests, `go vet` finds suspicious-but-compilable code, `go doc` is your `perldoc`, and `gofmt` reformats source into the one true style. There is no Makefile.PL, no prove, no ExtUtils anything — and, culturally the biggest shock, there is no `.perltidyrc`, because formatting is not configurable. Fifteen years of TMTOWTDI instincts meet a community that decided There Is Only One Way To Format It, and the payoff is that every Go file you will ever read looks the same.

## The Perl you know

```
$ perltidy -pbp -b lib/My/App.pm    # your style, per project, endlessly debated
$ perlcritic --severity 3 lib/
$ prove -lr t/
$ perldoc List::Util
```

Each tool is separate, optional, and configured per team; style arguments are a permanent background process.

## The Go you write

`gofmt` takes deliberately mangled (but valid) code:

```go
package main
import "fmt"
func main(){x:=map[string]int{"a":1,
"b":2}
for k,v:=range x{fmt.Println(k,v)}}
```

and emits, with no flags because there are none for style:

```go
package main

import "fmt"

func main() {
	x := map[string]int{"a": 1,
		"b": 2}
	for k, v := range x {
		fmt.Println(k, v)
	}
}
```

Tabs, brace placement, spacing: all decided for you. Editors run it on save; CI rejects unformatted code; nobody argues, because there is nothing to argue about. Semicolons exist in the grammar but are inserted automatically at line ends, which is why the opening brace *must* be on the same line as `func` or `if` — moving it to the next line is a syntax error, not a style choice.

`go vet` catches what compiles but is wrong. This program builds and runs:

```go
package main

import "fmt"

func main() {
	count := "many"
	fmt.Printf("processed %d records\n", count)
}
```

```console
$ go run vetdemo.go
processed %!d(string=many) records
$ go vet vetdemo.go
vetdemo.go:7:24: fmt.Printf format %d has arg count of wrong type string
```

And `go doc` works offline on the stdlib and every dependency, from the command line:

```console
$ go doc strings.Builder
package strings // import "strings"

type Builder struct {
	// Has unexported fields.
}
    A Builder is used to efficiently build a string using Builder.Write methods.
    It minimizes memory copying. The zero value is ready to use. Do not copy a
    non-zero Builder.

func (b *Builder) Cap() int
func (b *Builder) Grow(n int)
...
```

Documentation is plain comments directly above declarations (`// Distance returns ...`) — no POD, no `=cut`, and https://pkg.go.dev renders the same comments for the whole ecosystem.

## The mismatch

The tools you will actually run daily: `go run .` while iterating (compilation is fast enough that it feels interpreted), `go build` to produce a single static binary you can `scp` to a server with no interpreter or module tree waiting there — deployment is one file, the single biggest operational difference from shipping Perl — `go test ./...` before committing (the `./...` wildcard means "this package and everything below", and `table-driven-tests` covers what goes in the files it finds), and `go vet ./...` in CI. Adopt gofmt on day one and never format by hand; fighting it marks code as written by an outsider more surely than any other habit.

Further reading: https://go.dev/blog/gofmt and https://pkg.go.dev/cmd/go
