#!/usr/bin/perl
# CASE num-03: v-strings. `v5.10` is a string of chr(5).chr(10); `5.010` is a
# number; a bare `1.2.3` (two or more dots) is also a v-string. `use v5.36` is a
# version declaration, not either.
use strict; use warnings;

my $v = v5.10;
print "num-03 vstring-ords: ", join(",", map { ord } split //, $v), " len=", length($v), "\n";

my $n = 5.010;
print "num-03 number: $n\n";

my $three = v1.2.3;
print "num-03 v-three: ", join(",", map { ord } split //, $three), "\n";

my $bare = 1.22.333;
print "num-03 bare-three-part: ", join(",", map { ord } split //, $bare), "\n";

# sprintf %vd is the readable form.
printf "num-03 vd: %vd and %vd\n", v5.10, 1.22.333;

# $^V is a version object.
print "num-03 perl-version: ", sprintf("%vd", $^V), "\n";
print "num-03 dollar-rbracket: $]\n";

# Version comparisons in `use`.
my $out = `$^X -e 'use v5.10; print "v5.10 ok";' 2>&1`;
$out =~ s/\s+\z//;
print "num-03 use-version: [$out]\n";
my $out2 = `$^X -e 'use 5.010; print "5.010 ok";' 2>&1`;
$out2 =~ s/\s+\z//;
print "num-03 use-number: [$out2]\n";
