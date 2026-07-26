---
id: small-stdlib-philosophy
title: The standard library is the ecosystem
tags: [orientation, stdlib, dependencies, culture]
perl_triggers: [use-lwp, use-moose, use-datetime, use-plack, use-try-tiny, use-list-moreutils, cpanm]
severity: info
prerequisites: [go-mod-vs-cpan]
---

Your CPAN reflex — "someone has surely written this, let me search" — inverts in Go: the first question is "is it in the standard library?", and the answer is yes far more often than a Perl core-modules veteran expects. Production HTTP servers and clients, TLS, JSON, templating, cryptography, SQL access, and the test framework all ship in the box and are what real production services use directly, not toy versions you outgrow. The cultural corollary is a wariness of dependencies ("a little copying is better than a little dependency" is a Go proverb), so a Go code review will question an import where a Perl review would question reinventing.

## The Perl you know

Core Perl gives you `File::Spec` and `Getopt::Long`, but a real web service means assembling LWP or Mojo::UserAgent, Plack, JSON::MaybeXS, Moo(se), Try::Tiny, DateTime — dozens of distributions, each a version, an author, and an upgrade policy. CPAN's abundance is Perl's superpower and its operational burden.

```perl
use LWP::UserAgent;          # separate distribution
use Plack::Runner;           # separate distribution
use JSON::MaybeXS;           # separate distribution, wrapping others
```

## The Go you write

A complete HTTP server *and* client exercising it, standard library only — compiled and run as shown:

```go
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
)

func main() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from %s", r.URL.Path)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/reports")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(body))
}
```

```
hello from /reports
```

That is a real multiplexed HTTP/1.1+2 server (`net/http` is what production services deploy behind a load balancer, frameworkless) plus a test harness (`httptest`) — zero entries in `go.mod`.

## The mismatch

The trade is depth for breadth. Go's standard library is not *small* in capability, but it is deliberately narrow in surface: one way to do HTTP, one JSON codec, one testing package, and a refusal to absorb everything (there is no ORM, no Moose-style object system, no date-parsing DWIM like `Time::ParseDate`). Where CPAN offers forty modules with overlapping philosophies, Go usually offers one blessed way plus a handful of widely agreed community packages for genuine gaps (`golang.org/x/...` for semi-official extensions, `github.com/google/go-cmp` for test diffs, database drivers for `database/sql`). Resist two failure modes: importing a utility module for something achievable in five lines of stdlib (the Go reviewer will ask), and hand-rolling crypto or HTTP because "stdlib culture" — the stdlib *is* the sanctioned implementation of those. When you do search beyond it, https://pkg.go.dev is the index, and import counts shown there are the ecosystem's de facto ratings system.

Further reading: https://pkg.go.dev/std
