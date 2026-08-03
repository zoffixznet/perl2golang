#!/usr/bin/perl
# tr/// and its four modifiers, which change what a character-for-character
# replacement means rather than how it is written. The neighbouring case is at
# the bottom: a search list built from a variable, which tr cannot take at all
# and which is the reason s/// exists.
use strict;
use warnings;

my $dna = 'ACGTNNACGTXX';

print "-- plain replacement --\n";
( my $complement = $dna ) =~ tr/ACGT/TGCA/;
printf "complement: %s\n", $complement;

print "-- counting --\n";
my $bases = ( $dna =~ tr/ACGT// );
my $other = ( $dna =~ tr/ACGT//c );
printf "in ACGT: %d, outside: %d\n", $bases, $other;

print "-- deleting --\n";
( my $clean = $dna ) =~ tr/ACGT//cd;
printf "only bases: %s\n", $clean;

print "-- squeezing --\n";
my $spaced = "a   b\t\tc  d";
( my $tidy = $spaced ) =~ tr/ \t/ /s;
printf "tidied: [%s]\n", $tidy;

print "-- returning instead of changing --\n";
my $upper = ( $dna =~ tr/acgt/ACGT/r );
printf "unchanged: %s, returned: %s\n", $dna, $upper;

print "-- complement with a replacement --\n";
( my $masked = $dna ) =~ tr/ACGT/./cs;
printf "masked: %s\n", $masked;

print "-- ranges --\n";
( my $rot = 'Hello, World' ) =~ tr/A-Za-z/N-ZA-Mn-za-m/;
printf "rot13: %s\n", $rot;
( my $stripped = 'a1b2c3' ) =~ tr/0-9//d;
printf "digits gone: %s\n", $stripped;
