#!/usr/bin/perl
# The oracle: one line holding the keys a, b and c exactly once each, in any
# order. The order is hash order and legitimately differs run to run.
use strict;
use warnings;

my $line = <STDIN> // '';
chomp $line;
my %seen;
$seen{$_}++ for split /,/, $line;
if ( keys %seen != 3 or grep { !defined $seen{$_} or $seen{$_} != 1 } qw(a b c) ) {
    warn "expected the keys a,b,c exactly once each, got \"$line\"\n";
    exit 1;
}
exit 0;
