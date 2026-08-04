#!/usr/bin/perl
use strict;
use warnings;

# A hash slice on the left of an assignment, and a hash slice as the thing
# being deleted. Both are places rather than values, and Go has no syntax for
# either.

my %conf = ( host => 'db1', port => 5432, user => 'app', pass => 'secret', debug => 1 );

print "-- reading a slice --\n";
my @wanted = @conf{qw(host port)};
printf "read: %s\n", join( ',', @wanted );

print "-- writing a slice --\n";
my %fresh;
my @keys = qw(alpha beta gamma);
my @vals = ( 1, 2, 3 );
@fresh{@keys} = @vals;
printf "built %d key(s): %s\n", scalar keys %fresh,
    join( ' ', map { "$_=$fresh{$_}" } sort keys %fresh );

@fresh{ 'delta', 'epsilon' } = ( 4, 5 );
printf "after a literal slice: %s\n", join( ',', sort keys %fresh );

print "-- a short right-hand side --\n";
my %short;
@short{qw(one two three)} = ( 'x' );
printf "one=%s two=%s\n", $short{one},
    ( defined $short{two} ? $short{two} : 'undef' );

print "-- deleting a slice --\n";
delete @conf{qw(pass debug)};
printf "left: %s\n", join( ',', sort keys %conf );

my @removed = delete @fresh{qw(alpha beta)};
printf "delete returned %d value(s): %s\n", scalar @removed, join( ',', @removed );
printf "fresh now: %s\n", join( ',', sort keys %fresh );
