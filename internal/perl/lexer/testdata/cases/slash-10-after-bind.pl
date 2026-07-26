#!/usr/bin/perl
# CASE slash-10: `/` after `=~` or `!~` is always a MATCH. The bind operators are
# the one unambiguous signal, and they also make the following `/.../` a pattern
# even where a bare `/` would have been division.
use strict; use warnings;

my $s = "10/4";
print "slash-10 bind: ", ($s =~ /\d+/ ? "match" : "no"), "\n";
print "slash-10 negbind: ", ($s !~ /zzz/ ? "nomatch-ok" : "bad"), "\n";

# Substitution and transliteration bound the same way.
(my $t = $s) =~ s/\//-/;
print "slash-10 subst: $t\n";

my $n = ($s =~ tr{0-9}{});
print "slash-10 tr-count: $n\n";

# `=~` with a variable holding a qr// -- no slash at all.
my $re = qr/\d\/\d/;
print "slash-10 qr: ", ($s =~ $re ? "match" : "no"), "\n";
