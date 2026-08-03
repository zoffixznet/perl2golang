# What this entry exercises

Hash references used as records: the shape most Perl programs carry their
data in. Every key is a literal known before the program runs, the fields
have different types, and fields are added after construction as well as in
the constructor.

This is the neighbouring case that does not convert well yet. A record whose
keys are all known and whose fields have settled types is a struct, and the
generated Go should say `job.Secs` where the Perl said `$job->{secs}`. What
comes out instead is `map[string]any`, which compiles and runs and costs a
conversion at every read. The entry is here so that the next round has a
target with a recorded expectation behind it.

The last two lines are deliberately the hard part. A record nested inside
another record needs the same decision one level down, and a field read
through a variable key rules a struct out for that record: Go has no way to
reach a field by a name worked out while the program runs, short of
reflection.

What it costs to convert today:

- every field read goes through a conversion, because the map holds `any`
- the numeric fields cannot be added without a conversion first
- nothing catches a typo in a field name, which is the main thing a struct
  would have bought

## Go concepts to teach

- `structs-and-embedding` - what these records should become
- `collections-hold-one-type` - why a map of mixed values costs what it does
- `type-assertions-and-switches` - what every read of one turns into
