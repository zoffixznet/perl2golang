# What this exercises

One slot fed text here and numbers there, in the shapes scripts actually
use: a config hash mixing hostnames with ports, an accumulator that
arrives as a quoted string and is then added to, a report list
alternating labels and counts, and a fixed-width field carved out by
unpack and multiplied.

# What makes it hard

Go gives the slot one type, and the only one that holds every Perl
scalar is its string form. Choosing it is only half the work: every
number flowing in has to be written as text at its source, every
arithmetic use has to read the number back out, and the results have to
print exactly as Perl printed them, integer-looking floats trimmed, or
the equivalence check fails on formatting alone. The unpack template
adds the typed variant of the same decision: an all-text template can
say []string outright.
