# 31 - a dispatch table of code refs that disagree about their shape

## What this exercises
The hash-of-subs idiom Perl uses instead of a switch. The handlers here
deliberately differ: one returns a string, one a number, one a list, one
returns nothing at all, and one takes two arguments where the rest take one.
That is what a real table looks like, and it is exactly what a single Go
function type cannot hold.

This is a recorded target rather than a pass. The converter emits Go that
compiles, and calling through the table panics at run time with an interface
conversion error, because each closure keeps the signature inference gave it
while the call site asserts one shape for all of them. A conversion that
compiles and then fails is worse than a refusal, so this entry exists to keep
that visible until it is fixed.

## Perl constructs
- `%handlers` holding anonymous subs, and `ref $handlers{$name} eq 'CODE'`
- calling through the table with `$handlers{$name}->(...)`
- handlers with different arities and different return shapes, including one
  that returns a list read in scalar context and one that returns nothing
- a handler chosen by a name computed while the program runs, which is the
  reason for the table in the first place

## Go concepts a converter must teach
- A map has one value type, so a table of callbacks needs one signature. The
  usual answer is `map[string]func([]any) any`, or a small interface with one
  method per handler kind, and the conversion from Perl's loose shapes to the
  chosen one belongs at the point each handler is written.
- A type assertion on a function value compares the *whole* signature, so
  `func(string) any` and `func(any) string` are unrelated types and neither
  satisfies an assertion to the other.
- A handler that returns nothing cannot share a signature with one that
  returns a value: the Go equivalent returns the zero value, or the table
  splits into two tables.
- `ref $x eq 'CODE'` has no counterpart worth writing: a map of functions
  already holds only functions, and the compiler knows it.
