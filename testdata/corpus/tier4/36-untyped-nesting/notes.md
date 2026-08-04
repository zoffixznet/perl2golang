# 36 - a structure whose shape only the running program knows

## What this exercises
Every value under a key in `%doc` is a different kind of thing: a string, a
list, a record, and a list of records whose own fields are lists of lists.
Nothing in the file says which is which, and nothing could: the shape is a
property of the data, not of the code. So the conversion has no type to give
the structure beyond `any`, every read of it is a guess, and several of the
guesses are wrong.

That makes it a tier 4 entry rather than a bug: the standard here is not that
the program prints the right thing, it is that the tool says it could not work
the shape out **and the program still runs to its last line**. A wrong guess
has to leave an empty collection behind and let the next statement have its
turn, because the lines that do not depend on a guess are the ones a reader is
here to learn from, and a stack trace on line four takes all of them away.

## Perl constructs
- a hash whose values are a scalar, an array reference, a hash reference and
  a list of hash references
- `ref $value` dispatch over `''`, `ARRAY` and the rest
- `@$value`, `%$value`, `$doc{meta}{year}`, `@{ $table->{rows} }`,
  `@{ $rows[0] }`: four dereference spellings over the same untyped values
- `defined` on a key that was never set

## What comes out today
The program compiles, runs to the end, and gets four lines wrong: the record
under `meta` reads as empty, the author list prints as a Go slice rather than
joined, and the table walk sees one table instead of two. The report names the
dynamic fallback. Nothing panics, which is the point.

## Go concepts a converter must teach
- `any` is where the compiler stops helping, and every read of one is either a
  guess or a question.
- The comma-ok form is the same code as the assertion with the crash removed,
  and discarding the second result on purpose is a decision about what a wrong
  guess should cost.
- The type switch is the shape to move towards: it never guesses, and each
  branch hands back the value already typed.
- Reading a nil map is legal and yields the zero value, which is what lets an
  empty result propagate quietly rather than compound.
