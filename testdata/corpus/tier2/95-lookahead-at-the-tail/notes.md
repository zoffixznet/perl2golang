# What this exercises

Patterns whose last element is a lookaround: the commify idiom
`s/(\d{3})(?=\d)/$1,/g` over a reversed string, a country-code strip
guarded by `(?=\d{10}$)`, route normalising with `(?=/|$)`, the dotfile
test `/^\.(?!\.?$)/` with its negative assertion, and a substitution whose
count is read.

# What makes it hard

RE2 has no lookaround at all. The tail position is the one that
decomposes: two ordinary patterns and a scan that tests the assertion
after each match without consuming what it saw, sliding one position when
the test fails. The scan never retries a match shorter, which is stated
in the report; these patterns keep what the match consumes and what the
assertion expects disjoint, so the scan is exact. A lookaround anywhere
else still refuses (tier4 19 holds those).
