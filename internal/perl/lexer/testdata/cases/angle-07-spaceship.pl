#!/usr/bin/perl
# CASE angle-07: `<=>` is one token (numeric compare), NOT `<=` then `>` and NOT
# `<` then `=>`. Longest-match wins over the fat comma.
use strict; use warnings;

my @n = (5, 2, 9);
my @s = sort { $a <=> $b } @n;
print "angle-07 spaceship: @s\n";

print "angle-07 cmp: ", (1 <=> 2), (2 <=> 2), (3 <=> 2), "\n";
print "angle-07 strcmp: ", ("a" cmp "b"), ("b" cmp "b"), ("c" cmp "b"), "\n";

# `<=` and `=>` in close quarters, to force the tokenizer to choose.
my %h = (5 => "five");
print "angle-07 le-and-fatcomma: ", (5 <= 5 ? "le" : "gt"), " ", $h{5}, "\n";

# `<==` does not exist; `<=` then `=` would be a syntax error, so nothing to test,
# but `>=` and `=>` on one line do:
my %g = ( 7 => (7 >= 7 ? "ge" : "lt") );
print "angle-07 ge: $g{7}\n";
