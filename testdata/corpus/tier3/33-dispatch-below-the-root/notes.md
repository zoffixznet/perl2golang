# 33 - dispatch from halfway down a hierarchy, which does not convert yet

## What this exercises
The neighbour of entry 32. There the method that dispatches was declared at
the top of the hierarchy, so one interface covers every class in it and one
field on the root holds the object. Here `describe` and `children` are
declared on `Node::Container`, one level down, and `Node` knows nothing about
either.

That is the case the conversion still leaves alone. The interface would have
to name methods the root does not have, so the root could not satisfy it, and
the field cannot live anywhere else without one field per level.

The entry also covers `SUPER::describe`, which is the one shape of dispatch Go
expresses directly: asking for the parent's version by name is exactly what
`n.Container.Describe()` does.

## What goes wrong today
`Node::Container::Sorted` gets the base's `children` rather than its own
sorted version, and `Node::Container::Loud` reaches the base `describe`
through `SUPER::` correctly but that base then calls the wrong `kind`. Two of
the four lines are wrong.

## Perl constructs
- a three-level hierarchy with the dispatching method declared at level two
- two siblings at level three, one replacing the dispatched method and one
  replacing the method the dispatcher prints
- `SUPER::describe` calling up one level and having its result transformed
- a base object with none of the machinery in play

## Go concepts a converter must teach
- `SUPER::` is the easy half: an embedded field has a name, and
  `n.Container.Describe()` is the parent's version by name, checked at compile
  time.
- A hierarchy that dispatches from the middle needs the interface at the
  middle, which means the back-pointer has to live where the dispatching class
  can see it. In hand-written Go this is where people stop inheriting and
  start passing the behaviour in as a value.
- Where the conversion cannot express the dispatch, saying so is the right
  answer: the code compiles and runs, and the report names the calls that will
  miss an override.
