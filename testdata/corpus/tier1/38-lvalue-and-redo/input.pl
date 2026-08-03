#!/usr/bin/perl
# The places Perl lets you assign that Go has no name for: through a
# conditional, through substr, through a slice, and the loop word that
# restarts an iteration without a Go counterpart at all.
use strict;
use warnings;

print "--- a conditional as an assignment target ---\n";
my ( $even, $odd ) = ( 0, 0 );
for my $n ( 1 .. 6 ) {
    ( $n % 2 ? $odd : $even ) += $n;
}
print "even=$even odd=$odd\n";

my ( $a, $b ) = ( 'a', 'b' );
( 1 ? $a : $b ) .= '!';
print "a=$a b=$b\n";

print "--- substr as an assignment target ---\n";
my $stamp = '0000-00-00 label';
substr( $stamp, 0, 4 )  = '2024';
substr( $stamp, 5, 2 )  = '07';
substr( $stamp, 8, 2 )  = '19';
substr( $stamp, -5 )    = 'ready';
print "$stamp\n";

my $short = 'abc';
substr( $short, 3, 0 ) = 'def';
print "appended: $short\n";

print "--- a slice as an assignment target ---\n";
my %conf;
@conf{qw(alpha beta gamma)} = ( 1, 2, 3 );
my @order = ('x') x 3;
@order[ 0, 2 ] = ( 'first', 'third' );
print join( ' ', map { "$_=$conf{$_}" } sort keys %conf ), "\n";
print "order: @order\n";

print "--- redo re-runs the body without re-reading the list ---\n";
my $tries = 0;
my @done;
for my $item ( 'a', 'b' ) {
    $tries++;
    if ( $item eq 'a' && $tries < 3 ) {
        redo;
    }
    push @done, "$item after $tries";
}
print "$_\n" for @done;
