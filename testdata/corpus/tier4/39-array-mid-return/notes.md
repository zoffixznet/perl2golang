# What this exercises

A sub whose return puts an array between two fixed values:
`return ('tag', @parts, scalar(@parts))`. The values after the array land
at positions only the array's length decides.

# Why it cannot convert exactly

A Go function returns a fixed number of typed values. A slice as the final
result absorbs a tail of unknown length, which is why the trailing form
converts; a run of unknown length in the middle has no fixed-arity spelling
at all. The honest outcomes are an approximation that states what stood in
for the array, or a refusal at the return.
