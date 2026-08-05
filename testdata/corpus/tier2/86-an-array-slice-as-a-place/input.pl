#!/usr/bin/perl
use strict;
use warnings;

# The neighbour of entry 85: the same construct over an array rather than a
# hash. An array slice as a place has one thing a hash slice does not, which
# is that the indices decide how long the array has to be.

my @row = ( 'a', 'b', 'c' );

print "-- reading an array slice --\n";
my @picked = @row[ 0, 2 ];
printf "picked: %s\n", join ',', @picked;

my @want = ( 2, 0 );
my @reordered = @row[@want];
printf "reordered: %s\n", join ',', @reordered;

print "-- writing an array slice --\n";
@row[ 0, 1 ] = ( 'A', 'B' );
printf "after a literal slice: %s\n", join ',', @row;

my @at = ( 1, 3 );
@row[@at] = ( 'x', 'y' );    # index 3 is past the end and stretches the array
printf "after a computed slice: %s of %d\n", join( ',', @row ), scalar @row;

print "-- a short right-hand side --\n";
my @sparse;
@sparse[ 0, 2, 4 ] = ('only');
{
    no warnings 'uninitialized';
    printf "sparse: [%s] of %d\n", join( ',', @sparse ), scalar @sparse;
}
printf "index 2 is %s\n", ( defined $sparse[2] ? 'set' : 'undef' );

print "-- swapping through a slice --\n";
my @pair = ( 'left', 'right' );
@pair[ 0, 1 ] = @pair[ 1, 0 ];
printf "swapped: %s\n", join ',', @pair;
