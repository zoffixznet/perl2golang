#!/usr/bin/perl
use strict;
use warnings;

# A tiny command interpreter driven by a dispatch table of code refs, with
# callbacks handed to a generic walker. This is the shape most Perl
# scripts use instead of a long if/elsif chain.

my %store = (alpha => 3, beta => 7);

my %commands = (
    set => sub {
        my ($key, $val) = @_;
        $store{$key} = $val;
        return "set $key=$val";
    },
    get => sub {
        my ($key) = @_;
        return exists $store{$key} ? "$key=$store{$key}" : "$key is unset";
    },
    incr => sub {
        my ($key, $by) = @_;
        $by = 1 unless defined $by;
        $store{$key} += $by;
        return "$key now $store{$key}";
    },
    del => sub {
        my ($key) = @_;
        my $had = delete $store{$key};
        return defined $had ? "deleted $key (was $had)" : "no such key $key";
    },
    list => sub {
        return join ' ', map { "$_=$store{$_}" } sort keys %store;
    },
);

sub run {
    my ($line) = @_;
    my ($verb, @args) = split ' ', $line;
    my $handler = $commands{$verb};
    return "ERROR: unknown command '$verb'" unless $handler;
    return $handler->(@args);
}

for my $line ('list', 'get alpha', 'set gamma 11', 'incr beta 5',
              'incr gamma', 'del alpha', 'get alpha', 'frobnicate x', 'list') {
    printf "%-14s | %s\n", $line, run($line);
}

print "known verbs: ", join(',', sort keys %commands), "\n";

# Callbacks: a generic walker that takes a code ref per node type.
sub walk {
    my ($data, %cb) = @_;
    my $type = ref $data;
    if ($type eq 'ARRAY') {
        $cb{array}->(scalar @$data) if $cb{array};
        walk($_, %cb) for @$data;
    }
    elsif ($type eq 'HASH') {
        $cb{hash}->(scalar keys %$data) if $cb{hash};
        walk($data->{$_}, %cb) for sort keys %$data;
    }
    else {
        $cb{leaf}->($data) if $cb{leaf};
    }
    return;
}

my $doc = {
    name  => 'report',
    rows  => [ 1, 2, { deep => 'yes', more => [ 3, 4 ] } ],
    owner => 'ada',
};

my @leaves;
my ($arrays, $hashes) = (0, 0);
walk($doc,
    array => sub { $arrays++ },
    hash  => sub { $hashes++ },
    leaf  => sub { push @leaves, $_[0] },
);
print "arrays=$arrays hashes=$hashes leaves=", join('|', @leaves), "\n";

# Building code refs in a loop and stashing them in a table.
my %ops;
for my $spec ([add => sub { $_[0] + $_[1] }],
              [sub_ => sub { $_[0] - $_[1] }],
              [mul => sub { $_[0] * $_[1] }]) {
    my ($name, $fn) = @$spec;
    $ops{$name} = $fn;
}
printf "%s(9,4)=%s\n", $_, $ops{$_}->(9, 4) for sort keys %ops;

# Chaining: a sub that returns a sub selected from the table.
sub op_for { my ($n) = @_; return $ops{$n} || sub { 'n/a' } }
print "chained: ", op_for('mul')->(6, 7), " / ", op_for('nope')->(1, 2), "\n";
