#!/usr/bin/perl
# TRAP: assigning past the end silently grows the array with undefs, and
# assigning to $#a truncates or extends it. Go slices panic on
# out-of-range assignment; append() has different growth semantics.
use strict;
use warnings;

my @a = ( 1, 2, 3 );
$a[7] = 8;                                    # no panic: 3..6 become undef
print "len: ", scalar(@a), "\n";              # 8
print "gap defined? ", ( defined $a[5] ? "yes" : "no" ), "\n";
{
    no warnings 'uninitialized';
    print "joined: [", join( ",", @a ), "]\n";
}

$#a = 1;                                      # TRUNCATE by assigning to $#a
print "truncated: @a\n";

$#a = 4;                                      # ...and re-extend
print "re-extended len: ", scalar(@a), ", last defined? ",
    ( defined $a[4] ? "yes" : "no" ), "\n";
