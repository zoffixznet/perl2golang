#!/usr/bin/perl
# A release step described by a chaining builder, run under scope guards.
# Every with_* returns $self, so a Step built as a subclass must survive the
# chain as itself; the guards release in the order their scopes close.
use strict;
use warnings;

package Step;

sub new {
    my ( $class, $name ) = @_;
    return bless { name => $name, flags => [] }, $class;
}
sub with_flag { my ( $self, $f ) = @_; push @{ $self->{flags} }, $f; return $self }
sub with_retries { my ( $self, $n ) = @_; $self->{retries} = $n; return $self }

sub describe {
    my ($self) = @_;
    my $kind = ref $self;
    my $flags = join( ',', @{ $self->{flags} } ) || 'none';
    return sprintf '%s %s [%s] retries=%d', $kind, $self->{name}, $flags,
      $self->{retries} // 0;
}

sub run { my ($self) = @_; print "run: ", $self->describe, "\n" }

package Step::Risky;
our @ISA = ('Step');
sub run {
    my ($self) = @_;
    print "run (guarded): ", $self->describe, "\n";
}

package Lock;

sub new {
    my ( $class, $what ) = @_;
    print "lock $what\n";
    return bless { what => $what }, $class;
}
sub DESTROY { my ($self) = @_; print "unlock $self->{what}\n" }

package main;

# A subclass built through base-class chaining must still run its own run().
my $deploy = Step::Risky->new('deploy')->with_flag('canary')->with_retries(2);
my $verify = Step->new('verify')->with_flag('deep')->with_flag('slow');

{
    my $db = Lock->new('database');
    $deploy->run;
}    # database unlocks here, before verify begins

my $conf = Lock->new('config');
$verify->run;
undef $conf;    # config unlocks here, by hand
print "released early\n";

sub audited {
    my ($step) = @_;
    my $log = Lock->new('audit-trail');
    return uc $step->describe;    # audit-trail unlocks as the sub returns
}
print "audited: ", audited($deploy), "\n";
print "done\n";
