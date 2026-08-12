# What this exercises

`my $t = @ARGV && $ARGV[0] =~ /^\d+$/ ? shift @ARGV : 3;`, the canonical
optional leading argument, plus `//` and `||` with a sub call on the right
whose run is observable, because it prints.

# What makes it hard

The shift is a side effect that lives inside one arm of the ternary. It
must not run unless its arm is chosen, and the test must see @ARGV as it
was before the arm touched it, so the lowering cannot hoist the arm's
statements above the if it builds. The same discipline applies to the right
side of `//` and `||`: Perl only evaluates it when the left side did not
answer, and this entry's fallback sub prints when it runs, so an eager
evaluation changes the output rather than just wasting a call.
