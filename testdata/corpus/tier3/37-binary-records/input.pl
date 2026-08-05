#!/usr/bin/perl
# Encoder/decoder for a small binary event journal, the wire-format half of a
# metrics collector. Each record is a fixed header packed big-endian (tag,
# sequence, severity, payload length) followed by the payload bytes, and the
# journal ends with a trailer carrying the record count.
use strict;
use warnings;

my @events = (
    [ 3, 'disk /var 91% full' ],
    [ 1, 'rotated access.log' ],
    [ 2, 'slow query: 2.4s' ],
);

# ---- encode ------------------------------------------------------------
my $journal = '';
my $seq     = 100;
for my $e (@events) {
    my ( $sev, $msg ) = @$e;
    $journal .= pack 'a2 n C N', 'EV', $seq, $sev, length $msg;
    $journal .= $msg;
    $seq++;
}
$journal .= pack 'a2 n', 'XX', scalar @events;
printf "journal is %d bytes\n", length $journal;

# a hex glance at the first header, the way a wireshark habit reads it
printf "first header: %s\n", unpack 'H18', $journal;

# ---- decode ------------------------------------------------------------
my %sev_name = ( 1 => 'info', 2 => 'warn', 3 => 'crit' );
my $pos      = 0;
my $worst    = 0;
my @decoded;
while (1) {
    my ( $tag, $n ) = unpack 'a2 n', substr( $journal, $pos, 4 );
    if ( $tag eq 'XX' ) {
        printf "trailer: %d record(s) declared, %d decoded\n", $n, scalar @decoded;
        last;
    }
    my ( $sev, $len ) = unpack 'C N', substr( $journal, $pos + 4, 5 );
    my $msg = substr( $journal, $pos + 9, $len );
    push @decoded, sprintf '#%d %-4s %s', $n, $sev_name{$sev}, $msg;
    $worst = $sev if $sev > $worst;
    $pos += 9 + $len;
}
print "$_\n" for @decoded;
printf "worst severity: %s\n", $sev_name{$worst};

# ---- little-endian for the on-disk index, negatives included ----------
my $index = pack 'v V l', 7, 3600, -1;
my ( $slot, $ttl, $sentinel ) = unpack 'v V l', $index;
printf "index: slot=%d ttl=%d sentinel=%d\n", $slot, $ttl, $sentinel;

# C* as a byte tour of a checksum-free digest
my $sum = 0;
$sum = ( $sum + $_ ) % 251 for unpack 'C*', 'EV';
printf "tag byte sum mod 251: %d\n", $sum;
