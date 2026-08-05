# What this exercises

The handle-position builtins on a real file: an exact-length read of the
magic, tell to remember where the body starts, a negative seek from the end
(whence 2) to pull the fixed-width footer, and a seek back to the saved
position before the line loop. Both seeks carry `or die "...: $!"`, so the
error binding has to reach the failure branch.

# What makes it hard

The line loop starts only after the second seek, so the generated code has to
seek the file before wrapping it in a buffered reader, and the whence
arguments 0 and 2 have to come out as the named io constants for the code to
read like Go.
