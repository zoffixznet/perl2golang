#!/usr/bin/perl
# CASE sigil-03: `@$x`, `@{$x}`, `%$x`, `%{$x}` and their slice forms. The `@`/`%`
# followed by `$` or `{` is a dereference, not two separate tokens.
use strict; use warnings;

my @a = (1,2,3);
my %h = (x=>10, y=>20);
my $ar = \@a;
my $hr = \%h;

print "sigil-03 array-deref: @$ar / @{$ar}\n";
print "sigil-03 count: ", scalar(@$ar), " ", scalar(@{$ar}), "\n";
print "sigil-03 hash-deref: ", join(",", map { "$_=$$hr{$_}" } sort keys %$hr), "\n";
print "sigil-03 hash-braced: ", join(",", sort keys %{$hr}), "\n";

# Slices: @$ar[0,1] is an array slice of the deref; @{$hr}{qw(x y)} is a hash slice.
my @aslice = @$ar[0,1];
my @hslice = @{$hr}{qw(x y)};
print "sigil-03 array-slice: @aslice hash-slice: @hslice\n";

# %-slices (key/value slices, 5.20+).
my %kv = %$hr{'x'};
print "sigil-03 kv-slice: ", join(",", map { "$_=$kv{$_}" } sort keys %kv), "\n";

# Postfix deref forms.
print "sigil-03 postfix: ", join(",", $ar->@*), " / ", join(",", sort keys $hr->%*), "\n";

# `@{ EXPR }` with a real expression inside the braces.
print "sigil-03 expr-block: ", join(",", @{ [ map { $_*2 } @a ] }), "\n";
