#!/usr/bin/perl
# CASE slash-07: `/` after `)` is DIVISION (a closing paren ends a term).
# But `/` after `(` is a MATCH, because `(` re-enters expect-term state.
use strict; use warnings;

my @a = (1,2,3,4);
my $avg = ($a[0] + $a[3]) / 2;
print "slash-07 after-rparen: $avg\n";

my $c = scalar(@a) / 2;
print "slash-07 after-call-rparen: $c\n";

local $_ = "hay/stack";
my $m = (/stack/ ? "yes" : "no");    # `(` then `/` -> match
print "slash-07 after-lparen: $m\n";

# Nested: division inside a call whose argument list also contains a match.
sub pick { return "$_[0]:$_[1]" }
print "slash-07 mixed: ", pick(10/5, (/hay/ ? 1 : 0)), "\n";
