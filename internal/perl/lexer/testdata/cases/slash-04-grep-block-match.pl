#!/usr/bin/perl
# CASE slash-04: `grep { /foo/ } @l` -- `/` at the start of a statement inside a
# block is a MATCH against $_. `{` puts the lexer back into expect-term state.
use strict; use warnings;

my @l = qw(foo bar foobar baz);
my @g = grep { /foo/ } @l;
print "slash-04 grep-block: ", join(",", @g), "\n";

# Expression form of grep: `/foo/` directly after the comma-less grep EXPR slot.
my @g2 = grep /ba/, @l;
print "slash-04 grep-expr: ", join(",", @g2), "\n";

# map with a division inside, to prove the state machine flips back.
my @n = map { $_ / 2 } (10, 20);
print "slash-04 map-div: ", join(",", @n), "\n";
