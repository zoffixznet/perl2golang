#!/usr/bin/perl
# Reading @_ without naming it: the argument list sliced, counted, indexed
# past its end, and carried out behind a reference that is only ever read.
# None of these writes through to the caller, so all of them convert.
use strict;
use warnings;

sub label {
    my ( $first, $last ) = @_[ 0, 1 ];    # a slice of @_
    return "$last, $first";
}

sub arity {
    return scalar @_;
}

sub third_or_default {
    return $_[2] // 'none';               # read past the end is undef
}

sub summarise {
    my $args = \@_;                       # a reference, only read
    my $total = 0;
    $total += $_ for @$args;
    return sprintf '%d item(s), total %d', scalar @$args, $total;
}

print label( 'Grace', 'Hopper' ), "\n";
printf "arity: %d and %d\n", arity( 1, 2, 3 ), arity();
printf "third: %s / %s\n", third_or_default( 'a', 'b', 'c' ), third_or_default('a');
print summarise( 3, 4, 5 ), "\n";
