#!/usr/bin/perl
# A ledger import that reads its input four different ways through one
# handle: a header line, a line-at-a-time loop, a continuation read in the
# middle of the loop, and a slurp of whatever is left after the marker.
use strict;
use warnings;

my $header = <STDIN>;
chomp $header;
my @cols = split /,/, $header;
printf "columns: %d (%s)\n", scalar @cols, join( '|', @cols );

my $rows  = 0;
my $total = 0;
while ( my $line = <STDIN> ) {
    chomp $line;
    last if $line eq '__NOTES__';
    # A row ending in a backslash continues on the next physical line.
    while ( $line =~ /\\$/ and defined( my $more = <STDIN> ) ) {
        chomp $more;
        $line =~ s/\\$//;
        $line .= $more;
    }
    my @f = split /,/, $line;
    $rows++;
    $total += $f[2];
    printf "  %-8s %-14s %5d\n", $f[0], $f[1], $f[2];
}
printf "%d row(s), total %d\n", $rows, $total;

my $notes = do { local $/; <STDIN> };
my $lines = () = $notes =~ /\S+.*$/mg;
printf "notes: %d line(s), %d byte(s)\n", $lines, length $notes;
