---
id: typed-nil-interface
title: The typed-nil trap - a nil pointer inside a non-nil interface
tags: [trap, nil, interfaces, errors]
perl_triggers: [return-undef, return-error-object-or-undef, defined-error-check]
severity: trap
prerequisites: [nil-vs-undef, implicit-interfaces, errors-are-values]
---

This is the most famous trap in Go, it has no Perl analogue, and it produces error-handling code that reports failure when nothing failed. An interface value (like `error`) is a two-word pair: a concrete *type* and a *value*. It compares equal to `nil` only when *both* are empty. Return a nil `*MyError` from a function whose declared return type is the concrete pointer, assign it into an `error`, and the interface now holds (type=`*MyError`, value=nil) — a non-nil interface wrapping a nil pointer. Your `if err != nil` fires on success.

## The Perl you know

```perl
sub validate {
    my ($input) = @_;
    return My::Error->new(field => 'input') if $input eq '';
    return undef;                     # undef is undef, full stop
}
my $err = validate("fine");
die $err->message if defined $err;    # correctly does not die
```

Perl has one "nothing" and `defined` never lies about it. There is no way to accidentally wrap `undef` in something that claims to be defined.

## The Go you write

Compiled and run as shown — this program is *wrong on purpose*:

```go
package main

import "fmt"

type ValidationError struct {
	Field string
}

func (e *ValidationError) Error() string {
	return "invalid field: " + e.Field
}

func validate(input string) *ValidationError {
	if input == "" {
		return &ValidationError{Field: "input"}
	}
	return nil
}

func main() {
	var err error = validate("fine") // assigning *ValidationError into error
	fmt.Println(err == nil)
	fmt.Printf("%T %v\n", err, err)
	if err != nil {
		fmt.Println("BUG: we think validation failed")
	}
}
```

```
false
*main.ValidationError <nil>
BUG: we think validation failed
```

`validate` correctly returned nil — and `err == nil` is still `false`, because the conversion to `error` recorded the concrete type `*main.ValidationError`. The `%T %v` line shows the smoking gun: a typed nil.

## The mismatch

The fix is a rule you adopt wholesale: **functions that can fail return the `error` interface type, never a concrete error pointer type**. Written as `func validate(input string) error`, the `return nil` stores a true untyped nil into the interface and the comparison behaves. The trap variant to watch in ported code: a helper populates `var vErr *ValidationError` and later does `return vErr` from an `error`-returning function — same wrapping, same lie; return `nil` explicitly on the success path instead. This mechanism is not error-specific — any interface (`io.Writer`, your own) wrapping a nil pointer is non-nil, and calling its methods may even *work* if the method tolerates a nil receiver — but errors are where 95 percent of real-world encounters happen. When debugging a "why is err non-nil" mystery, `fmt.Printf("%T", err)` is the diagnostic that cracks it open. Perl gave you nothing to unlearn here; this one you simply must know cold.

Further reading: https://go.dev/doc/faq#nil_error
