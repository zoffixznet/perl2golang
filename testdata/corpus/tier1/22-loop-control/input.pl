use strict;
use warnings;

print "--- next and last ---\n";
for my $n (1 .. 10) {
    next if $n % 2;
    last if $n > 8;
    print "even $n\n";
}

print "--- labelled loops ---\n";
OUTER: for my $i (1 .. 4) {
    INNER: for my $j (1 .. 4) {
        next OUTER if $j > $i;
        last OUTER if $i * $j > 6;
        print "$i x $j = ", $i * $j, "\n";
    }
}

print "--- redo re-runs the body without re-evaluating the list ---\n";
my $tries = 0;
for my $item ('a', 'b') {
    $tries++;
    if ($item eq 'a' && $tries < 3) {
        redo;
    }
    print "handled $item after $tries attempts\n";
}

print "--- last out of a bare block ---\n";
{
    print "in the block\n";
    last;
    print "never reached\n";
}
print "after the block\n";
