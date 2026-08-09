# What this exercises

A slot that genuinely holds different shapes of thing: a hash whose
values are a count, a label, and a nested hashref; a list alternating
strings with hashrefs, walked with a ref() dispatch.

# What makes it hard

Nothing should make this typed, and that is the point: text has a
faithful form for every scalar, but a number beside a whole hash shares
no honest Go type, so these slots must stay `any` rather than being
forced into a string that would stringify a structure. The test is that
the file still converts, compiles, and prints what Perl printed, with
the dynamism contained to the two slots that earned it.
