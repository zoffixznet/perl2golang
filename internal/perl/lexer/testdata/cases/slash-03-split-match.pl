#!/usr/bin/perl
# CASE slash-03: `split /,/, $s` -- `/` after the list operator `split` starts a MATCH.
# A named list operator leaves the lexer in expect-term state.
use strict; use warnings;

my $s = "a,b,c";
my @f = split /,/, $s;
print "slash-03 split: ", join("|", @f), "\n";

# split with an explicit m// and with a string first argument, for contrast.
my @g = split m{,}, $s;
my @h = split ",", $s;
print "slash-03 mfoo: ", join("|", @g), " string: ", join("|", @h), "\n";

# split ' ' is the magic-whitespace special case (a string, not a pattern).
my @w = split ' ', "  x  y  ";
print "slash-03 magic-space: ", scalar(@w), " fields\n";
