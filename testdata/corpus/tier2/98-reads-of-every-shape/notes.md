# What this exercises

Four shapes of read through one handle: a single header line, a
line-at-a-time loop, a continuation read in the middle of the loop guarded
by `and` (which must not read when the left side already failed), and a
`do { local $/; <STDIN> }` slurp of everything after the marker.

# What makes it hard

Each shape alone is easy; sharing the handle is not. A buffered reader per
site would read ahead and swallow lines the other sites were waiting for,
so every read has to go through one buffered reader per handle. The
single-line read must also distinguish the end of input from an empty
line, which is what undef did, and the short-circuit in the loop condition
must keep the read behind the test that guards it.
