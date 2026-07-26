#!/usr/bin/perl
# TRAP: negative indices count from the END (arrays and substr), substr
# takes a negative LENGTH, and out-of-range reads give undef, not a
# panic. Go slices panic on every one of these.
use strict;
use warnings;

my @a = ( 10, 20, 30, 40 );
print "last: $a[-1]\n";
print "second-last: $a[-2]\n";
$a[-1] = 99;                                      # negative-index WRITE
print "after write: @a\n";

my $s = "hello";
print "substr -3:   ", substr( $s, -3 ),    "\n"; # llo
print "substr -3,2: ", substr( $s, -3, 2 ), "\n"; # ll
print "neg length:  ", substr( $s, 1, -1 ), "\n"; # ell (negative LENGTH)

print "oob read: ", ( $a[10] // "undef (no panic)" ), "\n";
