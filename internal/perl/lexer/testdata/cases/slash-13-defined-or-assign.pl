#!/usr/bin/perl
# CASE slash-13: `//=` is a single assignment operator. Maximal-munch over three
# characters, and it only exists in operator position.
use strict; use warnings;

my $u;
$u //= 9;
print "slash-13 dor-assign: $u\n";

my $z = 0;
$z //= 42;                 # 0 is defined, so unchanged
print "slash-13 dor-assign-zero: $z\n";

my %conf;
$conf{retries} //= 3;
$conf{retries} //= 99;
print "slash-13 hash: $conf{retries}\n";

# Contrast with ||= which does fire on 0.
my $y = 0;
$y ||= 42;
print "slash-13 or-assign-zero: $y\n";
