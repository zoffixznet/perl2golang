#!/usr/bin/perl
use strict;
use warnings;

# Absence that travels: a lookup sub hands undef back to its caller, and a
# list of records has a hole in it. Both cross a boundary where the type is
# decided once and for all.

my %price = ( bolt => 0, nut => 3 );
$price{washer} = undef;

sub lookup {
    my ($item) = @_;
    return $price{$item};
}

print "--- a sub that may answer with nothing ---\n";
for my $item (qw(bolt nut washer screw)) {
    my $p = lookup($item);
    printf "%-7s %s\n", $item, ( defined $p ? "costs $p" : 'no price' );
}

print "--- absence inside a list of records ---\n";
my @rows = (
    { name => 'bolt',   qty => 4 },
    { name => 'washer', qty => undef },
    { name => 'nut',    qty => 0 },
);
for my $row (@rows) {
    printf "%-7s %s\n", $row->{name},
        ( defined $row->{qty} ? "qty $row->{qty}" : 'qty unknown' );
}

print "--- the first defined one wins ---\n";
my @candidates = ( undef, 0, 7 );
my $first;
for my $c (@candidates) {
    next unless defined $c;
    $first = $c;
    last;
}
printf "first defined candidate: %s\n", ( defined $first ? $first : 'none' );
