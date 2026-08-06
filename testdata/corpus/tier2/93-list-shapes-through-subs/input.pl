#!/usr/bin/perl
# Inventory reorder report: every value here changes shape on its way through
# a sub boundary or a list. Costs and names travel as one flat list, a sub
# hands back a scalar and an array in one return, eval takes that list
# through a failure path, and the memoised depth check ends in //=.
use strict;
use warnings;

my %stock = (
    bolts   => { have => 40,  min => 100, per_crate => 50 },
    washers => { have => 900, min => 200, per_crate => 500 },
    nuts    => { have => 60,  min => 150, per_crate => 25 },
);
my %depends = (
    bolts => [ 'washers', 'nuts' ],
    nuts  => [ 'washers' ],
);

# Returns a cost plus the crates to order: one fixed value, then a tail
# whose length depends on the shortage.
sub order_plan {
    my ($item) = @_;
    my $s = $stock{$item};
    die "no such item: $item\n" unless $s;
    my $short = $s->{min} - $s->{have};
    return () if $short <= 0;
    my @crates;
    push @crates, "crate-of-$s->{per_crate}" while $s->{per_crate} * @crates < $short;
    return ( scalar(@crates) * 7, @crates );
}

for my $item ( sort keys %stock ) {
    my ( $cost, @crates ) = eval { order_plan($item) };
    if ($@)             { print "$item: error\n" }
    elsif ( !@crates )  { print "$item: stocked\n" }
    else                { printf "%s: %d crates, cost %d\n", $item, scalar(@crates), $cost }
}
my ( $cost, @crates ) = eval { order_plan('girders') };
printf "girders: %s\n", $@ ? 'unknown item' : 'ok';

# A memoised recursive depth whose sub body ends in //=.
my %memo;
sub depth {
    my ($item) = @_;
    my @below = @{ $depends{$item} // [] };
    my $deepest = 0;
    for my $d (@below) {
        my $got = depth($d);
        $deepest = $got if $got > $deepest;
    }
    $memo{$item} //= 1 + $deepest;
}
printf "%s depth %d\n", $_, depth($_) for sort keys %stock;

# One flat list built from mixed pieces: a scalar, an array, a slice.
my @head = ( 'bolts', 'nuts' );
my @all  = ( 'start', @head, ( sort keys %depends )[ 0, 1 ], 'end' );
printf "trail: %s (%d stops)\n", join( '>', @all ), scalar @all;

# A parenthesised ternary in a value position is grouping, not a list.
my @row = ( 3, 9, 4 );
my $j = 2;
$row[$j] = $j > 0 ? ( $row[$j] >= $row[ $j - 1 ] ? $row[$j] : $row[ $j - 1 ] ) : 0;
print "settled: @row\n";
