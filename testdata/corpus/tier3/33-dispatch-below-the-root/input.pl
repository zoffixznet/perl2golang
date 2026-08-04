#!/usr/bin/perl
use strict;
use warnings;

# The template method's neighbour: the method that dispatches is declared
# halfway down the hierarchy rather than at the top of it, and one class
# reaches its parent's version by name on the way past.

package Node;
sub new { my ( $class, %a ) = @_; return bless { name => $a{name}, kids => $a{kids} }, $class }
sub name { $_[0]{name} }
sub kind { 'node' }

package Node::Container;
our @ISA = ('Node');

# Declared here, not on Node. Node::Container::describe calls a method that
# Node::Container::Sorted replaces, so the dispatch has to happen through a
# type that only the classes below Node::Container satisfy.
sub children { @{ $_[0]{kids} || [] } }

sub describe {
    my ($self) = @_;
    my @kids = $self->children;
    return sprintf '%s(%s) holds %d: %s', $self->kind, $self->name,
        scalar @kids, join( ',', @kids );
}

sub kind { 'container' }

package Node::Container::Sorted;
our @ISA = ('Node::Container');
sub kind     { 'sorted' }
sub children { sort @{ $_[0]{kids} || [] } }

package Node::Container::Loud;
our @ISA = ('Node::Container');
sub kind { 'LOUD' }

sub describe {
    my ($self) = @_;
    # SUPER:: asks for the parent's version by name, which is the one shape
    # of dispatch Go's embedding does express.
    return uc( $self->SUPER::describe() );
}

package main;

my @kids = qw(pear apple fig);

for my $n ( Node::Container->new( name => 'box', kids => \@kids ),
            Node::Container::Sorted->new( name => 'box', kids => \@kids ),
            Node::Container::Loud->new( name => 'box', kids => \@kids ) ) {
    print $n->describe, "\n";
}

my $plain = Node->new( name => 'leaf' );
printf "%s(%s)\n", $plain->kind, $plain->name;
