#!/usr/bin/perl
# A journal file with a fixed header, a line-oriented body, and a fixed-width
# footer: the layout a reader has to move around in rather than stream
# through. The footer is read first, from the end, then the body from where
# the header stopped.
use strict;
use warnings;

open my $fh, '<', 'files/journal.dat' or die "open: $!\n";

read $fh, my $magic, 4;
die "not a journal: '$magic'\n" unless $magic eq 'JRNL';
read $fh, my $nl, 1;

my $body_start = tell $fh;
printf "body starts at byte %d\n", $body_start;

# The footer is the last 8 bytes: a 4-byte tag and a zero-padded count.
seek $fh, -8, 2 or die "seek to footer: $!\n";
read $fh, my $footer, 8;
my ( $ftag, $declared ) = $footer =~ /^([A-Z]+)0*(\d+)$/;
die "bad footer '$footer'\n" unless defined $ftag && $ftag eq 'FOOT';
printf "footer declares %d line(s)\n", $declared;

# Back to the body for the real count.
seek $fh, $body_start, 0 or die "seek to body: $!\n";
my ( $lines, $started ) = ( 0, 0 );
while ( my $line = <$fh> ) {
    chomp $line;
    last if $line =~ /^FOOT/;
    $lines++;
    $started++ if $line =~ /started$/;
}
close $fh;

printf "body has %d line(s), %d started\n", $lines, $started;
printf "count %s\n", $lines == $declared ? 'agrees' : 'DISAGREES';
