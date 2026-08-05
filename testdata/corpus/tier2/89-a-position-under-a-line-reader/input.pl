#!/usr/bin/perl
# The two corners of handle positioning that do not convert yet: tell asked
# in the middle of a line-reading loop, where a buffered reader has walked
# ahead of the lines handed out, and the four-argument read, which lands its
# bytes at an offset inside the target instead of replacing it.
use strict;
use warnings;

open my $fh, '<', 'files/marks.txt' or die "open: $!\n";

# tell between line reads has to report the byte after the line just
# returned, not wherever the buffer's read-ahead happens to be.
my @stops;
while ( my $line = <$fh> ) {
    chomp $line;
    push @stops, sprintf '%s@%d', ( split /,/, $line )[0], tell $fh;
    last if @stops >= 3;
}
print "stops: @stops\n";

# The offset form patches bytes into the middle of a buffer it grew first.
seek $fh, 0, 0 or die "seek: $!\n";
my $patched = 'XXXXXXXXXX';
read $fh, $patched, 5, 3;
print "patched: $patched\n";
close $fh;
