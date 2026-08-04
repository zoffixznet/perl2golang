# 32 - the template method

## What this exercises
A base class method that calls another method on its own object, where the
subclass replaces the one being called. Every report generator, exporter and
error hierarchy in Perl is built on it, and it is the one place where
translating `@ISA` to embedding stops being faithful: Perl looks the inner
call up on the object's real class every time, Go resolves it against the type
the receiver is declared as, and inside a base method that is always the base.

`Report::render` calls four methods on `$self` -- `prefix`, `suffix`, `title`,
`format_row` and `style` -- and each subclass replaces a different subset of
them. One base method, three sets of behaviour, and nothing overrides `render`
itself.

The last two lines matter as much as the rest: a bare `Report` with no
subclass in play has to keep working, because a value that never went through
a subclass's constructor has nothing to dispatch through.

## Perl constructs
- `our @ISA` single inheritance, two siblings off one base
- a base method calling five methods on `$self`, three of them overridden
- an override that calls no base method (`style`) and one that reimplements a
  loop (`format_row`)
- an accessor (`title`) called from the base method
- constructing all three classes and calling the same method on each
- a base object used on its own

## Go concepts a converter must teach
- Embedding promotes methods but does not dispatch through them. A base
  method's call on itself is resolved when the base method is compiled.
- The fix is composition plus an interface: an interface naming the methods
  the base calls on itself, a field of that type on the base struct, and each
  constructor storing the finished object in it.
- Reading that field through a one-line accessor that falls back to the
  receiver is what keeps a plain struct literal working, and turns the pattern
  from a trap into a strict improvement on embedding.
- Nothing declares that a type implements the interface. Every class in the
  hierarchy satisfies it because the methods are promoted, and a class that
  replaces one still does.
