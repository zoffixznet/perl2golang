#!/usr/bin/perl
use strict;
use warnings;
use Scalar::Util qw(blessed);

# Structured errors: dying with a hashref and with a blessed exception
# object, dispatching on the error type, and running cleanup on both the
# success and the failure path. Ends with a non-zero exit status.

package MyApp::Error;

sub new {
    my ($class, %args) = @_;
    my $self = {
        code    => $args{code}    || 'E_UNKNOWN',
        message => $args{message} || 'unknown error',
        context => $args{context} || {},
    };
    return bless $self, $class;
}
sub code    { return $_[0]{code} }
sub message { return $_[0]{message} }
sub context { return $_[0]{context} }
sub as_string {
    my ($self) = @_;
    my $ctx = join ' ', map { "$_=$self->{context}{$_}" } sort keys %{ $self->{context} };
    return sprintf('[%s] %s%s', $self->code, $self->message, (length $ctx ? " ($ctx)" : ''));
}

package MyApp::Error::NotFound;
our @ISA = ('MyApp::Error');

package main;

my @log;
sub logmsg { push @log, $_[0]; print "$_[0]\n"; return }

my %db = (1 => 'alpha', 2 => 'beta', 4 => 'delta');

sub fetch {
    my ($id) = @_;
    die MyApp::Error->new(
        code    => 'E_BADARG',
        message => 'id must be a positive integer',
        context => { got => $id },
    ) unless defined $id && $id =~ /^\d+$/ && $id > 0;

    die MyApp::Error::NotFound->new(
        code    => 'E_NOTFOUND',
        message => 'no such record',
        context => { id => $id },
    ) unless exists $db{$id};

    die { code => 'E_LOCKED', id => $id, retry_after => 5 } if $id == 4;

    return $db{$id};
}

# A guard object whose destructor runs whether we leave normally or by die.
package Guard;
sub new { my ($c, $cb) = @_; return bless { cb => $cb }, $c }
sub DESTROY { my ($s) = @_; $s->{cb}->(); return }
package main;

sub with_transaction {
    my ($id) = @_;
    my $committed = 0;
    my $guard = Guard->new(sub {
        logmsg($committed ? "  commit $id" : "  rollback $id");
    });
    my $row = fetch($id);
    $committed = 1;
    return $row;
}

my $failures = 0;
for my $id (1, 3, 4, 'abc', 2) {
    logmsg("fetch(" . (defined $id ? $id : 'undef') . ")");
    my $row = eval { with_transaction($id) };
    my $err = $@;

    if (!$err) {
        logmsg("  ok: $row");
        next;
    }

    $failures++;
    if (blessed($err) && $err->isa('MyApp::Error::NotFound')) {
        logmsg('  missing: ' . $err->as_string);
    }
    elsif (blessed($err) && $err->isa('MyApp::Error')) {
        logmsg('  app error: ' . $err->as_string);
    }
    elsif (ref $err eq 'HASH') {
        logmsg(sprintf('  plain hashref error: %s',
            join(' ', map { "$_=$err->{$_}" } sort keys %$err)));
    }
    else {
        my $text = $err; chomp $text;
        logmsg("  unexpected: $text");
    }
}

# Propagation: a wrapper that re-throws after adding context.
sub guarded_fetch {
    my ($id) = @_;
    my $row = eval { fetch($id) };
    if (my $e = $@) {
        die MyApp::Error->new(
            code    => 'E_WRAPPED',
            message => 'guarded_fetch failed',
            context => {
                id    => $id,
                cause => blessed($e) ? $e->code : (ref($e) eq 'HASH' ? $e->{code} : 'raw'),
            },
        );
    }
    return $row;
}

for my $id (2, 99, 4) {
    my $v = eval { guarded_fetch($id) };
    if (my $e = $@) {
        logmsg('wrapped: ' . $e->as_string);
    }
    else {
        logmsg("wrapped ok: $v");
    }
}

printf "%d log line(s), %d failure(s)\n", scalar @log, $failures;
exit($failures > 0 ? 1 : 0);
