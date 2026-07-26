#!/usr/bin/perl
# CASE quote-09: `q`, `qq`, `qw`, `m`, `s`, `y`, `tr`, `qr` are quote-like
# operators ONLY when they appear as a bare identifier in term position and are
# followed by a delimiter. As `$q`, as a hash key, after `->`, or before `=>`
# they are ordinary names.
use strict; use warnings;

my $q  = "scalar named q";
my $s  = "scalar named s";
my $tr = "scalar named tr";
print "quote-09 scalars: [$q][$s][$tr]\n";

print "quote-09 operator: ", q(the q operator), "\n";

# Barewords before => are autoquoted, even reserved-looking ones.
my %h = (q => 1, qq => 2, qw => 3, m => 4, s => 5, y => 6, tr => 7, qr => 8);
print "quote-09 fatcomma-keys: ", join(",", map { "$_=$h{$_}" } sort keys %h), "\n";

# Same names as hash subscripts (autoquoted inside {}).
my %g = (s => "sub", y => "why");
print "quote-09 subscripts: $g{s} $g{y}\n";

# Same names as method names after ->.
package P; sub new { bless {}, shift } sub q { "method q" } sub s { "method s" } sub tr { "method tr" }
package main;
my $o = P->new;
print "quote-09 methods: ", $o->q, " / ", $o->s, " / ", $o->tr, "\n";

# `q` immediately followed by `(` with no space is the operator; `$q` never is.
print "quote-09 empty-q: [", q(), "] len=", length(q()), "\n";
