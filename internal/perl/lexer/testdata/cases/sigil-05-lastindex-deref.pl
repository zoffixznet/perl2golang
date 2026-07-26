#!/usr/bin/perl
# CASE sigil-05: `$#$aref` and `$#{$aref}` -- the `$#` sigil followed by a deref.
# Three characters of sigil before any name appears.
use strict; use warnings;

my @a = (1,2,3,4);
my $r  = \@a;
my $rr = \$r;

print "sigil-05 bare: $#a\n";
print "sigil-05 deref-bare: ", $#$r, "\n";
print "sigil-05 deref-braced: ", $#{$r}, "\n";
print "sigil-05 double-deref: ", $#{$$rr}, "\n";
print "sigil-05 postfix: ", $r->$#*, "\n";
print "sigil-05 expr-block: ", $#{ [ 1..10 ] }, "\n";

# All the same value.
print "sigil-05 all-equal: ",
      ($#a == $#$r && $#$r == $#{$r} && $#{$r} == $r->$#* ? "yes" : "no"), "\n";

# `$#` immediately followed by `{` that is a BLOCK-looking expression.
my %of_arrays = (k => [10,20,30]);
print "sigil-05 hash-of-arrays: ", $#{ $of_arrays{k} }, "\n";
