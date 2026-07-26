#!/usr/bin/perl
# CASE interp-03: `$array[EXPR]` inside a string. The index is a full expression,
# so `[` after a scalar variable starts a subscript -- which collides with a
# literal `[` and with a regex character class.
use strict; use warnings;

my @a = ("zero","one","two","three");
my $i = 2;

print "interp-03 literal-index: [$a[1]]\n";
print "interp-03 var-index: [$a[$i]]\n";
print "interp-03 expr-index: [$a[$i+1]]\n";
print "interp-03 negative: [$a[-1]]\n";
print "interp-03 call-index: [$a[ int(1.7) ]]\n";
print "interp-03 nested-sub: [$a[ $#a - 1 ]]\n";

# Array of arrays.
my @m = ([1,2],[3,4]);
print "interp-03 aoa: [$m[1][0]] and [$m[1]->[1]]\n";

# A LITERAL `[` right after a scalar must be escaped or separated.
my $s = "S";
print "interp-03 escaped-bracket: [$s\[not an index]]\n";
print "interp-03 braced-then-bracket: [${s}[literal]]\n";

# Whole-array interpolation and a slice.
print "interp-03 whole: [@a]\n";
print "interp-03 slice: [@a[0,2]]\n";
print "interp-03 lastindex: [$#a]\n";
