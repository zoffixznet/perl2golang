#!/usr/bin/perl
# Exception objects as blessed hashrefs, an error class hierarchy,
# rethrow-with-context, nested eval, the classic $@-clobbering-in-DESTROY
# hazard, and the local $@ fix.
use strict;
use warnings;

# ---- error classes -----------------------------------------------------
package App::Error;

sub new {
    my ( $class, %args ) = @_;
    return bless {
        message => $args{message} // 'unknown error',
        context => $args{context} // [],
    }, $class;
}

sub throw { my $class = shift; die $class->new(@_) }
sub message { $_[0]{message} }

sub with_context {
    my ( $self, $note ) = @_;
    push @{ $self->{context} }, $note;
    return $self;
}

sub describe {
    my ($self) = @_;
    my $s = sprintf '[%s] %s', ref $self, $self->message;
    $s .= ' (' . join( ' <- ', @{ $self->{context} } ) . ')'
      if @{ $self->{context} };
    return $s;
}

sub is_retryable { 0 }

package App::Error::IO;
our @ISA = ('App::Error');
sub is_retryable { 1 }

package App::Error::Parse;
our @ISA = ('App::Error');

package App::Error::Timeout;
our @ISA = ('App::Error::IO');    # timeouts are IO errors, also retryable

# ---- a guard with a DESTROY that must not clobber $@ -------------------
package Guard;

my $log = '';
sub log_lines { my @l = split /\n/, $log; $log = ''; @l }

sub new {
    my ( $class, $name, $careful ) = @_;
    return bless { name => $name, careful => $careful }, $class;
}

sub DESTROY {
    my ($self) = @_;
    if ( $self->{careful} ) {
        local $@;    # the fix: protect caller's $@ from our eval
        eval { $log .= "guard $self->{name} released (careful)\n"; 1 };
    }
    else {
        # Historic hazard: on perl < 5.14 an eval here wiped the caller's
        # $@ mid-unwind. Modern perl saves $@ around destructors, so both
        # branches now behave alike -- but converters must not reintroduce
        # the old bug when modelling deferred cleanup.
        eval { $log .= "guard $self->{name} released (sloppy)\n"; 1 };
    }
}

# ---- worker ------------------------------------------------------------
package main;

my %jobs = (
    'fetch-feed'   => sub { App::Error::Timeout->throw( message => 'no response in 5s' ) },
    'parse-config' => sub { App::Error::Parse->throw( message => 'unexpected token }' ) },
    'read-cache'   => sub { App::Error::IO->throw( message => 'cache file missing' ) },
    'plain-die'    => sub { die "string death, not an object\n" },
    'clean-run'    => sub { return 'ok:42' },
);

sub run_job {
    my ($name) = @_;
    my $result = eval {
        my $inner = eval { $jobs{$name}->() };
        if ( my $err = $@ ) {
            # promote strings, annotate objects, rethrow either way
            if ( ref $err && $err->isa('App::Error') ) {
                die $err->with_context("job=$name");
            }
            chomp( my $msg = $err );
            App::Error->throw( message => $msg,
                context => ["promoted in job=$name"] );
        }
        $inner;
    };
    return ( $result, $@ );
}

for my $name ( sort keys %jobs ) {
    my ( $result, $err ) = run_job($name);
    if ( !$err ) {
        print "$name: succeeded ($result)\n";
        next;
    }
    my $kind =
        $err->isa('App::Error::Timeout') ? 'timeout'
      : $err->isa('App::Error::IO')      ? 'io'
      : $err->isa('App::Error::Parse')   ? 'parse'
      :                                    'generic';
    printf "%s: %s %s retry=%s\n", $name, $kind, $err->describe,
      $err->is_retryable ? 'yes' : 'no';
}

# ---- the $@ clobbering demonstration -----------------------------------
print "--- clobber demo ---\n";
for my $careful ( 0, 1 ) {
    my $seen;
    eval {
        my $g = Guard->new( $careful ? 'safe' : 'racy', $careful );
        App::Error::IO->throw( message => 'disk on fire' );
    };
    $seen = ref $@ ? $@->describe : "(\$\@ was '$@')";
    printf "careful=%d -> caught: %s\n", $careful, $seen;
    print "  log: $_\n" for Guard::log_lines();
}

# ---- nested eval where the inner failure is survivable -----------------
my $value = eval {
    my $primary = eval { App::Error::IO->throw( message => 'primary down' ) };
    if ($@) {
        print "falling back after: ", $@->message, "\n";
        'fallback-value';
    }
    else { $primary }
};
die "outer eval should not have failed: $@" if $@;
print "recovered value: $value\n";
