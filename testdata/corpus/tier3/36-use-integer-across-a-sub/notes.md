# 36 - how far `use integer` reaches

## What this exercises
The neighbour of tier2 entry 87. There the pragma sat around some arithmetic
and the question was what `/` and `%` mean inside it. Here the question is
where "inside it" ends, and the answer catches people out: the pragma is
*lexical*, so it governs the text it encloses and not the calls that text
makes.

- A sub declared outside the pragma keeps floating-point arithmetic even when
  called from inside it.
- A sub declared inside keeps whole-number arithmetic wherever it is called
  from.
- A nested block inherits it, and `no integer` turns it back off for the rest
  of the block it appears in.
- The pragma truncates the *operands* of `+`, `-` and `*` as well as the
  result, so `7.9 + 0.2` is 7 and `2.5 * 3.5` is 6.

Every one of those matches how a Go program is read, because Go's rule is the
same one written differently: the types of the operands decide, and a value
declared `int` stays an int wherever it travels.

## Perl constructs
- `use integer` around a sub declaration and around a call
- an anonymous sub closed over inside the pragma, called outside it
- a nested block inheriting the pragma, and `no integer` cancelling it
- `+`, `-` and `*` on fractional literals inside the pragma

## What goes wrong today
The named sub declared inside the block is refused. Perl hoists every named
sub to its package whatever block it is written in, so `inner_half` is callable
from the file scope below; the converter registers subs by walking the file's
top-level statements, so a `sub` nested inside a bare block is never seen and
the call to it becomes a stub.

That is worth fixing at the level where subs are hoisted rather than at the
call, because the same gap swallows a sub declared inside `if`, inside a loop,
or inside a `BEGIN` block, all of which are ordinary Perl.

## Go concepts a converter must teach
- Go has nothing that changes the meaning of an operator over a region of
  source. Where Perl needs a pragma to say "these are whole numbers", Go says
  it in the type of the variable, once, and every operator downstream follows.
  That is the same trade the language makes everywhere: less to remember about
  where you are, more to write about what you have.
- A function in Go is declared at package level however it is written, so the
  order of declarations does not decide what is callable. Perl agrees, which
  is why a sub inside a block is still a package sub, and it surprises people
  in both languages for opposite reasons.
