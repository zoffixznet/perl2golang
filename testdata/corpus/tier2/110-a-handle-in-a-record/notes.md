# What this exercises

A handle as one field of a record that also holds text and a counter: the
shape of a logger or a downloader written without a class. The record is
built in a sub, the handle is opened into `$sink->{fh}` after the record
exists, and both the record and the handle outlive the sub that made them.

# What makes it hard

The field is written by an `open` rather than by an assignment, so unless
the open counts as evidence about the field, the record's struct either
loses the field entirely or gives it a type nothing can be stored in.
Mixing a `*os.File` with a string and an int in one record is also the case
that decides whether the record becomes a struct at all: a map cannot hold
all three without becoming a map of `any`, which would put an assertion
around every use of every field.
