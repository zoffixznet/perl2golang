#!/usr/bin/perl
# Perl flattens every array in an argument list into one @_, so a sub cannot
# tell `f($x, @xs)` from `f($x, $xs[0], $xs[1])`. Go spreads exactly one
# slice and nothing else, so the flattening has to be written out.
use strict;
use warnings;

sub total {
    my $sum = 0;
    $sum += $_ for @_;
    return $sum;
}

sub labelled {
    my ( $label, @values ) = @_;
    return sprintf '%s[%d]: %s', $label, scalar @values, join( ',', @values );
}

my @batch  = ( 10, 20, 30 );
my @extra  = ( 40, 50 );
my $single = 5;

# one array, spread whole
print "a: ", total(@batch), "\n";

# single values only
print "b: ", total( 1, 2, 3 ), "\n";

# a single value in front of an array, which is the shape that needs a list
print "c: ", total( $single, @batch ), "\n";

# two arrays, one after the other
print "d: ", total( @batch, @extra ), "\n";

# arrays and single values interleaved
print "e: ", total( 1, @batch, 2, @extra, 3 ), "\n";

# a fixed parameter in front of a variadic tail
print "f: ", labelled( 'batch', @batch ), "\n";
print "g: ", labelled( 'mixed', $single, @batch, @extra ), "\n";

# an empty array contributes nothing at all
my @nothing;
print "h: ", total( @nothing, 7 ), "\n";
print "i: ", labelled( 'empty', @nothing ), "\n";
