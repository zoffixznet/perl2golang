#!/usr/bin/perl
# CASE slash-15: `/` right after a block-taking builtin's closing `}` returns to
# OPERATOR state relative to the whole expression, while `/` at the start of that
# block's body is a term. Same brace, two different follow-on states.
use strict; use warnings;

my @n = (10, 4, 8);
my @sorted = sort { $a <=> $b } @n;
print "slash-15 sorted: @sorted\n";

# The result of the sort feeds a division.
my $half = (sort { $a <=> $b } @n)[-1] / 2;
print "slash-15 div-after-sort: $half\n";

# Block body starting with a match, block result feeding a division count.
my @words = qw(alpha beta gamma);
my $count = grep { /a$/ } @words;
print "slash-15 grep-count-div: ", $count / 2, "\n";

# do BLOCK then division.
my $d = do { 12 } / 3;
print "slash-15 do-block-div: $d\n";
