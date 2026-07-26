#!/usr/bin/perl
use strict;
use warnings;

# A filter in the grep/awk mould: reads STDIN, keeps the lines that match,
# renumbers them and prints a summary at the end. Options come from @ARGV.

my $pattern = shift @ARGV;
$pattern = '.' unless defined $pattern;
my $invert  = (shift(@ARGV) || '') eq '-v';

my $re = eval { qr/$pattern/ };
die "$0: bad pattern '$pattern'\n" unless $re;

my $seen    = 0;
my $kept    = 0;
my $bytes   = 0;
my @matched;

while (my $line = <STDIN>) {
    $seen++;
    $bytes += length $line;
    chomp $line;

    my $hit = $line =~ $re ? 1 : 0;
    next if $invert ? $hit : !$hit;

    $kept++;
    push @matched, [ $seen, $line ];
}

for my $rec (@matched) {
    printf "%4d: %s\n", $rec->[0], $rec->[1];
}

printf "-- %d/%d lines kept (%d bytes read), pattern=/%s/%s\n",
    $kept, $seen, $bytes, $pattern, ($invert ? ' inverted' : '');

# Summarise the surviving lines: longest, shortest, field counts.
if (@matched) {
    my @lens = sort { $a <=> $b } map { length $_->[1] } @matched;
    printf "-- shortest=%d longest=%d\n", $lens[0], $lens[-1];

    my %fieldcount;
    for my $rec (@matched) {
        my @f = split ' ', $rec->[1];
        $fieldcount{ scalar @f }++;
    }
    printf "-- %d line(s) with %d field(s)\n", $fieldcount{$_}, $_
        for sort { $a <=> $b } keys %fieldcount;
}
else {
    print "-- nothing matched\n";
}
