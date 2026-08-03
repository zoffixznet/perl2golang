# 45 - hash references used as records

## What this exercises
The shape most Perl programs carry their data in, in the four places it turns
up: a list of records, a lookup that holds records, a record nested inside
another, and a field named by a value rather than written out.

## Perl constructs
- a sub that returns a hash reference with a fixed set of keys, called three
  times so the type has to be shared between the constructor and its callers
- a field added after construction (`$r->{fahrenheit} = ...`), which widens
  the record rather than making a new one
- a list field appended to through `push @{ $r->{notes} }, ...`
- sorting by a field, which only works once the field has a type
- `$latest{ $r->{station} } ||= { ... }`, which fills a hash with records
- a record inside a record, read two levels deep
- `@{ $readings[0] }{qw(station celsius ok)}`, several fields at once
- `$readings[1]{$field}`, a field named by a variable

## Go concepts a converter must teach
- A hash whose keys are written into the program and whose values are
  different kinds of thing is a struct. A map would need one value type for
  all of them, which could only be `any`, and every read would cost a lookup
  and a conversion.
- Two literals with the same keys are the same type, which is what lets a
  constructor function return `*Reading` and the callers hold `[]*Reading`.
- A hash of records is `map[string]*Reading`, and `||=` on a missing key is
  the `if m[k] == nil` shape.
- A slice over a record's keys is a list literal of field selectors, and it
  holds `any` because the fields do not all have the same type.
- A struct field cannot be reached by a name computed at run time. The
  generated reader switches over the names, which is what a Go program writes
  instead of reaching for reflection, and what comes back is `any`.
