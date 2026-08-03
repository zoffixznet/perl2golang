# 47 - taking a timestamp apart and putting it back together

## What this exercises
The two things a report does with a time: split it into fields, and format it.
Everything here is UTC and the epoch is written into the program, so the
transcript is the same wherever and whenever it runs.

The second half is the set of questions `Scalar::Util` asks about a scalar,
which is the other place a Perl program reaches for run-time inspection that
Go mostly answers at compile time.

## Perl constructs
- `gmtime $epoch` in list context, and the two offsets that catch everyone:
  the month counts from zero and the year counts from 1900
- an array slice of the result used as `printf` arguments, which Perl flattens
  into the verbs
- `strftime` with formats that map onto a Go layout: `%Y %m %d %H %M %S %a %b
  %A %B %I %p`
- `strftime` with formats that do not: `%j` and `%w`, and a mixed one
- `strftime '...', gmtime($x)` written as one expression
- `reftype`, which reports what a reference is made of, and
  `looks_like_number`, including the messy cases `12abc` and `' 7 '`
- `blessed` on a reference that was never blessed

## Go concepts a converter must teach
- A moment is a `time.Time`, and every part of it is a method on that value.
  Nothing takes it apart into a list, because nothing needs to, and the two
  offsets disappear along with the list.
- A layout is an example timestamp rather than a set of percent codes: it is
  always `Mon Jan 2 15:04:05 MST 2006`, whose fields are the numbers 1 to 7 in
  order. That is the whole trick, and it is checked by eye rather than from a
  table.
- The codes with no layout are methods instead: `t.YearDay()`, `t.Weekday()`,
  `t.ISOWeek()`.
- Go's time formatting is not locale-aware at all. Month and day names are
  English and there is no locale to set, which is a guarantee rather than a
  gap: the program formats the same on every machine.
- Perl flattens every argument into one list before matching the format, so an
  array fills as many verbs as it has elements. Go passes arguments one at a
  time, which is why a list argument has to be written out or read back by
  position.
