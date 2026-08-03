# 48 - time as a quantity, which does not convert yet

## What this exercises
The other half of working with time: differences, durations, rounding to a
boundary, going from a written-out date back to an epoch, and stepping by
whole calendar units. Perl does all of it with plain numbers, because a time
*is* a number of seconds to it. Go has two types, `time.Time` and
`time.Duration`, and will not let you add one to the other by accident.

`timegm` is the blocker: it is the inverse of `gmtime` and there is no rule
for it yet.

## Perl constructs
- `$end - $start` as a plain number, and the divmod that turns it into hours,
  minutes and seconds
- `$t - $t % 3600` to round a moment down to a boundary
- `timegm(sec, min, hour, mday, mon, year)` turning fields back into an epoch
- bumping `$g[4]` to step a whole month, which is not a fixed number of
  seconds
- comparing and sorting moments with numeric operators

## Go concepts a converter must teach
- Subtracting two `time.Time` values gives a `time.Duration`, which is its own
  type. `d.Hours()`, `d.Minutes()` and `d.String()` are what read it, and the
  divmod disappears.
- `t.Truncate(time.Hour)` is the rounding, and it works on the duration since
  the zero time rather than on an epoch, which is the same answer for hours
  and *not* the same for months.
- `time.Date(...)` is `timegm`, and it normalises out-of-range fields rather
  than refusing them, which is how you add a month to the 31st and land where
  you expect.
- `t.AddDate(0, 1, 0)` is the calendar step and `t.Add(45 * 24 * time.Hour)`
  is the fixed one. They are different operations and Go makes you say which.
- `t.Before(u)`, `t.After(u)` and `t.Equal(u)` are the comparisons; `<` does
  not compile, and `==` compares the wall clock, the monotonic reading and the
  location together, which is almost never what was meant.
