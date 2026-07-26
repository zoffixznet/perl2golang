#!/usr/bin/perl
# CASE angle-01: `<FH>` is a single readline token, not `<` `FH` `>`.
# Recognised only in TERM position, and only when the contents match
# an identifier / simple scalar / empty.
use strict; use warnings;

open my $w, '>', \my $buf or die;
print $w "one\ntwo\n";
close $w;

open(FH, '<', \$buf) or die;    ## no critic -- bareword handle on purpose
my $first = <FH>;
chomp $first;
my @rest = <FH>;
close FH;
print "angle-01 first: $first rest: ", scalar(@rest), "\n";

# Same handle, explicit readline() call: identical semantics, different tokens.
open(FH, '<', \$buf) or die;
my $l = readline(FH);
chomp $l;
close FH;
print "angle-01 readline: $l\n";
