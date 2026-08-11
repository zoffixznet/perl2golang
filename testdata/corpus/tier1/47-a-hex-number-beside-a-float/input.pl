#!/usr/bin/perl
use strict;
use warnings;

# A hex literal is just a number, and nothing stops it landing beside a
# float: file-format modules compare fractional totals against 0x00FFFFFF
# style ceilings all the time.

my $limit = 1.5;
$limit = $limit * 2;

if ( $limit > 0x00FFFFFF ) {
    print "past the ceiling\n";
}
else {
    print "under the ceiling\n";
}

my $mask = 0xFF;
printf "limit %g, mask %d, sum %g\n", $limit, $mask, $limit + $mask;
