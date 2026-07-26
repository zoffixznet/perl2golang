#!/usr/bin/perl
# CASE slash-14: `/=` is divide-assign in operator position, but in TERM position
# the same two characters begin a pattern whose first character is `=`.
use strict; use warnings;

my $n = 10;
$n /= 2;
print "slash-14 divide-assign: $n\n";

local $_ = "a=b";
# Term position: this is m/=/ -- a pattern matching a literal `=`.
my $hit = /=/ ? "matched-equals" : "no";
print "slash-14 pattern-equals: $hit\n";

# Explicit form of the same pattern, for the token-stream comparison.
print "slash-14 explicit: ", (m/=/ ? "matched-equals" : "no"), "\n";

# And a substitution whose pattern is `=`.
(my $c = "x=y") =~ s/=/:/;
print "slash-14 subst: $c\n";
