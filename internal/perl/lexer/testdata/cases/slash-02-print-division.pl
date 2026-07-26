#!/usr/bin/perl
# CASE slash-02: `print $x/2` -- the `/` is division, not a match delimiter,
# because `$x` puts the lexer in expect-operator state even inside a list operator.
use strict; use warnings;

my $x = 10;
print "slash-02 value: ", $x/2, "\n";

# The classic trap: no spaces at all.
print $x/2;
print "\n";

# And the same expression where a naive lexer would swallow `/2, "\n"; print $x/`
# as a regex and then choke.
my @parts = ($x/2, $x/5);
print "slash-02 list: @parts\n";
