# 46 - the record shapes deliberately left as maps

## What this exercises
The two cases the record analysis stops short of, kept here as a recorded
target rather than as a pass.

The first is a **named** hash used as a record. `my %server = (host => ...,
port => 5432, healthy => 1)` is every bit as much a record as the same thing
written as a reference, but a named hash is also where `keys`, `delete` and
slices turn up, and turning one into a struct without knowing that first is
how a conversion breaks. Only hash references become structs today.

The second is the set of questions you ask a collection, asked of a record.
`keys` and `values` are answered by writing the field list out, which is
correct and reported. `exists`, `delete` and copying key by key are not: a
struct has no key to delete and no way to grow one at run time.

## Perl constructs
- `my %server = (...)` with mixed value kinds, read by literal key
- `keys`, `scalar keys`, `exists` and `delete` over that hash
- `keys %$job` and `values %$job` over a hash reference that *is* a struct
- `$job->{$k}` inside a loop over the keys
- copying one record into another key by key

## What still goes wrong, and why it is here
`join(',', @{ $server{tags} })` prints Go's default rendering of a slice
rather than the joined text, because `%server` stayed a map of `any` and the
list behind that key lost its type on the way in. Making a named hash a record
fixes that line and several like it.

## Go concepts a converter must teach
- A struct is not a collection: there is nothing to enumerate at run time, and
  the field list is written out and checked by the compiler.
- `delete` has no counterpart at all. A field that can be absent is a pointer
  or a zero value with a separate flag, decided when the type is declared.
- `exists` on a struct field is answered at compile time; on a map it is the
  comma-ok form, which is a different question and a different shape.
- Copying a record is `*b = *a`, one assignment, and it copies the fields
  rather than sharing them, which is the opposite of what copying a hash
  reference does.
