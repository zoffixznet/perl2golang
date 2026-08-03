# 30 - error objects caught in $@ and sorted out by asking what they are

## What this exercises
The half of exception-object handling an ordinary script uses: a small class
hierarchy, a `throw` class method, objects landing in `$@`, and the caller
deciding what to do by calling methods and asking `isa`. No destructor and no
operator overloading, so nothing here is out of the converter's declared
scope.

## Perl constructs
- `sub throw { my $class = shift; die $class->new(@_) }` called on four
  different classes, so which type is built depends on the invocant
- `die` with an object rather than a string, and `$@` holding that object
- `$err->isa('Failure::Network')` answered true for `Failure::Timeout`,
  because isa walks the whole chain
- accessors (`detail`, `code`) read through a value whose class the file never
  pins down
- an eval that finishes leaving `$@` empty even though an eval inside it
  failed and was handled
- four classes collected in one array and walked with the same method call

## Go concepts a converter must teach
- `panic` carries a value of any type, so the object survives the unwinding
  and `recover` hands it back whole. Go's own convention is to panic with an
  `error`, so the recovering side has something with an `Error` method.
- `$class` has no Go counterpart inside a function: a function written from
  `throw`'s body would build the parent whatever it was called on. The call
  has to be resolved where the class is written down.
- An interface is what several classes share, and a value of unknown class is
  asserted to it before a method is called on it.
- Embedding is **not** subtyping. `*FailureTimeout` will not satisfy an
  assertion to `*FailureNetwork`, so `isa` becomes a predicate that lists the
  concrete types, and adding a class to the hierarchy means adding it there.
- An interface can promise a method but never a field, so an accessor reached
  through one has to become a real method: the field steps back to the
  unexported name and the method takes the exported one.

## Why this is not expected to be byte-equivalent
`report` calls `$self->label`, and each subclass overrides `label`. Perl looks
that up on the object, so `Failure::Timeout`'s `report` prints `timeout`. Go's
embedding promotes the parent's `Report` method, and inside it the receiver is
the *parent*, which knows nothing about the type that embedded it, so it calls
the parent's `Label`. The converter reports this rather than hiding it, and it
is the single most instructive difference between Perl's method lookup and
Go's embedding. Closing it means giving the base type a field holding the
behaviour, or turning the hierarchy into an interface plus one implementation
per class.
