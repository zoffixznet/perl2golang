#!/usr/bin/perl
# CASE angle-06: `->` is one token. `$a - $b > $c` contains `-` and `>` as two
# separate tokens. Maximal munch must not glue `-` `>` across whitespace, and must
# glue them when adjacent.
use strict; use warnings;

my $h = { k => 5 };
print "angle-06 arrow: ", $h->{k}, "\n";

my ($a, $b, $c) = (10, 3, 5);
print "angle-06 minus-gt: ", (($a - $b > $c) ? "yes" : "no"), "\n";

# Adjacent but semantically a subtraction followed by comparison:
print "angle-06 tight: ", (($a-$b > $c) ? "yes" : "no"), "\n";

# `$a->$b` with a method name in a variable, and `-$x` unary minus.
package P; sub new { bless {}, shift } sub hi { "HI" }
package main;
my $o = P->new; my $meth = "hi";
my $x = 4;
print "angle-06 dynamic-method: ", $o->$meth, " unary-minus: ", -$x, "\n";
