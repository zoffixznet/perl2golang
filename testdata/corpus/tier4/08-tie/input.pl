#!/usr/bin/perl
# TRAP: tie -- a plain-looking scalar whose every read and write secretly
# runs methods. Assignment is STORE, interpolation is FETCH.
use strict;
use warnings;

package UpperScalar;
sub TIESCALAR { my $v = ""; return bless \$v, shift }
sub FETCH     { my $s = shift; print "  (FETCH ran)\n"; return uc $$s }
sub STORE     { my ( $s, $v ) = @_; print "  (STORE ran)\n"; $$s = $v }

package main;

tie my $x, 'UpperScalar';

$x = "quiet";             # looks like assignment; actually calls STORE
print "value: $x\n";      # looks like a read; actually calls FETCH

my $copy = $x;            # EVERY read is a method call
print "copy: $copy\n";
