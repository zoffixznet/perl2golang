#!/usr/bin/perl
# CASE slash-01: `/` after a scalar variable is DIVISION.
# The lexer is in expect-operator state after `$b`, so `/` cannot start a match.
use strict; use warnings;

my ($a, $b) = (10, 4);
my $r = $a / $b;
print "slash-01 division: $r\n";

# Whitespace does not change the decision: still operator state.
my $r2 = $a/$b;
my $r3 = $a  /  $b;
print "slash-01 no-space: $r2 spaced: $r3\n";
