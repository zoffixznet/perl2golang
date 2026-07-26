use strict;
use warnings;

my @nums = (7, 3, -7, 3, 7, -3, -7, -3, 10, 4, -10, 4, 13, 5, -13, 5);
while (@nums) {
    my $n = shift @nums;
    my $d = shift @nums;
    printf "%4d %% %-4d = %4d     int(%d/%d) = %4d\n",
        $n, $d, $n % $d, $n, $d, int($n / $d);
}

print "--- the identity Perl guarantees ---\n";
for my $pair ("13:5", "-13:5", "13:-5", "-13:-5") {
    my ($n, $d) = split /:/, $pair;
    my $m = $n % $d;
    my $q = ($n - $m) / $d;
    printf "%-7s mod=%3d  floor-quotient=%3d  q*d+m=%4d\n", $pair, $m, $q, $q * $d + $m;
}

print "--- truncating semantics via use integer ---\n";
{
    use integer;
    print "use integer:  -7 % 3 = ", -7 % 3, "\n";
    print "use integer:   7 % -3 = ", 7 % -3, "\n";
    print "use integer:  -7 / 3 = ", -7 / 3, "\n";
}
print "no integer:   -7 % 3 = ", -7 % 3, "\n";
print "no integer:    7 % -3 = ", 7 % -3, "\n";
