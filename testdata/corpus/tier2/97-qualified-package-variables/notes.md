# What this exercises

One hash with two spellings: %TALLY inside package Counter and
%Counter::TALLY from main, written and read from both sides, plus an `our`
scalar with an initialiser that main later overwrites by its full name.

# What makes it hard

`our` declares the package variable itself, not a lexical, so the
declaration site and every qualified use must land on one Go variable. A
converter that reads `our` as `my` splits the state in two: the package
counts into one variable while the script reads zeros out of the other,
and the initialiser lands on the copy nothing else sees.
