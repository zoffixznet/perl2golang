#!/usr/bin/perl
# CASE interp-08: exactly where interpolation STOPS. This is a maximal-munch
# problem with several special cases; getting the boundary wrong silently changes
# the string rather than failing.
use strict; use warnings;

my $x = "X";
my %h = (a => { b => "AB" }, k => "K");
my @a = ([1,2],[3,4]);

print "interp-08 dot:        [$x.]\n";        # $x then "."
print "interp-08 dot-word:   [$x.y]\n";       # $x then ".y"  (NOT $x.y)
print "interp-08 underscore: [", "${x}_1", "]\n";
print "interp-08 hyphen:     [$x-1]\n";       # $x then "-1"
print "interp-08 colons:     [", "${x}::y", "]\n";
print "interp-08 hash-chain: [$h{a}{b}]\n";   # both subscripts consumed
print "interp-08 arr-chain:  [$a[1][0]]\n";
print "interp-08 space-stops:[$h{k} {b}]\n";  # space ends the chain
print "interp-08 paren-stops:[$x(not a call)]\n";

# A `{` after a scalar is ALWAYS tried as a subscript; escape it to stop that.
print "interp-08 escaped-brace: [$x\{literal}]\n";

# A `[` after a scalar likewise.
print "interp-08 escaped-bracket: [$x\[literal]]\n";

# `$x->` followed by something that is not [ or { stops the chain.
my $r = { f => "F" };
print "interp-08 arrow-then-word: ", ("$r->{f}x" =~ /\AFx\z/ ? "F then x" : "unexpected"), "\n";

# Two consecutive subscripts separated by an arrow and by nothing: same result.
print "interp-08 arrow-optional: ",
      ("$h{a}{b}" eq "$h{a}->{b}" ? "identical" : "different"), "\n";
