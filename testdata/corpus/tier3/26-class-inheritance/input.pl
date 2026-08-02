#!/usr/bin/perl
# A class hierarchy of the shape @ISA is used for: a base class holding the
# shared state, subclasses supplying one step each, a method the base calls
# on itself that every subclass overrides, and SUPER:: to extend rather than
# replace. The template method is the part Go's embedding cannot express.
use strict;
use warnings;

package Job;

sub new {
    my ( $class, %args ) = @_;
    return bless { name => $args{name}, tries => 0 }, $class;
}

sub name  { $_[0]{name} }
sub tries { $_[0]{tries} }

# The step every subclass supplies.
sub run { return 'nothing to do' }

# The template method: fixed steps, one of them supplied by the subclass.
sub describe {
    my ($self) = @_;
    $self->{tries}++;
    return sprintf '%s: %s', $self->name, $self->run;
}

package Job::Copy;
our @ISA = ('Job');

sub run { my ($self) = @_; return 'copied ' . $self->name }

package Job::Verify;
our @ISA = ('Job::Copy');

sub run {
    my ($self) = @_;
    return $self->SUPER::run() . ' and verified';
}

package main;

my @jobs = (
    Job::Copy->new( name => 'archive' ),
    Job::Verify->new( name => 'backup' ),
);

print "--- direct ---\n";
print $_->run, "\n" for @jobs;

print "--- through the base ---\n";
print $_->describe, "\n" for @jobs;

printf "tries: %d %d\n", $jobs[0]->tries, $jobs[1]->tries;
printf "verify isa Copy: %s\n", ( $jobs[1]->isa('Job::Copy') ? 'yes' : 'no' );
printf "verify isa Job:  %s\n", ( $jobs[1]->isa('Job')       ? 'yes' : 'no' );
printf "copy isa Verify: %s\n", ( $jobs[0]->isa('Job::Verify') ? 'yes' : 'no' );
