---
id: accept-interfaces-return-structs
title: Accept interfaces, return structs
tags: [idiom, interfaces, api-design]
perl_triggers: [dependency-injection, mock-object, test-mockobject, passing-filehandle-to-sub]
severity: info
prerequisites: [implicit-interfaces, io-reader-writer]
---

This is Go's API-design proverb, and it answers a question Perl never made you ask because everything was dynamically substitutable anyway: which side of a function signature should be abstract? Answer: parameters accept interfaces (so callers can pass a file, a network stream, or a five-byte test string), while return values are concrete structs (so callers get the full documented API without guessing what they really received). Following it gives you, for free, the testability Perl needed `Test::MockObject` and monkey-patching to achieve — a test double in Go is just another type with the right methods.

## The Perl you know

```perl
sub first_line {
    my ($fh) = @_;            # duck-typed: anything <>-able works
    my $line = <$fh>;
    chomp $line;
    return $line;
}
# tests pass an in-memory handle:
open my $fh, '<', \"first\nsecond\n";
say first_line($fh);
```

Perl gets substitutability implicitly. Go must *choose* it in the signature — and this idiom is that choice made consistently.

## The Go you write

Compiled and run as shown:

```go
package main

import (
	"fmt"
	"io"
	"strings"
)

// Accept an interface: callers can pass files, sockets, buffers, test strings.
func firstLine(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(string(data), "\n")
	return line, nil
}

// Return a concrete struct: callers get the full API, documented and visible.
type Report struct {
	Title string
	Rows  []string
}

func NewReport(title string) *Report {
	return &Report{Title: title}
}

func main() {
	line, err := firstLine(strings.NewReader("first\nsecond\n"))
	fmt.Println(line, err)

	r := NewReport("Q3")
	r.Rows = append(r.Rows, "revenue up")
	fmt.Println(r.Title, len(r.Rows))
}
```

```
first <nil>
Q3 1
```

Had `firstLine` taken `*os.File`, testing it would require a real file on disk; taking `io.Reader`, the test passes a `strings.Reader` and the production caller passes the file — same function, zero mocking framework.

## The mismatch

Why each half. Accepting interfaces: an interface parameter documents the *minimum* capability the function needs (`io.Reader` says "I only read"), widens the caller's options, and is the entire Go substitute for Perl's mock/monkey-patch testing culture — dependency injection here means "the constructor takes a small interface", e.g. a `Storer` interface with `Save`/`Load` implemented by both the real database type and a ten-line in-memory map type in tests. Returning structs: a concrete return exposes all methods and fields, lets the library *add* methods later without breaking anyone, and keeps godoc useful; returning an interface hides capabilities behind the narrowest contract and, done reflexively, resurrects `typed-nil-interface` bugs. The proverb has honest exceptions you should recognise rather than fight: `error` is an interface and is always returned as one; constructors in plugin-style architectures return interfaces deliberately; and do not *over*-abstract parameters either — a function needing five methods of `*os.File` should just take `*os.File`. The smell to avoid from day one: defining a fat interface that mirrors your struct's whole method list and returning that — it is Java import duty, not Go, and reviewers will say so.

Further reading: https://go.dev/wiki/CodeReviewComments#interfaces
