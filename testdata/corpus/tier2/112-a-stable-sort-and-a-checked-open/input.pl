#!/usr/bin/perl
use strict;
use warnings;
no warnings qw(unopened closed);

# Two habits that ordinary scripts have and never state. Sorting on one field
# of a record and expecting the records that tie to stay in the order they
# arrived, and taking the result of an open as a value rather than dying on
# it, then reporting what went wrong out of $!.

my @rows = (
    { host => 'web-3', zone => 'eu', load => 4 },
    { host => 'db-1',  zone => 'us', load => 9 },
    { host => 'web-1', zone => 'eu', load => 4 },
    { host => 'cache', zone => 'ap', load => 1 },
    { host => 'web-2', zone => 'eu', load => 4 },
    { host => 'db-2',  zone => 'us', load => 9 },
);

# Sorted on load alone. Every tie keeps the order above, which is the whole
# reason this is readable output rather than a lottery.
my @by_load = sort { $a->{load} <=> $b->{load} } @rows;
print "by load:  ", join( ' ', map { $_->{host} } @by_load ), "\n";

# Descending, ties still in arrival order.
my @by_desc = sort { $b->{load} <=> $a->{load} } @rows;
print "by load desc: ", join( ' ', map { $_->{host} } @by_desc ), "\n";

# Sorted on the zone, which three rows share.
my @by_zone = sort { $a->{zone} cmp $b->{zone} } @rows;
print "by zone:  ", join( ' ', map { $_->{host} } @by_zone ), "\n";

# An open whose answer is a value. The script decides what to do about it.
my $missing = 'no-such-directory/report.txt';
my $opened  = open( my $fh, '<', $missing );
if ($opened) {
    print "opened the report\n";
    close $fh;
}
else {
    print "could not open the report: $!\n";
}

# Reading a handle that was never opened gives nothing, and the script runs on.
my @lines = <$fh>;
printf "read %d line(s) from it anyway\n", scalar @lines;
print "still here, exiting 0\n";
