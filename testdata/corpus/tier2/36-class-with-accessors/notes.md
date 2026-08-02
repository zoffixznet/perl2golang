# What this entry exercises

The shape most scripts write a class in: a package, a constructor that
blesses a hash reference and takes its arguments by name, accessors written
by hand, methods that mutate the object and return it so calls chain, a
second class holding the first in a hash, and a file-scope `my` that every
instance shares.

What it costs to convert:

- the named arguments become positional parameters, in the order the
  constructor reads them, because Go has neither named nor optional ones
- the read-only accessors disappear: the field takes their name and callers
  read it directly, which is what Go code does instead of a getter
- the read/write accessor becomes an assignment to the field
- `isa`, `can` and `ref` are answered from the class hierarchy this file
  declares, so each is a constant in the generated code
- the class-level `my` becomes a package-level variable, because Go has no
  file-scope lexical for the methods to close over
