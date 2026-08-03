#!/usr/bin/perl
# Accumulators. Perl's arithmetic operators read numbers out of whatever
# they are given, so a total built out of text fields is still a number, and
# the type a Go variable ends up with has to say so.
use strict;
use warnings;

my @rows = (
    "disk0 512 1.5",
    "disk1 1024 2.25",
    "disk2 256 0.5",
);

# ---- a total built from text fields ------------------------------------
my $bytes = 0;
my $load  = 0;
my $count = 0;
for my $row (@rows) {
    my ( $name, $size, $factor ) = split ' ', $row;
    $bytes += $size;      # text on the right, a number on the left
    $load  += $factor;    # and this one really does carry a fraction
    $count++;
}
printf "rows=%d bytes=%d load=%.2f\n", $count, $bytes, $load;

# ---- an average, which division makes fractional either way ------------
my $avg = $bytes;
$avg /= $count;
printf "avg=%.2f\n", $avg;

# ---- the other compound operators, each with its own result type -------
my $scaled = $bytes;
$scaled *= 2;
my $trimmed = $bytes;
$trimmed -= 256;
my $wrapped = $bytes;
$wrapped %= 1000;
my $squared = 3;
$squared **= 4;
print "scaled=$scaled trimmed=$trimmed wrapped=$wrapped squared=$squared\n";

# ---- text accumulation stays text --------------------------------------
my $report = '';
$report .= "$_;" for map { ( split ' ', $_ )[0] } @rows;
print "report=$report\n";

# ---- a counter fed from a loop variable of no obvious type -------------
my %seen;
my $hits = 0;
for my $word (qw(alpha beta alpha gamma alpha)) {
    $seen{$word}++;
    $hits += 1;
}
print "hits=$hits distinct=", scalar( keys %seen ), "\n";
print join( ' ', map { "$_=$seen{$_}" } sort keys %seen ), "\n";
