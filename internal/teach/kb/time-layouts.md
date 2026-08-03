---
id: time-layouts
title: Date formats are an example date, not strftime codes
tags: [trap, time, formatting, stdlib]
perl_triggers: [localtime, gmtime, time, strftime, posix-strftime, mktime, timelocal, timegm, setlocale, tzset, sleep, datetime, time-hires, epoch-seconds]
severity: trap
prerequisites: [explicit-conversions-no-coercion, errors-are-values]
---

Go has no `%Y`, no `%H`, and no format codes at all. You describe an output format by writing out one specific reference instant the way you want your dates to look, and the package works out the pattern from it. The reference instant is **Mon Jan 2 15:04:05 MST 2006**, which is the sequence 1 2 3 4 5 6 7 if you read it as month, day, hour (12), minute, second, year, zone offset (-0700). Once you have seen that, the system is easy. Until you have, `t.Format("%Y-%m-%d")` looks perfectly reasonable, compiles, runs, and prints the literal text `%Y-%m-%d`, because every character that is not part of the reference date is a literal. There is no error, no warning, and no way for the compiler to help.

## The Perl you know

```perl
use POSIX qw(strftime);

my $now   = time;                                   # epoch seconds
my @parts = localtime $now;                          # 9 elements, and two traps:
my $year  = $parts[5] + 1900;                        #   year is since 1900
my $month = $parts[4] + 1;                           #   month is 0..11

my $stamp = strftime "%Y-%m-%d %H:%M:%S", localtime $now;
my $short = strftime "%a %b %e", localtime $now;
sleep 2;
```

## The Go you write

```go
package main

import (
	"fmt"
	"time"
)

func main() {
	// A fixed instant, so this program prints the same thing every run.
	// time.Now() is what a real program calls.
	t := time.Date(2026, time.March, 9, 15, 4, 5, 123456789, time.UTC)

	// The layout is the reference instant, written the way you want it.
	fmt.Println(t.Format("2006-01-02 15:04:05"))
	fmt.Println(t.Format("Mon Jan _2 03:04:05PM 2006 -0700"))
	fmt.Println(t.Format(time.RFC3339), t.Format(time.DateOnly), t.Format(time.Kitchen))

	// Fractional seconds: .000 keeps trailing zeros, .999 drops them.
	fmt.Println(t.Format("15:04:05.000"), t.Format("15:04:05.999"))

	// The trap, in full view: unknown characters are copied through.
	fmt.Println(t.Format("%Y-%m-%d"))

	// Components are methods, and they are the real numbers.
	fmt.Println(t.Year(), t.Month(), int(t.Month()), t.Day(), t.Weekday(), t.YearDay())

	// Parsing uses the same layout, and reports failure as an error.
	when, err := time.Parse("2006-01-02 15:04", "2026-03-09 08:30")
	fmt.Println(when.Format(time.RFC3339), err)
	_, err = time.Parse("2006-01-02", "09/03/2026")
	fmt.Println(err)

	// Durations are a type, not a number of seconds.
	d, _ := time.ParseDuration("1h30m")
	fmt.Println(d, d.Minutes(), t.Add(d).Format(time.TimeOnly), t.Sub(when))

	fmt.Println(t.Before(when), t.Unix())

	ny := time.FixedZone("EST", -5*60*60)
	fmt.Println(t.In(ny).Format("2006-01-02 15:04:05 MST"))
}
```

```
2026-03-09 15:04:05
Mon Mar  9 03:04:05PM 2026 +0000
2026-03-09T15:04:05Z 2026-03-09 3:04PM
15:04:05.123 15:04:05.123
%Y-%m-%d
2026 March 3 9 Monday 68
2026-03-09T08:30:00Z <nil>
parsing time "09/03/2026" as "2006-01-02": cannot parse "09/03/2026" as "2006"
1h30m0s 90 16:34:05 6h34m5.123456789s
false 1773068645
2026-03-09 10:04:05 EST
```

The parse error is worth reading twice. It names the layout, the input, and the exact component that did not line up, which makes a format mismatch a two-second diagnosis rather than a session with a debugger.

## The reference date, component by component

| You want | Write | Not |
| --- | --- | --- |
| four-digit year | `2006` | `%Y` |
| two-digit year | `06` | `%y` |
| zero-padded month | `01` | `%m` |
| short month name | `Jan` | `%b` |
| zero-padded day | `02` | `%d` |
| space-padded day | `_2` | `%e` |
| day, no padding | `2` | `%-d` |
| 24-hour hour | `15` | `%H` |
| 12-hour hour | `03` or `3` | `%I` |
| minute, second | `04`, `05` | `%M`, `%S` |
| AM or PM | `PM` or `pm` | `%p` |
| weekday name | `Mon`, `Monday` | `%a`, `%A` |
| zone offset | `-0700`, `-07:00` | `%z` |
| zone name | `MST` | `%Z` |
| RFC 3339 Z form | `Z07:00` | none |

`15` is the only unambiguous hour: it is 3pm in 24-hour form, so it can mean nothing else. `03` is the same hour in 12-hour form, which is why the AM/PM marker matters when you use it.

## The codes that have no layout at all

Most of `strftime` maps across one code at a time: `%Y` is `2006`, `%m` is `01`, `%d` is `02`, `%H` is `15`, `%M` is `04`, `%S` is `05`, `%y` is `06`, `%b` is `Jan`, `%B` is `January`, `%a` is `Mon`, `%A` is `Monday`, `%I` is `03`, `%p` is `PM`, `%Z` is `MST`, `%z` is `-0700`. The compound ones are just longer layouts: `%F` is `2006-01-02`, `%T` is `15:04:05`, `%R` is `15:04`.

A handful have no layout, because they are not a way of writing part of the date but a *derived number*. Those become method calls:

| code | Go |
|---|---|
| `%j` day of the year | `t.YearDay()`, formatted `%03d` |
| `%w` weekday number | `int(t.Weekday())` |
| `%u` weekday, Monday as 1 | `int(t.Weekday())` with Sunday remapped to 7 |
| `%U` / `%W` week number | arithmetic on `t.YearDay()` and `t.Weekday()` |
| `%V` ISO week | `_, week := t.ISOWeek()` |
| `%s` epoch seconds | `t.Unix()` |

A second trap comes with the layout system, and it is the reason a mechanical translation of a format can go wrong: **literal text in a layout is only literal if it does not spell a field.** `t.Format("Day 2 of the month")` replaces the `2` with the day of the month, which is what you wanted, and would also replace a `15` or a `2006` anywhere in the surrounding words, which is not. You cannot see which characters are live by looking. `2006-01-02T15:04:05Z` is safe because a bare `Z` is only a field when digits follow it, and a bare `T` is never one. When a format is genuinely mixed, formatting the parts with `fmt.Sprintf` around a `Format` call is clearer than a clever layout.

The third thing to know before porting: **Go's time formatting is not locale-aware and has no setting to make it so.** `%A` and `%B` are always English. That is a guarantee rather than a gap, because it means a program formats identically on every machine and `setlocale` has nothing to do; `golang.org/x/text/message` is where localised formatting lives when it is genuinely wanted.

## The mismatch

Beyond the layout, the differences are all improvements you have to opt into. `time.Time` is a value with a location attached, not a list: `t.Year()` is 2026 rather than 126, and `t.Month()` is `time.March`, a typed constant that prints as `March` and converts to `3` only when you ask. `localtime` in list context has no equivalent and no need for one. `time.Now()` gives the wall clock in the local zone, `time.Now().UTC()` the other one, and epoch seconds come from `t.Unix()` (with `UnixMilli`, `UnixNano`, and `time.Unix(sec, nsec)` for the trip back).

Durations are the part that catches people. `time.Duration` is an `int64` count of nanoseconds with its own type, so `time.Sleep(2)` compiles and sleeps two nanoseconds: you must write `time.Sleep(2 * time.Second)`. Arithmetic is by method, since Go has no operator overloading: `t.Add(d)`, `t.Sub(u)` (which returns a Duration), `t.Before(u)`, `t.After(u)`, and `t.Equal(u)` rather than `==`, because two Times can be the same instant with different locations. `time.Since(start)` is the stopwatch idiom and is what a benchmark or a timing log should use.

Time zones need the system database: `time.LoadLocation("America/New_York")` reads the zoneinfo files and returns an error when they are absent, which is the usual surprise inside a minimal container (importing `time/tzdata` embeds a copy in your binary and fixes it). `time.FixedZone` is the dependency-free stand-in used above. Finally, formatting a `time.Time` with `%v` or `Println` gives the full `2006-01-02 15:04:05.999999999 -0700 MST` form, useful for debugging and never what you want in a report, so `Format` is not optional there either. JSON encodes it as RFC 3339 automatically (`encoding-json`), which is one decision fewer than any Perl date module makes you take.

Further reading: https://pkg.go.dev/time#pkg-constants
