#!/usr/bin/perl
use strict;
use warnings;

# A sub yields whatever it evaluated last, and that value is often a call:
# the one-line constructor, the one-line accessor, and the sub that hands
# back what a helper worked out.

package Counter;

sub new { bless { n => 0, step => 1 }, shift }

sub bump {
    my ($self) = @_;
    $self->{n} += $self->{step};
    return $self->{n};
}

sub total { $_[0]{n} }

package main;

my $c = Counter->new;
$c->bump for 1 .. 3;
printf "counter reached %d\n", $c->total;

# A sub whose last statement is a call returns that call's value.
sub double { my ($x) = @_; return $x * 2 }
sub quadruple { double( double( $_[0] ) ) }
printf "quadruple(5) = %d\n", quadruple(5);

# A sub that returns nothing at all: the scalar it lands in is undef.
sub announce { print "announce: @_\n"; return }
my $nothing = announce('ready');
printf "announce gave back %s\n", ( defined $nothing ? 'a value' : 'undef' );

# map building a hash, where the value needs a lookup of its own per element.
my %stock = ( apple => [ 'crate', 'crate' ], fig => ['box'] );
my @fruit = qw(apple fig plum);
my %packed = map { $_ => scalar @{ $stock{$_} // [] } } @fruit;
for my $f ( sort keys %packed ) {
    printf "  %-6s %d container(s)\n", $f, $packed{$f};
}
