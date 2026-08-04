#!/usr/bin/perl
use strict;
use warnings;

# The same table, one level of indirection further away: the grid is an array
# reference held in a scalar, which is how it is written the moment it has to
# be passed to a sub or kept in a structure.

my $grid = [];
my ( $rows, $cols ) = ( 3, 4 );

for my $r ( 0 .. $rows - 1 ) {
    for my $c ( 0 .. $cols - 1 ) {
        $grid->[$r][$c] = ( $r + 1 ) * ( $c + 1 );
    }
}

sub row_total {
    my ( $g, $r ) = @_;
    my $sum = 0;
    $sum += $_ for @{ $g->[$r] };
    return $sum;
}

sub widen {
    my ( $g, $r, $c, $v ) = @_;
    $g->[$r][$c] = $v;    # may be past the end of either level
    return;
}

print "--- the grid ---\n";
for my $r ( 0 .. $#{$grid} ) {
    printf "  %s\n", join ' ', map { sprintf '%2d', $_ } @{ $grid->[$r] };
}
printf "row totals: %s\n", join ',', map { row_total( $grid, $_ ) } 0 .. $rows - 1;

widen( $grid, 4, 5, 99 );
printf "after widening: %d rows, last row %d wide\n",
  scalar @$grid, scalar @{ $grid->[-1] };
printf "the corner holds %d\n", $grid->[4][5];
printf "a cell the widening skipped is %s\n",
  ( defined $grid->[4][0] ? 'set' : 'undef' );
