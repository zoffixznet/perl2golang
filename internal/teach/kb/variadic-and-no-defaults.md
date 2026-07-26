---
id: variadic-and-no-defaults
title: Variadics, no default arguments, no overloading
tags: [idiom, functions, variadic, defaults]
perl_triggers: [args-hash, named-arguments, shift-default, arg-flattening]
severity: info
prerequisites: [multiple-return-values, structs-and-embedding]
---

Perl's argument handling is one flexible mechanism — everything flattens into `@_`, and you build named arguments, defaults, and arity-dispatch on top by hand. Go gives you exactly one flexibility: a final `...T` variadic parameter, typed and homogeneous. There are no default parameter values, no keyword arguments, and no overloading — two functions may not share a name even with different signatures — so the Perl `%args`-hash idiom needs a designed replacement, and the standard one is an options struct, whose zero values (`static-types-and-zero-values`) play the role your `// 'default'` expressions used to.

## The Perl you know

```perl
sub report {
    my ($name, %args) = @_;
    $args{format} //= 'text';
    $args{limit}  //= 10;
    ...
}
report("sales");
report("sales", limit => 50);
```

## The Go you write

Compiled and run as shown:

```go
package main

import "fmt"

func sum(xs ...int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total
}

type ReportOpts struct {
	Format string
	Limit  int
}

func report(name string, opts ReportOpts) string {
	if opts.Format == "" {
		opts.Format = "text" // defaults live in code, not in signatures
	}
	if opts.Limit == 0 {
		opts.Limit = 10
	}
	return fmt.Sprintf("%s as %s, limit %d", name, opts.Format, opts.Limit)
}

func main() {
	fmt.Println(sum(1, 2, 3))
	nums := []int{4, 5, 6}
	fmt.Println(sum(nums...)) // spread a slice into variadic args

	fmt.Println(report("sales", ReportOpts{}))
	fmt.Println(report("sales", ReportOpts{Limit: 50}))
}
```

```
6
15
sales as text, limit 10
sales as text, limit 50
```

Overloading is a redeclaration error:

```go-invalid
package main

func greet(name string)        {}
func greet(name string, n int) {}

func main() {}
```

```
./overload_err.go:4:6: greet redeclared in this block
	./overload_err.go:3:6: other declaration of greet
```

## The mismatch

Three specifics. First, `xs ...int` is *not* `@_`: it must be last, it is homogeneous, and inside the function it is an ordinary `[]int` — and the spread `sum(nums...)` is the only unflattening; Go never merges `f(a, slice...)`-style mixtures beyond that one form, and passing a slice *without* `...` to a variadic is a type error, not a one-element call. Second, the options-struct pattern shown is the mainstream default-arguments replacement; its known weakness — a caller cannot distinguish "explicitly zero" from "left unset" — is inherited straight from `omitempty`-style zero-value semantics, and when it matters the field becomes a pointer or the API grows the *functional options* pattern (`WithLimit(50)` closures) you will meet in many libraries; recognise it rather than reinvent it. Third, since there is no overloading, Go APIs encode variants in names — `Print`/`Printf`/`Println`, `ParseInt`/`ParseUint` — and ported Perl subs that dispatch on `ref($_[0])` or `scalar @_` should become two or three honestly named functions, which is the same advice Perl Best Practices gave you anyway; the rare truly-generic call sites use generics (`func Max[T cmp.Ordered](...)`) or `any` plus type switches (`type-assertions-and-switches`), in that order of preference.

Further reading: https://go.dev/ref/spec#Function_types and https://go.dev/ref/spec#Passing_arguments_to_..._parameters
