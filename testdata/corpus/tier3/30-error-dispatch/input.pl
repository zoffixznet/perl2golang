#!/usr/bin/perl
# Exception objects thrown, caught in $@, and sorted out by asking what they
# are. No destructor and no overloading: this is the part of an error
# hierarchy an ordinary script actually uses.
use strict;
use warnings;

package Failure;

sub new {
    my ( $class, %args ) = @_;
    return bless {
        detail => $args{detail} // 'no detail',
        code   => $args{code}   // 0,
    }, $class;
}

sub throw  { my $class = shift; die $class->new(@_) }
sub detail { $_[0]{detail} }
sub code   { $_[0]{code} }
sub label  { 'failure' }

sub report {
    my ($self) = @_;
    return sprintf '%s(%d): %s', $self->label, $self->{code}, $self->{detail};
}

package Failure::Network;
our @ISA = ('Failure');
sub label { 'network' }

package Failure::Timeout;
our @ISA = ('Failure::Network');
sub label { 'timeout' }

package Failure::Config;
our @ISA = ('Failure');
sub label { 'config' }

package main;

sub attempt {
    my ($what) = @_;
    if    ( $what eq 'net' )    { Failure::Network->throw( detail => 'connection refused', code => 61 ) }
    elsif ( $what eq 'slow' )   { Failure::Timeout->throw( detail => 'no answer in 5s',    code => 60 ) }
    elsif ( $what eq 'config' ) { Failure::Config->throw( detail => 'missing key host',    code => 22 ) }
    return "$what ok";
}

for my $what (qw(fine net slow config)) {
    my $result = eval { attempt($what) };
    if ( !$@ ) {
        print "$what: $result\n";
        next;
    }
    my $err = $@;
    printf "%-7s caught %-9s network=%-3s timeout=%-3s code=%d\n",
        $what,
        $err->label,
        ( $err->isa('Failure::Network') ? 'yes' : 'no' ),
        ( $err->isa('Failure::Timeout') ? 'yes' : 'no' ),
        $err->code;
    print "        ", $err->report, "\n";
}

# An eval that succeeds leaves $@ empty even when one inside it failed.
my $outer = eval {
    my $inner = eval { Failure->throw( detail => 'inner problem', code => 1 ) };
    $@ ? 'recovered' : $inner;
};
printf "outer eval: value=%s error=%s\n", $outer, ( $@ ? 'set' : 'empty' );

# Every class in the hierarchy answers the same three methods, so they can be
# collected and walked together.
my @all = (
    Failure->new( detail => 'plain', code => 1 ),
    Failure::Network->new( detail => 'refused', code => 61 ),
    Failure::Timeout->new( detail => 'slow', code => 60 ),
    Failure::Config->new( detail => 'bad key', code => 22 ),
);
print "-- every failure --\n";
print '  ', $_->report, "\n" for @all;
