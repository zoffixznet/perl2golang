#!/usr/bin/perl
# CASE interp-05: `@array`, `@$ref`, `@{$ref}`, `@{[ ... ]}` all interpolate, and
# elements are joined with `$"` (default a single space). `@` NOT followed by a
# valid name is a literal `@` -- but under strict it can become a compile error.
use strict; use warnings;

my @a = (1,2,3);
my $r = \@a;

print "interp-05 array: [@a]\n";
print "interp-05 deref: [@$r]\n";
print "interp-05 braced: [@{$r}]\n";
print "interp-05 expr: [@{[ map { $_*10 } @a ]}]\n";
print "interp-05 slice: [@a[0,1]]\n";
{
    use feature 'postderef_qq';   # required for ->@* to interpolate
    print "interp-05 postfix: [$r->@*]\n";
}

{
    local $" = ", ";
    print "interp-05 custom-sep: [@a]\n";
}

# `@` followed by punctuation or a digit is literal.
print "interp-05 literal-at: [a\@b] [10@ 5] [\@]\n";

# An empty array interpolates to the empty string, not to "()".
my @e;
print "interp-05 empty: [@e] len=", length("@e"), "\n";

# Hash slices and key/value slices in strings.
my %h = (x=>1, y=>2);
print "interp-05 hash-slice: [@h{qw(x y)}]\n";
