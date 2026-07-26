use strict;
use warnings;

my %stock = (apples => 10, pears => 4, plums => 0, kiwis => 7, figs => 12);

print "--- each, collected then sorted ---\n";
my @rows;
while (my ($k, $v) = each %stock) {
    push @rows, sprintf("%-7s %3d", $k, $v);
}
print "$_\n" for sort @rows;

print "--- keys in scalar context is the count ---\n";
my $n = keys %stock;
print "pairs: $n\n";

print "--- values, aggregated order-independently ---\n";
my $sum = 0;
my $min = 1e9;
my $max = -1;
for my $v (values %stock) {
    $sum += $v;
    $min = $v if $v < $min;
    $max = $v if $v > $max;
}
print "sum=$sum min=$min max=$max\n";

print "--- iterating in a chosen order ---\n";
for my $k (sort { $stock{$a} <=> $stock{$b} or $a cmp $b } keys %stock) {
    printf "%2d %s\n", $stock{$k}, $k;
}

print "--- modifying values while iterating keys is fine ---\n";
for my $k (sort keys %stock) {
    $stock{$k} *= 2;
}
print join(" ", map { "$_=$stock{$_}" } sort keys %stock), "\n";
