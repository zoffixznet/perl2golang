#!/usr/bin/perl
# A hashref class of the shape most scripts write one in: a constructor
# taking named arguments, read-only and read/write accessors, methods that
# mutate and return the object so calls can be chained, a second class that
# keeps the first in a hash, and class-level state shared by every instance.
use strict;
use warnings;

package Account;

my $OPENED = 0;    # class-level state: every instance shares this one

sub new {
    my ( $class, %args ) = @_;
    my $self = {
        id      => $args{id},
        owner   => $args{owner},
        balance => $args{balance},
        frozen  => 0,
    };
    $OPENED++;
    return bless $self, $class;
}

sub opened_count { return $OPENED }

# Read-only accessors: the caller reads, nothing writes.
sub id    { $_[0]{id} }
sub owner { $_[0]{owner} }

# A read/write accessor: given a value it sets, given none it reads.
sub balance {
    my $self = shift;
    $self->{balance} = shift if @_;
    return $self->{balance};
}

sub deposit {
    my ( $self, $amount ) = @_;
    die "deposit must be positive\n" unless $amount > 0;
    $self->{balance} += $amount;
    return $self;    # chainable
}

sub withdraw {
    my ( $self, $amount ) = @_;
    die "account is frozen\n" if $self->{frozen};
    die "insufficient funds\n" if $amount > $self->{balance};
    $self->{balance} -= $amount;
    return $self;
}

sub freeze { my $self = shift; $self->{frozen} = 1; return $self }

sub line {
    my ($self) = @_;
    return sprintf "%-6s %-12s %9.2f %s", $self->id, $self->owner,
      $self->balance, ( $self->{frozen} ? 'frozen' : 'open' );
}

package Ledger;

sub new { my ($class) = @_; return bless { accounts => {} }, $class }

sub add {
    my ( $self, $account ) = @_;
    $self->{accounts}{ $account->id } = $account;
    return $self;
}

sub get { my ( $self, $id ) = @_; return $self->{accounts}{$id} }

sub ids { my ($self) = @_; return sort keys %{ $self->{accounts} } }

sub total {
    my ($self) = @_;
    my $sum = 0;
    $sum += $self->get($_)->balance for $self->ids;
    return $sum;
}

package main;

my $ledger = Ledger->new;
$ledger->add( Account->new( id => 'A-1', owner => 'Jane Doe',  balance => 120 ) );
$ledger->add( Account->new( id => 'A-2', owner => 'John Roe',  balance => 40 ) );
$ledger->add( Account->new( id => 'B-9', owner => 'Ada Byron', balance => 5 ) );

# Method chaining, and mutation through a read/write accessor.
$ledger->get('A-1')->deposit(30)->withdraw(50);
$ledger->get('A-2')->balance(75);
$ledger->get('B-9')->freeze;

print "--- ledger ---\n";
print $ledger->get($_)->line, "\n" for $ledger->ids;
printf "total: %.2f\n", $ledger->total;
printf "accounts opened: %d\n", Account->opened_count;

# The things a class is asked about at run time.
my $one = $ledger->get('A-1');
printf "isa Account: %s\n", ( $one->isa('Account')  ? 'yes' : 'no' );
printf "isa Ledger:  %s\n", ( $one->isa('Ledger')   ? 'yes' : 'no' );
printf "can deposit: %s\n", ( $one->can('deposit')  ? 'yes' : 'no' );
printf "can fly:     %s\n", ( $one->can('fly')      ? 'yes' : 'no' );
printf "ref:         %s\n", ref $one;

# A method that dies, caught and carried past.
my $err = '';
eval { $ledger->get('B-9')->withdraw(1); 1 } or $err = $@;
print "frozen withdrawal: $err";
