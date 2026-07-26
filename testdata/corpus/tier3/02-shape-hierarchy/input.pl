#!/usr/bin/perl
# Drives the Shape/Rectangle/Square/Circle hierarchy spread across .pm files.
use strict;
use warnings;
use lib '.';
use Shape;
use Rectangle;
use Square;
use Circle;
use List::Util qw(sum0);

my @shapes = (
    Rectangle->new( width => 4,   height => 3 ),
    Circle->new( radius => 2.5, name => 'wheel' ),
    Square->new( side => 5 ),
    Rectangle->new( width => 10, height => 0.5, name => 'plank' ),
);

print "--- catalogue ---\n";
print $_->describe, "\n" for @shapes;

# Polymorphic dispatch drives sorting.
my @by_area =
  sort { $a->area <=> $b->area or $a->serial <=> $b->serial } @shapes;
print "--- by area ---\n";
printf "%s (%s)\n", $_->name, ref $_ for @by_area;

printf "total area: %.3f\n", sum0( map { $_->area } @shapes );

# isa walks the whole chain, including two levels for Square.
my $sq = $shapes[2];
for my $class (qw(Square Rectangle Shape Circle)) {
    printf "square isa %-9s : %s\n", $class, $sq->isa($class) ? 'yes' : 'no';
}
printf "square can(is_square): %s\n", $sq->can('is_square') ? 'yes' : 'no';

# Abstract class enforcement.
for my $bad (
    sub { Shape->new( name => 'blob' ) },
    sub { Rectangle->new( width => -1, height => 2 ) },
  )
{
    if ( eval { $bad->(); 1 } ) { print "unexpectedly survived\n" }
    else                        { print "rejected: $@" }
}

# A subclass made on the fly still hits the base "pure virtual" die.
{

    package Blob;
    our @ISA = ('Shape');
    sub init { }
}
my $blob = Blob->new( name => 'blob' );
if ( eval { $blob->area; 1 } ) { print "blob has area?!\n" }
else                           { print "virtual: $@" }
