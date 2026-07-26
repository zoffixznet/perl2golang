#!/usr/bin/perl
# TRAP: string-to-number coercion. Leading junk parses as far as it can,
# "0x10" is 0 (not 16), "010" is 10 (not 8), whitespace is tolerated,
# "" is 0. Go's strconv.Atoi would ERROR on most of these.
use strict;
use warnings;
no warnings 'numeric';

for my $s ( "3abc", "0x10", "010", " 12 ", "1e3", ".5", "+7", "1_000", "inf", "nan", "" ) {
    printf "%-8s -> %s\n", "'$s'", $s + 0;
}

print "hex() does hex:     ", hex("0x10"), "\n";
print "oct() does all:     ", oct("0x10"), " ", oct("010"), " ", oct("0b101"), "\n";
