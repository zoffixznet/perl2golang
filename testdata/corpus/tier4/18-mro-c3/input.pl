#!/usr/bin/perl
# TRAP: multiple inheritance. Default MRO is depth-first (A can shadow C
# in a diamond!); 'use mro "c3"' changes the answer for the SAME shape.
use strict;
use warnings;

package A;
sub new   { bless {}, shift }
sub who   { "A" }
sub hello { "hello from A" }

package B;
our @ISA = ('A');
sub who { "B" }

package C;
our @ISA = ('A');
sub who   { "C" }
sub hello { "hello from C" }

package D;
our @ISA = ( 'B', 'C' );

package D3;
use mro 'c3';
our @ISA = ( 'B', 'C' );

package main;
my $d = D->new;
print "who:      ", $d->who,   "\n";    # B (leftmost)
print "dfs hello: ", $d->hello, "\n";   # DFS: A shadows C (!)
my $d3 = D3->new;
print "c3 hello:  ", $d3->hello, "\n";  # C3: C wins
