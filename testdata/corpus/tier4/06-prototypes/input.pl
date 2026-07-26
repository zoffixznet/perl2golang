#!/usr/bin/perl
# TRAP: prototypes change how calls PARSE, not just how they behave:
# (&@) makes block syntax legal, (\@\@) takes refs to the caller's
# arrays, ($) forces scalar context on the argument.
use strict;
use warnings;

sub apply (&@) {
    my ( $fn, @items ) = @_;
    return map { $fn->($_) } @items;
}
my @doubled = apply { $_[0] * 2 } 1, 2, 3;    # bare block: looks like syntax
print "doubled=@doubled\n";

sub zipfirst (\@\@) {
    my ( $a, $b ) = @_;                       # receives TWO ARRAY REFS
    return "$a->[0]/$b->[0]";
}
my @x = ( 10, 20 );
my @y = ( 30, 40 );
print "zip=", zipfirst( @x, @y ), "\n";       # not four flattened elements

sub one ($) { return $_[0] }
my @stuff = ( 7, 8, 9 );
print "one=", one(@stuff), "\n";              # prints 3 (the count), not 7
