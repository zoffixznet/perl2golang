---
id: error-wrapping
title: Wrap with %w, test with errors.Is and errors.As
tags: [idiom, errors, wrapping]
perl_triggers: [rethrow, error-string-match, exception-class-hierarchy, carp-confess]
severity: info
prerequisites: [if-err-nil-rhythm]
---

Perl code that inspects failures usually string-matches `$@` - brittle, locale-hostile, and everyone knows it. Go's answer is structural: wrapping an error with `fmt.Errorf("context: %w", err)` builds a *chain* that keeps the original error reachable, and two functions traverse it - `errors.Is(err, target)` asks "is this specific error anywhere in the chain?" and `errors.As(err, &typedVar)` asks "is there an error of this *type* in the chain, and hand it over if so". Ported code that translates `$@ =~ /No such file/` into a Go string match is wrong twice: fragile, and ignoring the mechanism built for exactly this.

## The Perl you know

```perl
my $data = eval { load_config($path) };
if (my $e = $@) {
    if ($e =~ /no such file/i) {        # string-matching an error message
        return default_config();
    }
    die "loading config failed: $e";    # "rethrow" loses the original class
}
```

## The Go you write

Compiled and run as shown:

```go
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

func loadConfig(path string) error {
	if _, err := os.ReadFile(path); err != nil {
		return fmt.Errorf("loading config %q: %w", path, err)
	}
	return nil
}

func main() {
	err := loadConfig("/etc/app/missing.conf")
	fmt.Println(err)

	fmt.Println(errors.Is(err, fs.ErrNotExist)) // matches through the wrapping

	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		fmt.Println("op:", pathErr.Op, "path:", pathErr.Path)
	}

	fmt.Println(errors.Unwrap(err))
}
```

```
loading config "/etc/app/missing.conf": open /etc/app/missing.conf: no such file or directory
true
op: open path: /etc/app/missing.conf
open /etc/app/missing.conf: no such file or directory
```

One `%w` at each level of the call stack produced a message that reads outside-in, *and* preserved both the sentinel identity (`fs.ErrNotExist` matched) and the concrete type (`*fs.PathError` extracted, with structured fields).

## The mismatch

Rules of use. `%w` versus `%v`: both interpolate the error's text, but only `%w` keeps the chain - use `%w` by default; choose `%v` deliberately when you intend to *sever* the chain because the underlying error is an implementation detail callers must not start depending on (an API-design decision, not a formatting one). `errors.Is` is for sentinel *values* (`ErrNotFound`-style - `sentinel-and-custom-errors`); `errors.As` is for *types* you need data out of; plain `err == ErrX` breaks the moment anyone wraps, so audit ported comparisons. `errors.As` takes a pointer to your typed variable (`&pathErr` above - passing the non-pointer is a runtime panic, a rare Go API that checks at runtime). Two things Go does *not* give you here: stack traces (the chain of `%w` contexts is the intended, cheaper substitute - write context strings like `"parsing header"` that compose into a story, and never start them with "error:" or end with punctuation, since they will be mid-sentence when printed) and exception class hierarchies (`errors.Is`/`As` walking a chain replaces `->isa()` walking `@ISA`). Also `errors.Join(err1, err2)` (Go 1.20+) merges multiple failures - the "collect all validation errors" pattern - and both `Is` and `As` traverse the joined tree.

Further reading: https://go.dev/blog/go1.13-errors
