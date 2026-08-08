# What this exercises

List assignments whose targets do several jobs at once: an inline `my` in
the middle of the list, a hash element as a target, and a re-assignment of
the same variable being split again. A slice of the named captures,
@+{qw(addr service outcome)}. And the two readings that share one
spelling family: $_->[0] dereferencing the topic against $_[0] reading a
sub's first argument.

# What makes it hard

Every target must be stored, by position, whatever shape it has, and a
dropped one must not shift the positions after it. The named-capture
slice has no hash behind it, only numbered groups. And the arrow is the
whole difference between the topic's arrayref and the argument list, so
misreading it folds the wrong value in silently.
