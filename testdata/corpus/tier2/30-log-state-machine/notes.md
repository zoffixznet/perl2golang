# 30 - a line-oriented state machine

## What this exercises
Parsing a build log with `BEGIN`/`END` markers: an explicit state variable, a
"current record" that only exists inside a state, transition logging, and
consistency checks that `die` on malformed input.

## Perl constructs
- `my $state = 'OUTSIDE';` with `if ($state eq 'OUTSIDE') { ... next }` blocks
- `my $current;` holding a hashref only while inside a stage, and `undef
  $current;` on exit
- building a record incrementally: `$current->{end} = $lineno;` after creation
- `push @{ $current->{failures} }, $1;` and `$current->{warnings}++` - mutating
  a nested structure through the cursor
- `die "stage mismatch at line $lineno: ..."` as an assertion
- a post-loop invariant check: `die "log ended while still inside ..."`
- several mutually exclusive `elsif` branches each with its own capture:
  `/^END STAGE (\S+) (\S+)$/`, `/^WARNING (.+)$/`, `/^FAILURE test (\S+)$/`,
  `/^\[(\d\d):(\d\d)\]\s+(.*)$/`
- arithmetic on captures: `$1 * 60 + $2` (string captures used as numbers)
- `sprintf('%3ds %s', ...)` inside a push
- `grep { /^(warn|fail):/ }` filtering the accumulated detail lines
- `map { @{ $_->{failures} } } @stages` - flattening a list of arrayrefs
- `%counts` autovivified by result string

## Go concepts a converter must teach
- The state variable is a `type state int` with `const (...iota)`, and the
  branches become a `switch` - a clear improvement the converter can make.
- `my $current;` being `undef` outside a stage maps to a `*Stage` pointer with
  a nil check; using a value type would lose the "not in a stage" state.
- **`$1` is only valid immediately after a successful match**, and here each
  `elsif` branch relies on its own match. In Go each branch needs its own
  `FindStringSubmatch` result variable - a converter must not hoist a shared
  `matches` variable across branches.
- Numeric use of string captures needs `strconv.Atoi` per capture.
- `die` inside a parse loop is a fatal assertion - in Go, either `panic` (rare)
  or an `error` return that unwinds the loop. The converter must decide, and
  note that Perl's `die` here is *not* caught by anything.
- Flattening `map { @{ $_->{x} } }` is a nested append loop.
- The ordered `@transitions` log exists because hash order is not stable -
  the same reasoning applies in Go.
