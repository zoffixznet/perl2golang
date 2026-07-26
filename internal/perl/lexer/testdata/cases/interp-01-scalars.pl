#!/usr/bin/perl
# CASE interp-01: what a bare `$name` interpolates, and where the name ENDS.
# The name is the longest run of word characters (plus `::`), so `"$x."` is
# $x then a literal dot and `"$x_y"` is the variable $x_y.
use strict; use warnings;

my $x   = "X";
my $x_y = "XY";
$Pkg::v = "PKGV";

print "interp-01 plain: [$x]\n";
print "interp-01 dot-after: [$x.]\n";
print "interp-01 word-after: [$x_y]\n";
print "interp-01 dot-word-after: [$x.y]\n";
print "interp-01 braced-stops: [${x}_y]\n";
print "interp-01 qualified: [$Pkg::v]\n";
print "interp-01 colon-after: [$x:tail]\n";
print "interp-01 digit-after: is [$x] then 1: [${x}1]\n";
print "interp-01 escaped: [\$x]\n";
print "interp-01 backslash-then-var: [\\$x]\n";

# Adjacent variables with no separator.
my $a = "A"; my $b = "B";
print "interp-01 adjacent: [$a$b]\n";

# A `$` at the very end of a string is a literal dollar.
print "interp-01 trailing-dollar: [", "cost: 5\$", "]\n";
