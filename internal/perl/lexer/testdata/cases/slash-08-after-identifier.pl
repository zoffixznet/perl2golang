#!/usr/bin/perl
# CASE slash-08: `/` after a bare IDENTIFIER depends on whether the identifier is a
# value-ish term already consumed (operator state) or a list operator / sub name
# still expecting arguments (term state). Perl resolves this with the symbol table.
use strict; use warnings;

use constant TEN => 10;
print "slash-08 constant-div: ", TEN / 2, "\n";      # TEN is a term -> division

sub want_args { return "want_args(" . join(",", @_) . ")" }
local $_ = "a2b";
# `want_args` is a known sub with no prototype => list operator => `/` starts a match.
my $out = want_args /2/;
print "slash-08 listop-match: $out\n";

# A method name after -> is never followed by a pattern slash in practice.
package Obj; sub new { bless {n=>20}, shift } sub n { $_[0]{n} }
package main;
my $o = Obj->new;
print "slash-08 method-div: ", $o->n / 4, "\n";
