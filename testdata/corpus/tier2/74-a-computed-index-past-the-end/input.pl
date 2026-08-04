#!/usr/bin/perl
use strict;
use warnings;

# The same stretching and the same tolerant reads, but with an index the
# converter cannot see the value of.

my @bucket = ( 0, 0, 0 );

print "--- writing at a computed index ---\n";
for my $n ( 1, 4, 9, 2 ) {
    my $where = $n % 7;
    $bucket[$where] += $n;
}
{
    no warnings 'uninitialized';
    printf "buckets: [%s], length %d\n", join( ',', @bucket ), scalar @bucket;
}
for my $i ( 0 .. $#bucket ) {
    printf "  %d %s\n", $i, ( defined $bucket[$i] ? $bucket[$i] : '(gap)' );
}

print "--- reading at a computed index past the end ---\n";
my @three = ( 'x', 'y', 'z' );
for my $step ( 1, 3, 5 ) {
    my $i = $step * 2;
    my $v = $three[$i];
    printf "index %d: %s\n", $i, ( defined $v ? $v : 'undef' );
}
printf "the array is still %d long\n", scalar @three;
