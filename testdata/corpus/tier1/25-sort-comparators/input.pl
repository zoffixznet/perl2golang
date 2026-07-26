use strict;
use warnings;

my %score = (alice => 88, bob => 95, carol => 88, dave => 71, erin => 95);
sub by_score_then_name {
    $score{$b} <=> $score{$a} or $a cmp $b;
}

print "--- named comparator over hash keys ---\n";
for my $n (sort by_score_then_name keys %score) {
    printf "%-6s %3d\n", $n, $score{$n};
}
sub numerically { $a <=> $b }
sub by_length_desc { length($b) <=> length($a) or $a cmp $b }

print "--- reusable comparators ---\n";
my @n = (30, 4, 200, 1, 17);
print join(",", sort numerically @n), "\n";
my @w = qw(pear fig banana kiwi apple plum);
print join(",", sort by_length_desc @w), "\n";

print "--- multi-field records ---\n";
sub by_last_then_age {
    my @x = split /,/, $a;
    my @y = split /,/, $b;
    $x[0] cmp $y[0] or $x[2] <=> $y[2];
}
my @recs = ("smith,john,40", "doe,jane,32", "smith,ann,29", "doe,bob,45");
for my $r (sort by_last_then_age @recs) {
    my ($last, $first, $age) = split /,/, $r;
    printf "%-6s %-5s %2d\n", $last, $first, $age;
}

print "--- sorting by hash value, ties broken by key ---\n";
my %pop = (tokyo => 37, delhi => 32, shanghai => 29, dhaka => 23, cairo => 23);
for my $city (sort { $pop{$b} <=> $pop{$a} or $a cmp $b } keys %pop) {
    printf "%-9s %2d\n", $city, $pop{$city};
}
