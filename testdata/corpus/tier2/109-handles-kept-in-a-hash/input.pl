#!/usr/bin/perl
use strict;
use warnings;

# A script that writes several files at once keeps the handles in a hash and
# opens straight into the slot. There is no variable to declare, which is the
# whole point: the handle belongs to the name it is filed under.

my @streams = qw(access error debug);
my %out;

for my $name (@streams) {
    open( $out{$name}, '>', "$name.log" ) or die "open $name.log: $!\n";
}

my @events = (
    [ 'access', 'GET /index.html 200' ],
    [ 'error',  'permission denied' ],
    [ 'access', 'POST /login 302' ],
    [ 'debug',  'cache miss' ],
    [ 'error',  'timeout after 30s' ],
);

my %count;
for my $event (@events) {
    my ( $stream, $text ) = @$event;
    print { $out{$stream} } "$text\n";
    $count{$stream}++;
}

close $out{$_} or die "close $_: $!\n" for sort keys %out;

for my $name (@streams) {
    printf "%-7s %d line(s), %d bytes\n", $name, $count{$name}, -s "$name.log";
}

# The handles are gone, and the files they wrote are not.
my $total = 0;
for my $name (@streams) {
    open my $in, '<', "$name.log" or die "reopen $name.log: $!\n";
    my @lines = <$in>;
    close $in;
    $total += @lines;
    unlink "$name.log";
}
print "read back $total line(s)\n";
