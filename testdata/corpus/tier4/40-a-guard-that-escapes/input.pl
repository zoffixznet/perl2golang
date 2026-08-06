#!/usr/bin/perl
# A guard object whose reference escapes its block. The lock is registered
# with a keeper, so the reference count keeps it alive past the closing
# brace, and the release only happens at program exit. Explicit destruction
# cannot follow this lifetime: only the reference count knows the instant.
use strict;
use warnings;

package Lock;

sub new {
    my ( $class, $what ) = @_;
    print "lock $what\n";
    return bless { what => $what }, $class;
}
sub DESTROY { my ($self) = @_; print "unlock $self->{what}\n" }

package main;

my @keep;
sub remember { push @keep, $_[0] }

{
    my $g = Lock->new('shared-state');
    remember($g);
    print "block done\n";
}    # NOT released here: @keep still holds it
print "after block\n";
