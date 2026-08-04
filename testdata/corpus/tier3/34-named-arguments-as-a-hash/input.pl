#!/usr/bin/perl
use strict;
use warnings;

# The other half of named arguments: a constructor that does not name the
# keys it wants, but walks whatever hash the caller passed. There is nothing
# to turn into a Go parameter list, because the parameter list is the data.

package Record;

my %ALLOWED = map { $_ => 1 } qw(title author year);

sub new {
    my ( $class, %args ) = @_;
    my $self = bless { fields => {} }, $class;
    for my $key ( sort keys %args ) {
        next unless $ALLOWED{$key};
        $self->{fields}{$key} = $args{$key};
    }
    $self->{count} = scalar keys %{ $self->{fields} };
    return $self;
}

sub describe {
    my ($self) = @_;
    return join ', ',
      map { "$_=$self->{fields}{$_}" } sort keys %{ $self->{fields} };
}

package main;

my $r = Record->new(
    title  => 'Dune',
    author => 'Herbert',
    year   => 1965,
    colour => 'orange',
);
printf "kept %d field(s)\n", $r->{count};
printf "record: %s\n", $r->describe;

my $bare = Record->new( title => 'Untitled' );
printf "bare:   %s\n", $bare->describe;
