# What this entry exercises

Single inheritance through `@ISA`, two levels deep, with `SUPER::` used to
extend a method rather than replace it, and a template method in the base
class that calls a step every subclass overrides.

The template method is the neighbouring case that does not convert. `@ISA`
becomes embedding, and embedding covers everything here except the one line
that matters most: `$self->run` inside `Job::describe` is resolved against
the embedded `Job` when Go compiles it, so the base class's own `run` is
what runs and the override is never reached. The converter reports that at
the call site rather than emitting a dispatch that quietly goes to the wrong
method, and the `late-binding-vs-embedding` lesson gives the shape that does
work: an interface for the varying step, held by the base, set by each
constructor.

Everything else here converts: the two-level chain, the fields promoted
through the embedded parent, `SUPER::` as a call on the embedded value, and
`isa` answered from the hierarchy the file declares.
