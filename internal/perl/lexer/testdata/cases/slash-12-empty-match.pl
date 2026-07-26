#!/usr/bin/perl
# CASE slash-12: `//` in TERM position is an EMPTY PATTERN, which re-uses the last
# successfully matched pattern. Identical spelling to defined-or; only lexer state
# tells them apart.
use strict; use warnings;

my $s = "hello";
$s =~ /ell/;                   # last successful pattern is now /ell/

my $t = "xelloy";
print "slash-12 empty-match: ", ($t =~ // ? "reused-ell" : "no"), "\n";

my $u = "nothing here";
print "slash-12 empty-nomatch: ", ($u =~ // ? "bad" : "correctly-no"), "\n";

# The same two characters, one line apart, meaning different things:
my $x;
print "slash-12 dor-on-next-line: ", ($x // "dflt"), "\n";
