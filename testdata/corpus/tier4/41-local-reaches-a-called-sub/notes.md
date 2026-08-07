# What this exercises

`local $/ = '|'` in one block changing what a sub defined elsewhere reads:
dynamic scoping proper, where the localised value travels with the call,
not with the text.

# Why it cannot convert exactly

The conversion folds separators into the reads it can see, which is
lexical. The sub's read was compiled for the default separator, so the
second call disagrees with perl, and the report's note at the local is
required to say exactly that and how to pass the separator explicitly.
