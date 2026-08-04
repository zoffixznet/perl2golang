#!/usr/bin/perl
use strict;
use warnings;

# Reading and writing an array outside the range it currently has. Perl grows
# on write and answers undef on read; Go panics on both.

my @a = ( 10, 20, 30, 40 );

print "-- reads past the end --\n";
print "in range:   $a[2]\n";
print "past end:   ", ( defined $a[99] ? $a[99] : 'undef' ), "\n";
print "count is unchanged: ", scalar @a, "\n";

my @empty;
print "first of an empty list: ", ( defined $empty[0] ? $empty[0] : 'undef' ), "\n";

my @fields = split /,/, 'one';
print "second field: ", ( defined $fields[1] ? $fields[1] : 'undef' ), "\n";

print "-- writes past the end --\n";
$a[6] = 'fig';
print "after \$a[6]: count=", scalar @a, " last index=$#a\n";
print "the gap at 5 is ", ( defined $a[5] ? 'filled' : 'undef' ), "\n";

print "-- negative indices --\n";
print "last: $a[-1] second-last: ", ( defined $a[-2] ? $a[-2] : 'undef' ), "\n";
$a[-1] = 'end';
print "after \$a[-1] = 'end': $a[-1]\n";

my @lines = ( "a\n", "b\n" );
$lines[-1] .= "continued\n";
print "appended through -1: ", join( '', @lines );

print "-- \$#a as a place --\n";
$#a = 2;
print "after \$#a = 2: ", join( ',', map { defined $_ ? $_ : 'undef' } @a ), "\n";
