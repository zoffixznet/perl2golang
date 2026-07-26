#!/usr/bin/perl
# CASE quote-08: modifiers follow the FINAL delimiter and are a run of word
# characters. The lexer must consume them greedily and must not confuse `x` (the
# /x modifier) with the repetition operator, nor `e` with a bareword.
use strict; use warnings;

my $s = "Hello World";
print "quote-08 i: ",  ($s =~ /hello/i  ? "match" : "no"), "\n";
print "quote-08 xms: ", ($s =~ /
        hello \s+ world   # free-spacing comment
    /xi ? "match" : "no"), "\n";

my @all = ("aXbXc" =~ /(\w)X/g);
print "quote-08 g-list: @all\n";

(my $e = "2+3") =~ s/(\d+)\+(\d+)/$1+$2/e;
print "quote-08 e: $e\n";

my $x = 5;
(my $ee = "1+1") =~ s/(.+)/'$x * 2'/ee;    # first /e yields the STRING '$x * 2',
print "quote-08 ee: $ee\n";                # second /e evals that string as Perl

my $r    = "aaa";
my $rmod = $r =~ s/a/b/gr;                 # /r leaves $r untouched
print "quote-08 r-modifier: orig=$r new=$rmod\n";

my $qr = qr/world/i;
print "quote-08 qr-with-mods: ", ($s =~ $qr ? "match" : "no"), " stringified=$qr\n";

# The whole modifier run right before a `,` and before `;`.
my @m = grep { /o/i } ("Foo","bar");
print "quote-08 mods-then-brace: @m\n";
