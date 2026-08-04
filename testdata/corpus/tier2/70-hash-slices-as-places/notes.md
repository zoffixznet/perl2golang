# 70 - a hash slice as a place, which does not convert yet

## What this exercises
The neighbour of entry 69. A hash slice on the **right** of an assignment is
a list of values and converts already. On the left, or as the thing being
deleted, it is a place, and Go has no syntax for either.

- `@fresh{@keys} = @vals` sets several keys at once, pairing the two lists up
  by position.
- `@fresh{'delta', 'epsilon'} = (4, 5)` is the same with the keys written out.
- A short right-hand side leaves the extra keys present and undef, which is a
  real difference from doing nothing.
- `delete @conf{qw(pass debug)}` removes several keys and hands back their
  values, in key order.

## What goes wrong today
The slice on the left is treated as one key: `@fresh{@keys} = @vals` produces
a single entry called `alpha beta gamma` holding the whole list. The delete
removes nothing. Both are silent: the program runs, prints plausible output,
and is wrong.

Each of these is a loop of two or three lines in Go, which is why the
conversion is worth writing rather than refusing:

```
for i, k := range keys {
	fresh[k] = at(vals, i)
}
```

## Go concepts a converter must teach
- There is no bulk assignment into a map and no plan for one. The loop is the
  idiom, and it makes the pairing-by-position visible where Perl hid it.
- `delete(m, k)` takes one key and returns nothing, so removing several and
  keeping their values is a loop that reads before it deletes.
- Reading a slice of a map is `pick`-shaped: one loop, one append, and a
  missing key gives the value type's zero value rather than undef.
