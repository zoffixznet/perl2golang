#!/usr/bin/perl
# CASE angle-05: `$a < $b` is a comparison. Distinguished from readline by lexer
# state: `<` in OPERATOR position is less-than; in TERM position it may open a
# readline.
use strict; use warnings;

my ($a, $b) = (3, 7);
print "angle-05 lt: ", ($a < $b ? "yes" : "no"), "\n";
print "angle-05 gt: ", ($a > $b ? "yes" : "no"), "\n";
print "angle-05 le-ge: ", ($a <= 3 ? 1 : 0), ($b >= 7 ? 1 : 0), "\n";

# The nasty one: `<` in operator position whose right operand starts with `$`,
# and a `>` later on the same line. A naive readline scan eats `< $b ? ... >`.
my @pairs = ([1,2],[5,4]);
my @lt = map { $_->[0] < $_->[1] ? "L" : "R" } @pairs;
print "angle-05 map: @lt\n";

# String comparison words for contrast.
print "angle-05 string-lt: ", ("abc" lt "abd" ? "yes" : "no"), "\n";
