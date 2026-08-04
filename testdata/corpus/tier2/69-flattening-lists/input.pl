#!/usr/bin/perl
use strict;
use warnings;

# Perl lists are flat. A list inside a list is not a nested thing, it is more
# elements, and that rule decides how many results half the idioms below
# produce.

my @rows = ( 'north,widget,12', 'south,gizmo,7', 'east,doohickey,40' );

print "-- a block whose value is a list --\n";
my @flat = map { my @f = split /,/; ( $f[0], $f[2] ) } @rows;
printf "%d field(s): %s\n", scalar @flat, join( '|', @flat );

print "-- flattening one level out of a nested structure --\n";
my %by_region = ( north => [ 'a', 'b' ], south => ['c'], east => [ 'd', 'e', 'f' ] );
my @all = map { @{ $by_region{$_} } } sort keys %by_region;
printf "%d item(s): %s\n", scalar @all, join( ',', @all );

print "-- a list repeated --\n";
my @doubled = map { ( $_ ) x 2 } qw(a b c);
printf "%d item(s): %s\n", scalar @doubled, "@doubled";
my @pattern = ( 1, 2 ) x 3;
printf "pattern: %s\n", "@pattern";

print "-- a block whose value is one thing --\n";
my @pairs = map { [ $_, length $_ ] } qw(alpha be c);
printf "%d pair(s), first is %s/%d\n", scalar @pairs, $pairs[0][0], $pairs[0][1];

print "-- merging hashes --\n";
my %defaults = ( host => 'localhost', port => 8080, tls => 0 );
my %site     = ( port => 443, tls => 1 );
my %conf     = ( %defaults, %site, name => 'edge' );
printf "%s\n", join( ' ', map { "$_=$conf{$_}" } sort keys %conf );

print "-- a list in the middle of another --\n";
my @head = ( 'x', 'y' );
my @joined = ( 'start', @head, 'end' );
printf "%d item(s): %s\n", scalar @joined, "@joined";
