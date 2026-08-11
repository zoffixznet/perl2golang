#!/usr/bin/perl
use strict;
use warnings;

my %h = ( a => 1, b => 2, c => 3 );
print join( ",", keys %h ), "\n";
