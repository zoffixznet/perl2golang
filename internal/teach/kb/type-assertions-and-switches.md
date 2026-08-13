---
id: type-assertions-and-switches
title: any, type assertions, and type switches replace ref()
tags: [gotcha, interfaces, any, type-switch]
perl_triggers: [ref, ref-eq-array, ref-eq-hash, blessed, reftype, polymorphic-scalar]
severity: warning
prerequisites: [implicit-interfaces, comma-ok-idiom]
---

`any` (alias for `interface{}`) is Go's "could be anything" type - the closest thing to an untyped Perl scalar - but unlike a scalar you can do *nothing* with an `any` except find out what it really is and get it back out. The tool for that is the type assertion `v.(T)`, and it has a safety catch you must respect: the one-result form *panics* on mismatch, while the comma-ok form reports failure gently. Perl's `ref($x) eq 'ARRAY'` dispatch becomes the type switch, which is `ref()` with compiler support: each branch gives you the value *already at its concrete type*, no casting after the check.

## The Perl you know

```perl
sub describe {
    my ($v) = @_;
    return "nothing"                 if !defined $v;
    return "array of " . scalar(@$v) if ref $v eq 'ARRAY';
    return "hash"                    if ref $v eq 'HASH';
    return "object: " . ref $v       if Scalar::Util::blessed($v);
    return "plain: $v";
}
```

## The Go you write

Compiled and run as shown - including the deliberate panic at the end:

```go-fails
package main

import "fmt"

func describe(v any) string {
	switch x := v.(type) {
	case nil:
		return "nothing"
	case int:
		return fmt.Sprintf("int %d", x)
	case string:
		return fmt.Sprintf("string %q with %d bytes", x, len(x))
	case []string:
		return fmt.Sprintf("slice of %d strings", len(x))
	case error:
		return "error: " + x.Error()
	default:
		return fmt.Sprintf("unhandled %T", x)
	}
}

func main() {
	fmt.Println(describe(nil))
	fmt.Println(describe(42))
	fmt.Println(describe("hi"))
	fmt.Println(describe([]string{"a", "b"}))
	fmt.Println(describe(3.14))

	var v any = "hello"
	s, ok := v.(string)
	fmt.Println(s, ok)
	n, ok := v.(int) // comma-ok form: no panic on mismatch
	fmt.Println(n, ok)

	_ = v.(int) // no ok: mismatch panics
}
```

```
nothing
int 42
string "hi" with 2 bytes
slice of 2 strings
unhandled float64
hello true
0 false
panic: interface conversion: interface {} is string, not int

goroutine 1 [running]:
main.main()
	/.../typeswitch.go:35 +0x2ea
```

Inside each `case`, `x` has that branch's static type - `len(x)` on the `[]string` branch compiles because the compiler *knows*. A `case error:` matches interface satisfaction, not just concrete types; `%T` prints the dynamic type and is your best debugging friend (`typed-nil-interface`).

## When a wrong guess must not be fatal

There is a third position between "assert and hope" and "check every read", and it is the one a converted program lands in. Where a value's type is genuinely unknown, every read of it is a guess, and some of those guesses will be wrong on the first row of real data. The single-result assertion turns each of those into a stack trace, which loses not only the wrong line but every correct line below it. The comma-ok form with the second result deliberately discarded keeps the program alive:

```go
package main

import "fmt"

// as reads a value of no fixed type as a T, and gives T's zero value when it
// holds something else. It is the comma-ok form with the "ok" thrown away on
// purpose: the caller has already decided that a wrong guess is not fatal.
func as[T any](v any) T {
	t, _ := v.(T)
	return t
}

func main() {
	doc := map[string]any{
		"title":   "Quarterly report",
		"authors": []any{"ada", "grace"},
		"meta":    map[string]any{"year": 2024},
	}

	// The guess is right, and the read costs nothing.
	fmt.Println(as[map[string]any](doc["meta"])["year"])

	// The guess is wrong. An assertion would stop here; this yields an empty
	// map, and the program carries on to say so.
	meta := as[map[string]any](doc["title"])
	fmt.Println(len(meta), meta["year"] == nil)

	// Reading a whole structure without ever guessing: the type switch says
	// what each value is and hands it back already typed.
	for _, key := range []string{"title", "authors", "meta", "missing"} {
		switch v := doc[key].(type) {
		case string:
			fmt.Printf("%s: text %q\n", key, v)
		case []any:
			fmt.Printf("%s: list of %d\n", key, len(v))
		case map[string]any:
			fmt.Printf("%s: record of %d\n", key, len(v))
		case nil:
			fmt.Printf("%s: not there\n", key)
		default:
			fmt.Printf("%s: something else (%T)\n", key, v)
		}
	}
}
```

```
2024
0 true
title: text "Quarterly report"
authors: list of 2
meta: record of 1
missing: not there
```

Three things are worth taking from that. Reading a nil map is legal and gives the zero value, so an empty result propagates quietly instead of compounding. `case nil:` is a real branch of a type switch and matches an interface holding nothing, which is the `defined` test written where Go can see it. And the type switch is the shape to move towards: it never guesses, each branch has the value already at its concrete type, and the `default` branch is where an unexpected shape gets reported instead of crashing.

Prefer the type switch. Reach for the tolerant read when the surrounding code has no branch to offer and the honest answer is "carry on with nothing". Never use the single-result assertion on data whose shape you do not control.

## The mismatch

The cultural instruction comes first: `any` is a last resort, not a convenience. Perl programs are built on polymorphic scalars; Go programs that pass `any` around lose the compiler and gain assertions at every use site - legitimate uses cluster around true dynamism (JSON of unknown shape (`encoding-json`), `fmt`-style APIs, caches), and generics (`func f[T any](x T)`) took over the "same logic, many types" cases, so post-1.18 code reaching for `any` for that purpose is dated style. Mechanics: assertions work *only* on interface types (asserting on a concrete `int` variable is a compile error - no downcasting of non-interfaces exists, because a concrete type is never anything else); the panicking single-result form is correct only when a mismatch genuinely means a programmer bug, so default to comma-ok. Mapping from Perl: `ref` eq 'ARRAY'/'HASH' → `case []T:` / `case map[K]V:` - note you match *specific* element types (`[]string`, not "any slice"; matching all slices needs reflection, a door better left closed); `blessed($v) && $v->isa('X')` → `case *X:`; `reftype` has no equivalent because Go types do not have an underlying "kind" you dispatch on in ordinary code. And the assert-to-capability-interface trick - `v.(interface{ Flush() error })` - is the typed `can()` (`implicit-interfaces`).

Further reading: https://go.dev/ref/spec#Type_assertions and https://go.dev/ref/spec#Type_switches
