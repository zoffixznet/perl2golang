#!/usr/bin/perl
# CASE slash-09: `/` after a low-precedence word operator (`and`, `or`, `not`, `x`,
# `lt`, `eq`, ...) is a MATCH: word operators leave the lexer in expect-term state,
# even though they look like identifiers.
use strict; use warnings;

local $_ = "hello world";
my $r = (1 and /world/) ? "and-match" : "and-nomatch";
print "slash-09 $r\n";

my $r2 = (0 or /hello/) ? "or-match" : "or-nomatch";
print "slash-09 $r2\n";

my $r3 = (not /nope/) ? "not-match" : "not-nomatch";
print "slash-09 $r3\n";

# Contrast: `x` is also a word operator, but here it is in OPERATOR position,
# so what follows is a term (a number), not a pattern.
my $rep = "ab" x 3;
print "slash-09 x-op: $rep\n";
