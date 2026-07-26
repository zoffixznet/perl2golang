#!/usr/bin/perl
# CASE bare-04: `NAME:` at statement start is a LABEL. `::` inside a name is a
# package separator. `? :` is the ternary. `:` also appears in sub attributes and
# in `?:` inside a hash literal. Four different meanings of the same character.
use strict; use warnings;

my @hits;
OUTER: for my $i (1..3) {
  INNER: for my $j (1..3) {
    next OUTER if $j == 2;
    last OUTER if $i == 3;
    push @hits, "$i.$j";
  }
}
print "bare-04 labels: @hits\n";

# A label immediately followed by a block.
DONE: {
  push @hits, "block";
  last DONE;
}
print "bare-04 labeled-block: $hits[-1]\n";

# Ternary, including one whose branches are barewords before =>.
my $t = 1 ? "yes" : "no";
my %h = ( key => (0 ? "a" : "b") );
print "bare-04 ternary: $t $h{key}\n";

# Package separator in the same statement as a ternary.
$Foo::val = "pkg";
print "bare-04 mixed: ", (1 ? $Foo::val : "other"), "\n";

# A label that looks like a package name.
Foo: for (1) { }
print "bare-04 label-named-like-package: ok\n";

# `?:` with no spaces next to a `::` name.
print "bare-04 tight: ", (1?$Foo::val:"x"), "\n";
