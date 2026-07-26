# 04-numeric-arithmetic

## What this exercises
The five arithmetic operators plus `**`, `int` truncation toward zero,
`abs`, `sqrt`, floating point representation error, `%.0f` round-half-to-even,
and accumulation loops. Also shows that `/` in Perl is always floating point
division -- `10/5` prints `2`, not `2.0`, because Perl stringifies an integral
float without a decimal point.

## Perl constructs
- `+ - * / % **`, `**` is right-associative and returns a float
- `int` truncates toward zero (so `int(-3.7)` is -3, not -4)
- `sqrt`, `abs`
- `printf "%.17f"` to expose binary floating point
- statement-modifier `for` used as an accumulator loop

## Go concepts a converter must teach
- **Division is the big one.** Perl `/` on two integers yields a float
  (`17/5 == 3.4`). Go's `/` on two ints is integer division (`17/5 == 3`).
  Every Perl `/` must be lowered to float division unless the converter can
  prove an `int()` wraps it.
- `**` has no Go operator: it becomes `math.Pow` (float) and needs care when
  the Perl code expects an exact integer result.
- `int()` is truncation toward zero == Go's `int(f)` conversion, *not*
  `math.Floor`. Getting this wrong flips the sign case.
- **Number stringification differs.** Perl formats a number with `%.15g` when
  interpolating or printing, so `2**53` prints as `9.00719925474099e+15` and
  `2**62` as `4.61168601842739e+18` -- `**` always produces a float (NV) even
  for integral results. Go's `%v` on a float64 uses the shortest round-trip
  form (`9.007199254740992e+15`) and on an int64 prints all digits. A converter
  needs a `perlNumToString` helper implementing `%.15g` with the trailing-zero
  and exponent-format cleanup Perl does, not `strconv.FormatFloat(f, 'g', -1, 64)`.
- `10/5` prints `2` in Perl even though the value is a float, because `%.15g`
  drops the fractional part. Naively emitting Go `fmt.Sprintf("%v", 2.0)` gives
  `2` as well, but `fmt.Sprintf("%f", ...)` gives `2.000000` -- pick carefully.
- `printf "%.0f"` uses the C library's round-half-to-even in both languages, so
  0.5 -> 0 and 1.5 -> 2. Do not lower it to `math.Round`, which rounds half away
  from zero.
