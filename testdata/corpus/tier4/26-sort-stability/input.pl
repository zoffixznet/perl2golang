#!/usr/bin/perl
# TRAP: Perl's sort is a stable mergesort in practice -- equal-keyed
# elements keep their input order. Go's sort.Slice is NOT stable.
# Also: Perl comparators return any integer; Go wants a bool less().
use strict;
use warnings;

my @records = (
    [ "carol", 30 ], [ "alice", 25 ], [ "dave", 30 ],
    [ "bob",   25 ], [ "erin",  30 ],
);

# Sort by age ONLY. Ties keep input order: carol,dave,erin and alice,bob.
my @by_age = sort { $a->[1] <=> $b->[1] } @records;
print join( " ", map { "$_->[0]:$_->[1]" } @by_age ), "\n";

# Descending via swapped $a/$b -- ties again keep input order.
my @desc = sort { $b->[1] <=> $a->[1] } @records;
print join( " ", map { $_->[0] } @desc ), "\n";
