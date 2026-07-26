#!/usr/bin/perl
# TRAP: hash key order is RANDOMIZED PER PROCESS. Two runs of this exact
# script print different orders (but the order is stable WITHIN one run).
# A Go map is also random -- but differently random (per iteration, not
# per process); an ordered map would be deterministically wrong instead.
use strict;
use warnings;

my %h = ( alpha => 1, beta => 2, gamma => 3, delta => 4, epsilon => 5 );

print "keys:  ", join( ",", keys %h ), "\n";    # differs run to run
print "again: ", join( ",", keys %h ), "\n";    # same as line above (!)

my $csv = join( ",", map { "$_=$h{$_}" } keys %h );
print "csv: $csv\n";    # any consumer of this output sees a random order
