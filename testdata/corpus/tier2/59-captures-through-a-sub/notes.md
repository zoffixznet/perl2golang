# 59 - captures that leave the sub that made them, which does not convert yet

## What this exercises
The neighbour of entry 58. There the parentheses that ask for a list sit
beside the match, so the conversion can see what shape is wanted. Here they
sit at the call site, one function boundary away:

```perl
sub split_pair { my ($text) = @_; return $text =~ /host=(\w+)\s+port=(\d+)/ }
my ($host, $port) = split_pair($rec);
```

Perl decides at run time, per call. A Go function decides once, in its
signature. So the converter has to pick, and it currently picks the truth
value, which is right for `if (first_word($x))` and wrong for everything else
this entry does.

## Perl constructs
- `return $text =~ /(...)(...)/` as a sub's last expression
- the same sub's result taken as `my ($a, $b) = f(...)`, as `my @got = f(...)`
  and as an `if` condition
- a one-group version of the same sub, where the difference between "the
  capture" and "it matched" is a single character of output

## What goes wrong today, and why it is here
`split_pair` returns a `bool`, so `$host` gets `1`, `$port` gets nothing, and
`scalar(@got)` is 1 for a failed match where Perl says 0. The output is wrong
in a way that still looks plausible, which is the worst kind.

The fix is a choice, not a bug fix: a sub whose only `return` is a capturing
match returns the captures, and callers that only ever ask for truth get
`len(...) > 0`. Deciding that needs a pass over the call sites, which is the
same analysis that would let sub parameters be typed from how they are called.

## Go concepts a converter must teach
- A Go function's result list is part of its type. There is no `wantarray`,
  and there is no way to hand back a different number of values per call.
- Returning `[]string` and testing `len(m) > 0` at the call site is the honest
  translation of a Perl sub used both ways.
- Where a sub really is two functions wearing one name, splitting it into two
  named functions is what a Go developer would do, and it usually reads better
  than the original.
