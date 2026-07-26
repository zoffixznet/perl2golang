#!/usr/bin/perl
# Fixed-width bank-ledger reader/writer built on pack/unpack templates.
# Layout:
#   HDR a3 date A8 branch A10
#   TXN a3 seq A6 type A4 desc A20 cents A10   (type DEP=+, WDR/FEE=-)
#   TRL a3 count A6 net-cents A10
use strict;
use warnings;
use POSIX qw(floor);

my $file = shift @ARGV // 'files/ledger.dat';

my %SIGN = ( DEP => 1, WDR => -1, FEE => -1 );

open my $fh, '<', $file or die "open $file: $!\n";
my @lines = <$fh>;
close $fh;
chomp @lines;

# ---- header ------------------------------------------------------------
my ( $htag, $date, $branch ) = unpack 'a3 A8 A10', $lines[0];
die "bad header tag '$htag'\n" unless $htag eq 'HDR';
my ( $y, $m, $d ) = unpack 'A4 A2 A2', $date;
printf "ledger for %s on %04d-%02d-%02d\n", $branch, $y, $m, $d;

# ---- transactions ------------------------------------------------------
my ( @txns, $net, %by_type );
$net = 0;
for my $line ( @lines[ 1 .. $#lines - 1 ] ) {    # array slice, skip HDR/TRL
    my ( $tag, $seq, $type, $desc, $cents ) = unpack 'a3 A6 A4 A20 A10', $line;
    die "unexpected record '$tag'\n" unless $tag eq 'TXN';
    my $sign = $SIGN{$type} // die "unknown txn type '$type'\n";
    my $amount = $sign * $cents;                  # numify zero-padded field
    push @txns,
      { seq => $seq + 0, type => $type, desc => $desc, cents => $amount };
    $net += $amount;
    $by_type{$type}{count}++;
    $by_type{$type}{cents} += $cents + 0;
}

# ---- trailer validation ------------------------------------------------
my ( $ttag, $tcount, $tnet ) = unpack 'a3 A6 A10', $lines[-1];
die "bad trailer\n" unless $ttag eq 'TRL';
printf "trailer count: %s (%s)\n", $tcount + 0,
  $tcount == @txns ? 'ok' : 'MISMATCH';
printf "trailer net:   %.2f (%s)\n", $tnet / 100,
  $tnet == $net ? 'ok' : 'MISMATCH';

# ---- report ------------------------------------------------------------
sub money { sprintf '%9.2f', $_[0] / 100 }

print "-" x 46, "\n";
for my $t (@txns) {
    printf "%3d %-4s %-20s %s\n", $t->{seq}, $t->{type}, $t->{desc},
      money( $t->{cents} );
}
print "-" x 46, "\n";
for my $type ( sort keys %by_type ) {
    printf "%-4s x%d %s\n", $type, $by_type{$type}{count},
      money( $by_type{$type}{cents} );
}
printf "net movement: %s\n", money($net);

# Integer cents avoid float drift; derive a stress metric with floor().
my $avg_cents = floor( abs($net) / scalar(@txns) );
printf "avg per txn:  %s (floor'd)\n", money($avg_cents);

# ---- re-emit: filter + repack round trip -------------------------------
my @deposits = grep { $_->{type} eq 'DEP' } @txns;
my $out      = pack( 'a3 A8 A10', 'HDR', $date, $branch ) . "\n";
my $sum      = 0;
for my $t (@deposits) {
    $out .= pack( 'a3 a6 A4 A20 a10',
        'TXN', sprintf( '%06d', $t->{seq} ),
        $t->{type}, $t->{desc}, sprintf( '%010d', $t->{cents} ) )
      . "\n";
    $sum += $t->{cents};
}
$out .= pack( 'a3 a6 a10', 'TRL', sprintf( '%06d', scalar @deposits ),
    sprintf( '%010d', $sum ) )
  . "\n";
print "--- deposits-only file ---\n$out";
printf "emitted %d bytes\n", length $out;
