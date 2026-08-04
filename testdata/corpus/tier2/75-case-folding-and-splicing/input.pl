#!/usr/bin/perl
use strict;
use warnings;

# Two things a Go replacement template cannot say, and one call that edits
# the string it was handed rather than answering with a copy.

print "--- case markers in a string ---\n";
my $who = 'ada lovelace';
print "greeting:  \uhello, \u$who\n";
print "shouted:   \U$who\E, quietly again: \L\uADA\E\n";

print "--- case markers in a replacement ---\n";
my $heading = 'the ANALYTICAL engine';
( my $title = $heading ) =~ s/(\w+)/\u\L$1/g;
print "title:     $title\n";
my $first = $heading;
$first =~ s/(\w+)/\U$1/;
print "first only: $first\n";
my $tagged = 'one two';
$tagged =~ s/(\w+)/[\U$&\E]/g;
print "tagged:    $tagged\n";

print "--- a replacement that reads a table ---\n";
my %field = ( NAME => 'ada', ROLE => 'engineer' );
my $template = 'NAME is a ROLE';
( my $filled = $template ) =~ s/(NAME|ROLE)/$field{$1}/g;
print "filled:    $filled\n";

print "--- a pattern built from data ---\n";
my $literal = 'a.c';
my $haystack = 'abc a.c axc';
my @exact = ( $haystack =~ /\Q$literal\E/g );
my @loose = ( $haystack =~ /$literal/g );
printf "quoted matches: %d, unquoted: %d\n", scalar @exact, scalar @loose;

print "--- substr with four arguments edits in place ---\n";
my $sentence = 'The quick brown fox';
my $removed = substr( $sentence, 4, 5, 'lazy' );
print "removed '$removed' -> $sentence\n";
substr( $sentence, 0, 0, '>> ' );
substr( $sentence, length $sentence, 0, ' <<' );
print "framed:    $sentence\n";
