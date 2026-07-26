use strict;
use warnings;

print "step 1: starting\n";
my @queue = (4, 2, 0, 9);
my $total = 0;
for my $n (@queue) {
    print "step 2: dividing 100 by $n\n";
    if ($n == 0) {
        print "step 3: refusing to divide by zero\n";
        die "cannot divide by zero\n";
    }
    $total += 100 / $n;
}
print "never reached, total=$total\n";
