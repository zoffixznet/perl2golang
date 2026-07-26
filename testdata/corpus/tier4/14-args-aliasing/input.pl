#!/usr/bin/perl
# TRAP: the elements of @_ ALIAS the caller's variables. A sub that
# writes to $_[0] mutates its caller. Go args are always copies.
use strict;
use warnings;

sub bump { $_[0]++; $_[1] .= "!" }
my ( $n, $s ) = ( 5, "hi" );
bump( $n, $s );
print "n=$n s=$s\n";                  # n=6 s=hi!  -- caller mutated

sub blank_all { $_ = "" for @_ }
my @row = ( "a", "b", "c" );
blank_all(@row);
print "row=[@row]\n";                 # every element blanked in place

sub chompy { chomp $_[0] }            # classic idiom relying on aliasing
my $line = "text\n";
chompy($line);
print "line=<$line>\n";
