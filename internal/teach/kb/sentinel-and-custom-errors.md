---
id: sentinel-and-custom-errors
title: Sentinel errors and custom error types
tags: [idiom, errors, types]
perl_triggers: [exception-class, exception-class-module, die-object, isa-check, error-code]
severity: info
prerequisites: [error-wrapping, methods-and-receivers]
---

Where Perl grows exception class hierarchies (`Exception::Class`, Moose-based exceptions, `$e->isa(...)` dispatch), Go has two lighter conventions serving the same two needs. Need one: "callers must recognise *this particular* failure" — a **sentinel**, a package-level `var ErrNotFound = errors.New(...)`, matched with `errors.Is`. Need two: "callers need *data* about the failure" — a **custom error type**, any struct with an `Error() string` method, extracted with `errors.As`. Choosing between them is an API decision you will now make routinely, and the naming conventions (`ErrXxx` for sentinels, `XxxError` for types) are strong enough that violating them misleads readers.

## The Perl you know

```perl
package My::NotFound  { use parent 'My::Exception' }
package My::QuotaFull { use parent 'My::Exception';
                        sub user { $_[0]->{user} } }

eval { $svc->lookup($id) };
if    (my $e = $@) {
    if    ($e->isa('My::NotFound'))  { return undef }
    elsif ($e->isa('My::QuotaFull')) { notify($e->user) }
    else                             { die $e }
}
```

## The Go you write

Compiled and run as shown:

```go
package main

import (
	"errors"
	"fmt"
)

// Sentinel: a package-level, comparable error value.
var ErrNotFound = errors.New("user not found")

// Custom type: carries structured data.
type QuotaError struct {
	User  string
	Limit int
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("quota exceeded for %s (limit %d)", e.User, e.Limit)
}

func lookup(id int) error {
	switch id {
	case 1:
		return nil
	case 2:
		return fmt.Errorf("lookup %d: %w", id, ErrNotFound)
	default:
		return &QuotaError{User: "jdoe", Limit: 100}
	}
}

func main() {
	err := lookup(2)
	fmt.Println(errors.Is(err, ErrNotFound))

	err = lookup(3)
	var qe *QuotaError
	if errors.As(err, &qe) {
		fmt.Println("tell", qe.User, "the limit is", qe.Limit)
	}
}
```

```
true
tell jdoe the limit is 100
```

The stdlib's own famous sentinels calibrate the idiom: `io.EOF` (so central that `Read` loops check it by identity), `sql.ErrNoRows`, `fs.ErrNotExist`, `context.Canceled`.

## The mismatch

Differences from exception-class thinking. There is no hierarchy: a `QuotaError` does not "extend" anything, and grouping related errors happens by wrapping a shared sentinel (`fmt.Errorf("%w: quota", ErrLimitExceeded)`) rather than by inheritance — flatter, and it composes with `errors.Is` for free. Sentinels are part of your public API surface: exporting `ErrNotFound` is a permanent compatibility promise, so keep the set small and deliberate — every exported sentinel is a behaviour someone will depend on. Give custom error types pointer receivers and return `*QuotaError` (as `error` — remember `typed-nil-interface`): with value receivers and value returns, two identical `QuotaError` values compare equal, which makes `errors.Is` match errors that merely *look* alike. One habit that does not port: throwing rich exception objects for *control flow* (Perl code sometimes dies with an object to jump out of deep recursion); in Go, errors travel the return path only, and deep-exit is either restructured code or, in one narrow package-internal pattern, a panic recovered at the boundary (`panic-and-recover`). Finally, don't over-engineer: most functions should return wrapped opaque errors (`error-wrapping`); mint a sentinel or type only when at least one real caller will branch on it.

Further reading: https://go.dev/blog/go1.13-errors and https://pkg.go.dev/errors
