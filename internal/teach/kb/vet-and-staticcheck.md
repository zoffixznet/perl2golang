---
id: vet-and-staticcheck
title: go vet finds the bugs the compiler allows
tags: [idiom, tooling, linting, discipline]
perl_triggers: [perlcritic, use-diagnostics, use-warnings]
severity: info
prerequisites: [toolchain-gofmt-godoc, if-err-nil-rhythm]
---

Between "the compiler accepted it" and "it is correct" sits a narrow band of mistakes Go's type system cannot reach: a `%d` verb given a string, a mutex copied by value, a `context.CancelFunc` never called, an error assigned and forgotten. `go vet` ships in the toolchain to catch exactly that band, and unlike `perlcritic` it has no policy file, no severity dial, and no opinions about style — every check is meant to be a probable bug, so a vet report is something you fix rather than something you configure away. Outside the toolchain, `staticcheck` and `errcheck` extend the same idea, and `go test` runs a subset of vet automatically, which is why a broken `Printf` in a test file fails the test run before a single assertion executes.

## The Perl you know

```perl
# perlcritic: hundreds of policies, five severity levels, per-project config
$ perlcritic --severity 3 lib/
lib/My/App.pm: Subroutine "new" does not end with "return" at line 12
lib/My/App.pm: Two-argument "open" used at line 40

# .perlcriticrc decides which of those matter to your team
```

Half of what `perlcritic` reports is style, so teams tune it, argue about it, and often disable it.

## The Go you write

This program compiles cleanly, runs, and is wrong in three separate ways:

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Counter struct {
	mu sync.Mutex
	n  int
}

func show(c Counter) { // copies the mutex along with the struct
	fmt.Println(c.n)
}

func main() {
	c := Counter{n: 1}
	show(c)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_ = ctx // cancel is never called: the timer and its goroutine leak
	_ = cancel

	fmt.Printf("processed %d records\n", "many") // wrong verb for the argument
}
```

```console
$ go vet vetdemo.go
vetdemo.go:27:24: fmt.Printf format %d has arg "many" of wrong type string
vetdemo.go:15:13: show passes lock by value: command-line-arguments.Counter contains sync.Mutex
vetdemo.go:21:7: call of show copies lock value: command-line-arguments.Counter contains sync.Mutex
```

Three findings, no configuration, no false positives to triage. (The lost `cancel` is caught by the `lostcancel` check as soon as the variable is used normally rather than parked in a blank assignment — vet is conservative on purpose.)

## The mismatch

What vet actually covers is worth reading once, because it defines the class of bug you no longer have to review for by eye: `printf` verb and argument mismatches across the whole `fmt`-style family, including your own functions when they wrap it; `copylocks`, which is the enforcement behind "types containing a `sync.Mutex` take pointer receivers" (`waitgroup-and-mutex`); `structtag`, which catches a malformed `json:"..."` string the compiler sees as an ordinary string (`struct-tags`); `lostcancel` for context leaks (`context-cancellation`); `unusedresult` for discarding the result of `fmt.Sprintf` or `errors.New`; and `httpresponse`, `nilfunc`, `shift`, and a dozen more. Two habits follow. First, run `go vet ./...` in CI next to `go build` and `go test` — it is fast, and its findings are not negotiable the way `perlcritic` policies are. Second, add `staticcheck` when the project is more than a script: it subsumes vet, adds real dead-code and simplification analysis, and is the closest thing the community has to a standard linter, while `errcheck` (or staticcheck's equivalent check) restores the one guarantee the compiler declines to give you — that every returned error is looked at (`if-err-nil-rhythm`). What has no Go equivalent, and needs no replacement, is the style half of `perlcritic`: `gofmt` already settled layout (`toolchain-gofmt-godoc`), and naming conventions live in code review and https://go.dev/wiki/CodeReviewComments rather than in a rule engine.

Further reading: https://pkg.go.dev/cmd/vet and https://staticcheck.dev/docs/checks/
