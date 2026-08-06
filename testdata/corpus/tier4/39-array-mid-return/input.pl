#!/usr/bin/perl
# A sub returning an array in the MIDDLE of its list. The values after the
# array land at positions only the array's length can say, so a fixed-arity
# function signature cannot carry this shape: the tail can absorb a run of
# unknown length, the middle cannot.
use strict;
use warnings;

sub labelled {
    my ($tag) = @_;
    my @parts = split /-/, $tag;
    return ( 'tag', @parts, scalar(@parts) );
}

my ( $kind, @rest ) = labelled('a-b-c');
my $count = pop @rest;
printf "%s: %d parts, %s\n", $kind, $count, join( '+', @rest );

my @flat = labelled('x-y');
printf "flat: %s\n", join( ',', @flat );
