# 23-perl-idioms

The awkward-idioms stress test: every construct here is common in real
Perl and painful for a mechanical converter.

## Constructs exercised
- `local $TRACE_DEPTH = $TRACE_DEPTH + 1` on an `our` global inside
  recursion -- dynamic scoping with automatic restore, proven by the
  indentation of the trace log (leave lines are OUTSIDE the local scope)
- `@_` aliasing: `embiggen($low, @sizes)` multiplies the CALLER's scalar
  and array elements in place; `swap` exchanges two caller variables via
  `@_[0,1] = @_[1,0]` slice assignment
- `wantarray` three ways: list assignment, scalar assignment, and
  `my ($first) =` which is list context (the classic trap)
- `goto &render_v2` -- tail call replacing the current frame, forwarding
  `@_` untouched
- `do { if/elsif/else }` block as an expression yielding a value
- `unless` statement modifier and an `until` loop with `$n--` inside
- memoized fibonacci: closure over `%cache` + self-referential coderef
  (`my $self; $self = sub { ... $self->(...) }`) built inside a `do` block
- `sprintf/printf %*d` runtime widths; `%vd` on a v-string (`v2.14.7`),
  `length` of a v-string (3 chars!), string-compare `ge` on v-strings
- slices galore: hash slice `@config{qw(host port)}`, kv-slice
  `%config{'user','ssl'}` (5.20+), array slice ranges, mixed
  positive/negative indices `@week[0,-1]`, computed slice via `grep` on
  indices, and slice ASSIGNMENT writing two keys at once
- `map`/`grep` pipeline with ternary inside `map`

## Conversion challenges
- `@_` aliasing is the #1 semantic gap: Go is pass-by-value; converter
  must detect mutation-through-arguments and switch to pointers or
  multiple returns -- `sizes=10 20 30` in expected output catches it
- `local` = save/restore of a global with unwind safety -> defer pattern
- `wantarray` has no Go analogue; call sites must be split into two
  functions or an explicit mode
- `goto &sub` must become a plain tail call (behaviorally fine) -- but
  converters that panic on goto fail outright
- v-strings are char-vectors, not floats: `length == 3`, `%vd`, `ge`
  ordering all break float-based translations
- kv-slice/negative-index/slice-assignment forms all need distinct Go
  constructions

## Go teaching opportunities
- pointers vs values, defer for scoped state, closures for memoization,
  explicit multiple returns replacing context sensitivity
