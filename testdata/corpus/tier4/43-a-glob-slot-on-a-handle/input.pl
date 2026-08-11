#!/usr/bin/perl
# TRAP: the pre-lexical way of giving a filehandle its own fields. A glob is
# a symbol-table entry with one slot per sigil, so *$self->{Strict} reads the
# HASH slot of the glob $self points at and uses it as the object's data. The
# IO:: modules are written this way throughout, so any script that leans on
# one meets it.
use strict;
use warnings;

package Tiny::Handle;

sub new {
    my ( $class, %args ) = @_;
    no warnings 'once';
    my $self = \do { local *HANDLE };
    bless $self, $class;
    *$self->{Strict}  = $args{strict} // 0;
    *$self->{ErrorNo} = 0;
    return $self;
}

sub strict_mode { my $self = shift; return *$self->{Strict} }

sub fail {
    my ( $self, $code ) = @_;
    *$self->{ErrorNo} = $code;
    return;
}

sub errno { my $self = shift; return *$self->{ErrorNo} }

package main;

my $h = Tiny::Handle->new( strict => 1 );
printf "strict: %d\n", $h->strict_mode;
$h->fail(42);
printf "errno:  %d\n", $h->errno;
printf "is a glob ref: %s\n", ( ref($h) eq 'Tiny::Handle' ? 'yes' : 'no' );
