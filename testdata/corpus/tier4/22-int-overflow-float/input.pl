#!/usr/bin/perl
# TRAP: Perl numbers walk IV -> UV -> NV(double) silently. Past IV max
# they go unsigned (still exact); past UV max they become doubles and
# LOSE PRECISION with no error. Go int64 would wrap or refuse; neither
# matches. And ** always returns an NV, even for small integers.
use strict;
use warnings;

my $big = 9223372036854775807;      # 2**63-1: largest IV
$big += 1;                          # promotes to UV: still exact
print "IVmax+1: $big\n";            # 9223372036854775808 (Go: wraps negative)

my $huge = 18446744073709551615;    # UV max
$huge += 1;                         # NOW it silently becomes a double
print "UVmax+1: $huge\n";           # 1.84467440737096e+19

print "absorbs +1000? ", ( $huge == $huge + 1000 ? "YES" : "no" ), "\n";

my $x = 2**53;                      # ** returns an NV even here
print "2**53: $x\n";                # e-notation: %.15g stringification
print "2**53+1 == 2**53? ", ( $x + 1 == $x ? "YES" : "no" ), "\n";
