#!/usr/bin/perl
use strict;
use warnings;

# Account provisioning helper: named arguments passed as a flat key/value
# list, merged over a defaults hash, returned both as a hash and a hashref.

sub make_user {
    my %args = @_;

    my %defaults = (
        role   => 'guest',
        active => 1,
        quota  => 100,
        shell  => '/bin/sh',
    );

    my %user = (%defaults, %args);
    die "make_user: 'name' is required\n" unless defined $user{name};
    return %user;
}

sub make_user_ref {
    my %user = make_user(@_);
    return \%user;
}

sub describe {
    my (%u) = @_;
    return sprintf('%-8s role=%-7s active=%d quota=%5d shell=%s',
        $u{name}, $u{role}, $u{active}, $u{quota}, $u{shell});
}

sub keys_of {
    my ($href) = @_;
    return join ',', sort keys %$href;
}

my @requests = (
    [ name => 'ada' ],
    [ name => 'bob',  role => 'admin', quota => 50_000 ],
    [ name => 'cleo', active => 0, shell => '/usr/sbin/nologin' ],
);

for my $req (@requests) {
    my %u = make_user(@$req);
    print describe(%u), "\n";
}

my $ref = make_user_ref(name => 'dora', role => 'staff');
print "ref keys: ", keys_of($ref), "\n";
print "ref role: $ref->{role}\n";

# A named-argument sub that also reports which options were overridden.
sub overrides {
    my %args = @_;
    my @given = grep { $_ ne 'name' } sort keys %args;
    return @given ? join('+', @given) : 'none';
}
print "overrides: ", overrides(name => 'bob', role => 'admin', quota => 5), "\n";
print "overrides: ", overrides(name => 'ada'), "\n";
