#!/usr/bin/perl
# TRAP: / is ALWAYS floating point in Perl; int() truncates toward zero;
# `use integer` flips a whole lexical scope to C semantics (including %).
use strict;
use warnings;

print "7/2 = ",        7 / 2,          "\n";    # 3.5, never 3
print "-7/2 = ",       -7 / 2,         "\n";    # -3.5
print "int(-7/2) = ",  int( -7 / 2 ),  "\n";    # -3 (toward zero, like Go)
{
    use integer;
    print "integer 7/2 = ",  7 / 2,  "\n";      # 3
    print "integer -7/2 = ", -7 / 2, "\n";      # -3
    print "integer -7%2 = ", -7 % 2, "\n";      # -1 (C rules!)
}
print "plain -7%2 = ", -7 % 2, "\n";            # 1 (Perl rules)

my $avg = ( 3 + 4 ) / 2;
print "avg = $avg\n";    # 3.5 -- a naive Go int division gives 3
