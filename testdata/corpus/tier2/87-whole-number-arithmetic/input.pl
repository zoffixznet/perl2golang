#!/usr/bin/perl
use strict;
use warnings;

# Dividing bytes into pages, which is the ordinary reason a script reaches for
# `use integer`, next to the same sums written without it.

my @sizes = ( 8192, 5000, 1, 0, 40960 );
my $page  = 4096;

print "--- floating point, which is what Perl does by default ---\n";
for my $n (@sizes) {
    printf "%6d / %d = %s\n", $n, $page, $n / $page;
}

print "--- whole numbers, inside the pragma ---\n";
{
    use integer;
    for my $n (@sizes) {
        my $full = $n / $page;
        my $rest = $n % $page;
        printf "%6d = %d full page(s) + %d byte(s)\n", $n, $full, $rest;
    }
    # Rounding up, which only reads correctly with truncating division.
    for my $n (@sizes) {
        printf "%6d needs %d page(s)\n", $n, ( $n + $page - 1 ) / $page;
    }
}

print "--- and the pragma is over ---\n";
printf "5000 / %d is %s again\n", $page, 5000 / $page;

print "--- the sign rule, which the pragma also changes ---\n";
for my $pair ( [ -7, 3 ], [ 7, -3 ], [ -7, -3 ] ) {
    my ( $n, $d ) = @$pair;
    my $perl = $n % $d;
    my $c;
    {
        use integer;
        $c = $n % $d;
    }
    printf "%3d %% %-3d  perl=%3d  integer=%3d\n", $n, $d, $perl, $c;
}

print "--- int() truncates towards zero either way ---\n";
printf "int(-7/2) = %d, int(7/2) = %d\n", int( -7 / 2 ), int( 7 / 2 );
