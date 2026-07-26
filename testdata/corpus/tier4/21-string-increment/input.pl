#!/usr/bin/perl
# TRAP: ++ on a non-numeric string does alphabetic carrying ("az" -> "ba",
# "Zz" -> "AAa"), and ".." ranges use the same magic. No Go equivalent.
use strict;
use warnings;

my $id = "aa";
$id++ for 1 .. 3;
print "id=$id\n";                     # ad

for my $start (qw(az zz Az Zz a9 z9)) {
    my $v = $start;
    $v++;
    print "$start -> $v\n";
}

my @codes = ( "aa" .. "ac" );         # magic ranges too
print "codes=@codes\n";

my $v = "a9";
$v .= "";                             # concatenation keeps it a string...
$v++;                                 # ...so ++ is STILL magical
print "after concat: $v\n";
