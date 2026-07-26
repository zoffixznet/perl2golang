#!/usr/bin/perl
# CASE quote-11: `qr` builds a Regexp object. It takes one part plus modifiers,
# interpolates, and its stringification embeds the modifiers -- which matters when
# a qr// is interpolated into another pattern.
use strict; use warnings;

my $word = "wor";
my $re = qr/${word}ld/i;
print "quote-11 stringified: $re\n";
print "quote-11 match: ", ("Hello WORLD" =~ $re ? "yes" : "no"), "\n";

# Interpolating a qr into a bigger pattern.
my $big = qr/^Hello\s+$re$/;
print "quote-11 nested: ", ("Hello WORLD" =~ $big ? "yes" : "no"), "\n";
print "quote-11 nested-str: $big\n";

# qr with bracketing delimiters and with modifiers.
my $b = qr{ \d+ }x;
print "quote-11 brace-x: ", ("abc 123" =~ $b ? "yes" : "no"), " [$b]\n";

# qr'...' does NOT interpolate.
my $lit = qr'$word';
print "quote-11 single-quoted: [$lit] matches-literal-dollar: ",
      ('$word' =~ $lit ? "yes" : "no"), "\n";
