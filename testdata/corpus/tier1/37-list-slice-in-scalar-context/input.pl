#!/usr/bin/perl
use strict;
use warnings;

# A list slice picks one element out of a list without naming the list first.
# The whole expression is a single value, so it can be added to, compared and
# concatenated like any other scalar.

print "--- picking out of a literal list ---\n";
my $mid = (10, 20, 30)[1];
my $last = (10, 20, 30)[-1];
print "mid           = $mid\n";
print "mid + 5       = ", $mid + 5, "\n";
print "last          = $last\n";
print "mid * last    = ", $mid * $last, "\n";

print "--- picking a field out of a split ---\n";
my $line = "root:x:0:0:System Administrator:/root:/bin/sh";
my $uid = (split /:/, $line)[2];
my $shell = (split /:/, $line)[-1];
my $name = (split /:/, $line)[4];
print "uid           = $uid\n";
print "uid + 1000    = ", $uid + 1000, "\n";
print "shell         = $shell\n";
print "name is ", length($name), " characters\n";
print "label         = " . $name . " runs " . $shell . "\n";

print "--- picking out of a sorted list ---\n";
my @scores = (37, 12, 95, 60);
my $lowest = (sort { $a <=> $b } @scores)[0];
my $highest = (sort { $a <=> $b } @scores)[-1];
print "lowest        = $lowest\n";
print "highest       = $highest\n";
print "spread        = ", $highest - $lowest, "\n";

print "--- two at once is a list, not a scalar ---\n";
my @ends = (sort { $a <=> $b } @scores)[0, -1];
print "ends          = @ends\n";
print "ends count    = ", scalar(@ends), "\n";
