# 85 - a header row zipped against a data row

## What this exercises
The hash slice as a place, which is what half the CSV and TSV scripts in the
world are built on:

```perl
@rec{@header} = @fields;
```

One statement, as many assignments as the header has columns, paired by
position. Everything hard about it is in that sentence. The number of places
is not known until the program runs, so there is no list of targets to write
down; and when the data row is short, the remaining keys are still *there*,
holding undef, which is a different answer from the key being absent.

The rest of the entry is the same construct in its other three positions: a
slice on the right (`@{$rec}{@required}`), `delete` over a slice, which
removes several keys and answers with what it removed, and two lists zipped
into a lookup table where one is a name short.

## Perl constructs
- `@rec{@header} = @fields` with a run-time key list and a short value list
- `@slim{@required} = @{$rec}{@required}`, a slice on both sides
- `my @dropped = delete @first{@optional}`, read for its value
- `defined` distinguishing a key set to undef from a key that is set

## Go concepts a converter must teach
- Go has no list of places. A construct that assigns to several at once
  becomes a loop, and the loop is where Perl's padding rule stops being
  invisible.
- Where a missing value has to be told apart from an empty one, the map holds
  pointers and nil is the absence. That is the whole of `nil-vs-undef` in one
  line of real code, and it is why `map[string]*string` turns up in converted
  output at all.
- `delete` takes one key and returns nothing, so both halves of Perl's
  slice delete are written out; reading the value before removing the key is
  not an optimisation, it is the only order that works.
