#!/usr/bin/perl
# CASE quote-10: `tr` and `y` are the same operator. Their two parts are
# CHARACTER LISTS, not patterns: no interpolation of variables, `-` means a
# range, and the modifiers are cdsr.
use strict; use warnings;

my $s = "hello world";

(my $a = $s) =~ tr/a-z/A-Z/;
print "quote-10 upper: $a\n";

my $count = ($s =~ tr/o//);
print "quote-10 count-o: $count\n";

(my $d = $s) =~ tr/lo//d;
print "quote-10 delete: $d\n";

(my $c = "aaabbb") =~ tr/ab//s;
print "quote-10 squeeze: $c\n";

(my $comp = $s) =~ tr/a-z/./c;   # complement: non a-z becomes .
print "quote-10 complement: $comp\n";

my $ret = $s =~ tr/lo/LO/r;      # /r returns a copy
print "quote-10 r: $ret original: $s\n";

# y is a synonym; a `$` inside is a literal dollar sign, NOT interpolation.
my $var = "ZZZ";
(my $y = 'a$b') =~ y/$var/#/;    # translates any of $ v a r to #
print "quote-10 y-no-interp: $y\n";
