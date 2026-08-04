# 65 - closures that take records, and closures that call themselves

## What this exercises
The neighbour of entry 64: three code-reference shapes that still come out
wrong, for two different reasons.

**A closure that takes a record.** A comparator chosen at run time takes its
arguments through `@_`, so it has no way to say what they are, and the
conversion gives it the uniform `func(...any) any` signature. The body then
reads `$_[0]{n}`, which lowers to an assertion that the argument is a map,
while the caller hands it the struct the records became. The program compiles
and dies on the first comparison.

The fix is not a wider assertion. It is that reading a field out of a value of
unknown type has to work for both shapes a record can have, or say so and
carry on, rather than taking the program down.

**A closure that calls itself.** `my $fib; $fib = sub { ... $fib->(...) }` is
how Perl writes a recursive anonymous sub: the name has to exist before the
body can mention it. In Go the same two-step works, `var fib func(int) int`
then `fib = func(n int) int { ... }`, but the declaration needs the signature
written out, and the signature is what the conversion is still trying to
discover from the body that uses it.

**A closure calling its neighbour through the table it lives in.** Same
problem one level out: `%calc` is being built by the expression that reads it.

## Perl constructs
- a ternary choosing between two comparators, called through `sort`
- `$_[0]{field}` inside a closure whose arguments are records
- `my $f; $f = sub { ... $f->(...) }`
- `%h = ( a => sub {...}, b => sub { $h{a}->(...) } )`

## What goes wrong today
The first section panics with `interface conversion: interface {} is *main.Row,
not map[string]interface {}`. The recursive sections do not run at all,
because the program dies before them.

## Go concepts a converter must teach
- A recursive function value is declared before it is assigned, and `var f
  func(int) int` is the line that makes it possible. This is one of the few
  places where `var` beats `:=` outright.
- A type switch reads a value of unknown type without gambling on it, and
  `v, ok := x.(T)` is the same code with the crash removed.
- Where a callback really does take one kind of thing, giving it a concrete
  signature removes every assertion inside it at once.
