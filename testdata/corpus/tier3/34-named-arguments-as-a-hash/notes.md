# 34 - named arguments the constructor never names

## What this exercises
The neighbour of every constructor that reads `$args{title}`. A converter can
turn those into Go parameters, one per key, because the keys are written in
the source. This one never writes a key down: it walks `sort keys %args` and
copies whatever the caller passed, filtering against an allow-list.

There is nothing to turn into a parameter list, because the parameter list is
the data.

## Perl constructs
- `my ( $class, %args ) = @_` where `%args` is used as a whole hash
- `sort keys %args` and `$args{$key}` with a computed key
- a hash field inside the object, holding the fields that survived the filter
- `scalar keys %{ $self->{fields} }`

## What goes wrong today
The converter recovers named parameters from `%args` whenever it sees the
shape, and here there are no static keys to recover, so it emits a constructor
with no parameters and leaves the `keys %args` behind it pointing at a
variable that no longer exists. The generated Go does not compile.

The honest answers are two, and the round that fixes this has to pick one:
keep `%args` as a real hash parameter when the body uses it as a whole, or
take the arguments variadically and rebuild the hash from the pairs, which is
what perl does. Either is better than a name with nothing behind it.

## Go concepts a converter must teach
- Go has no named arguments. Where the keys are known, parameters are the
  better shape and the caller loses nothing; where they are not, a
  `map[string]any` parameter is the honest translation, and it is worth saying
  which of the two is happening and why.
- A struct field holding a map is how a record with a fixed part and an open
  part is written in Go, and it is what this class really is.
