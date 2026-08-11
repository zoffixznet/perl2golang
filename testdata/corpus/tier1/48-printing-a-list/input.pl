#!/usr/bin/perl
use strict;
use warnings;

# Printing a list and interpolating one are different operations with
# different separators. print flattens its arguments into one list and puts
# $, between them, which is nothing at all by default. "@array" puts $"
# between the elements, which is a space. The two are one character apart in
# the source and the difference shows up in every line of the output.

my @words = qw(alpha beta gamma);

print @words;
print "\n";
print "@words\n";
print @words, ' and ', scalar(@words), "\n";

# The reason the default matters: lines that already end in a newline are
# printed straight out, and anything between them would be wrong.
my @lines = ( "first\n", "second\n" );
print @lines;
print "done\n";

# Numbers behave the same way, having become text on the way out.
my @nums = ( 1, 2, 3 );
print @nums;
print "\n";
print "@nums\n";

# An empty list contributes nothing at all.
my @empty = grep { length() > 99 } @words;
print '[', @empty, "]\n";

# $, is what the default was hiding: set it and it goes between the things
# each print writes, arrays flattened into the list included.
$, = '-';
print @words;
print "\n";
print 'x', 'y', "\n";
$, = '';
print @words;
print "\n";
