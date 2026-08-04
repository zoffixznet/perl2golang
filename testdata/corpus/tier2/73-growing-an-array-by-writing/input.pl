#!/usr/bin/perl
use strict;
use warnings;

# An array used as a sparse table: writes land wherever they land, the array
# stretches to fit, and the slots nobody wrote hold undef rather than zero.

my @slot = ( 'a', 'b', 'c' );

print "--- writing past the end stretches the array ---\n";
$slot[5] = 'f';
printf "length %d, last index %d\n", scalar @slot, $#slot;
for my $i ( 0 .. $#slot ) {
    printf "  %d %s\n", $i, ( defined $slot[$i] ? $slot[$i] : '(gap)' );
}

print "--- reading past the end changes nothing ---\n";
my $far = $slot[20];
printf "read index 20: %s, length still %d\n",
    ( defined $far ? 'a value' : 'undef' ), scalar @slot;

print "--- the last element by negative index ---\n";
$slot[-1] = 'F';
printf "last is now %s\n", $slot[-1];
$slot[-2] .= '!';
printf "second last is now %s\n", ( defined $slot[-2] ? $slot[-2] : '(gap)' );

print "--- setting the length directly ---\n";
$#slot = 1;
printf "after truncating: [%s]\n", join( ',', @slot );
$#slot = 3;
{
    no warnings 'uninitialized';
    printf "after re-extending: [%s], length %d\n", join( ',', @slot ), scalar @slot;
}
printf "index 3 defined: %s\n", ( defined $slot[3] ? 'yes' : 'no' );

print "--- counting into a table nobody sized ---\n";
my @tally;
$tally[2] = 0;
$tally[2]++;
$tally[4]++;
{
    no warnings 'uninitialized';
    printf "tally: [%s], length %d\n", join( ',', @tally ), scalar @tally;
}
