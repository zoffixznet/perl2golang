#!/usr/bin/perl
use strict;
use warnings;

# An array's length, in the three places Perl never has to mention it: a
# two-dimensional table built by writing into it, a loop counting over the
# indices it has, and a read at an index it may not have.

my @word = qw(kitten sitting);
my @a = split //, $word[0];
my @b = split //, $word[1];

# A distance table. Nothing declares its size: every cell is created by the
# write that fills it.
my @d;
for my $i ( 0 .. @a ) { $d[$i][0] = $i }
for my $j ( 0 .. @b ) { $d[0][$j] = $j }
for my $i ( 1 .. @a ) {
    for my $j ( 1 .. @b ) {
        my $cost = $a[ $i - 1 ] eq $b[ $j - 1 ] ? 0 : 1;
        my $best = $d[ $i - 1 ][$j] + 1;
        $best = $d[$i][ $j - 1 ] + 1 if $d[$i][ $j - 1 ] + 1 < $best;
        $best = $d[ $i - 1 ][ $j - 1 ] + $cost
          if $d[ $i - 1 ][ $j - 1 ] + $cost < $best;
        $d[$i][$j] = $best;
    }
}
printf "edit distance %s -> %s is %d\n", $word[0], $word[1],
  $d[ scalar @a ][ scalar @b ];
printf "the table is %d rows of %d\n", scalar @d, scalar @{ $d[0] };

# Counting over the indices an array has: every read is inside it.
print "--- every index the list has ---\n";
for my $i ( 0 .. $#a ) {
    printf "  %d %s\n", $i, $a[$i];
}
print "--- the same count written the other way ---\n";
for my $i ( 1 .. @a ) {
    printf "  %d %s\n", $i, $a[ $i - 1 ];
}

# Counting over one array's indices while reading another: past the end of
# the shorter one is undef, not an error.
print "--- one list's indices against another list ---\n";
for my $i ( 0 .. $#b ) {
    my $left = $a[$i];
    printf "  %d %s/%s\n", $i, ( defined $left ? $left : '-' ), $b[$i];
}
