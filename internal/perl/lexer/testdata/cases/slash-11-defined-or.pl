#!/usr/bin/perl
# CASE slash-11: `//` in OPERATOR position is the defined-or operator, not an
# empty pattern. One token `//`, not two `/` tokens and not a match.
use strict; use warnings;

my $undef;
my $v = $undef // "default";
print "slash-11 dor: $v\n";

my $zero = 0;
print "slash-11 dor-zero: ", ($zero // "default"), "\n";   # 0 is defined
print "slash-11 or-zero:  ", ($zero || "default"), "\n";   # 0 is false

# Chained, and mixed with division, on one line.
my $a = undef; my $b = 8;
print "slash-11 mixed: ", (($a // $b) / 2), "\n";
