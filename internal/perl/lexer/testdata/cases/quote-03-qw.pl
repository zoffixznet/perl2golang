#!/usr/bin/perl
# CASE quote-03: `qw` produces a LIST of words split on whitespace. No
# interpolation, no escapes (a `,` inside is part of a word, which warns).
use strict; use warnings;

my @a = qw(alpha beta gamma);
my @b = qw{one two};
my @c = qw[x y z];
my @d = qw<p q>;
my @e = qw!bang words!;
my @f = qw#hash words#;
my @g = qw/slash words/;
print "quote-03 counts: ", join(",", map { scalar @$_ } (\@a,\@b,\@c,\@d,\@e,\@f,\@g)), "\n";
print "quote-03 a: @a\n";

# Multi-line qw and one with a `$` in it (not interpolated).
my @m = qw(
    first
    second   third
);
print "quote-03 multiline: ", scalar(@m), " [@m]\n";
my @dollar = qw($notavar @notanarray);
print "quote-03 literal-sigils: @dollar\n";
