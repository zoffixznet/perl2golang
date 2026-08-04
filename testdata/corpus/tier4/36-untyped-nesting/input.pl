#!/usr/bin/perl
use strict;
use warnings;

# A structure whose shape changes as it is walked: every value under a key is
# a hash, a list, or a plain scalar depending on the key, and which it is
# cannot be worked out without running the program. Nothing here is
# convertible into a Go type, so every read of it is a guess.

my %doc = (
    title   => 'Quarterly report',
    authors => [ 'ada', 'grace' ],
    meta    => { year => 2024, quarter => 3 },
    tables  => [
        { name => 'sales',  rows => [ [ 'north', 12 ], [ 'south', 7 ] ] },
        { name => 'errors', rows => [] },
    ],
);

sub describe {
    my ( $key, $value ) = @_;
    my $kind = ref $value;
    return "$key: scalar '$value'"                       if $kind eq '';
    return "$key: list of " . scalar( @$value ) . ' item(s)' if $kind eq 'ARRAY';
    return "$key: record with " . scalar( keys %$value ) . ' field(s)';
}

for my $key ( sort keys %doc ) {
    print describe( $key, $doc{$key} ), "\n";
}

print "authors: ", join( ', ', @{ $doc{authors} } ), "\n";
print "year: $doc{meta}{year} quarter: $doc{meta}{quarter}\n";

for my $table ( @{ $doc{tables} } ) {
    my @rows = @{ $table->{rows} };
    printf "table %-7s %d row(s)", $table->{name}, scalar @rows;
    print @rows ? ' first=' . join( '/', @{ $rows[0] } ) : ' (empty)';
    print "\n";
}

print "missing key is ", ( defined $doc{nope} ? 'there' : 'undef' ), "\n";
