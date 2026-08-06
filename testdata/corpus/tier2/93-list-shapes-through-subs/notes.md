# What this exercises

Values that change shape at a boundary: a sub returning one fixed cost and a
tail of crates in a single flat list, unpacked with `my ($cost, @crates)`;
the same call routed through `eval { }` so the failure path hands back the
empty list; a memoised recursive sub whose body ends in `//=` rather than a
return; a flat list assembled from a scalar, an array and a list slice; and
a parenthesised ternary in a value position, which is grouping and not a
one-element list.

# What makes it hard

Go has no flat lists and no context. The return needs a fixed-results-then-
slice signature, the eval needs to evaluate its block in the caller's list
context, the `//=` needs the sub to hand back what the assignment stored,
and the parenthesised ternary must not become a one-element slice, which
would read as 0 in the max comparison and settle the wrong element.
