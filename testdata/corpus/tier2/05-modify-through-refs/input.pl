#!/usr/bin/perl
use strict;
use warnings;

# Passing structures into subs by reference and mutating them in place,
# plus the @_ aliasing that lets a sub write to its caller's variables.

sub normalise_in_place {
    my ($aref) = @_;
    for my $item (@$aref) {
        $item =~ s/^\s+|\s+$//g;
        $item = lc $item;
    }
    return;
}

sub bump_counts {
    my ($href, @keys) = @_;
    $href->{$_}++ for @keys;
    return scalar keys %$href;
}

sub push_all {
    my ($aref, @items) = @_;
    push @$aref, @items;
    return scalar @$aref;
}

# @_ aliases the caller's scalars; \@_ makes that explicit.
sub increment_args {
    my $args = \@_;
    $_ += 1 for @$args;
    return scalar @$args;
}

sub swap {
    @_[0, 1] = @_[1, 0];
    return;
}

my @raw = ('  Alpha ', "BETA\t", ' Gamma');
normalise_in_place(\@raw);
print "normalised: [", join('][', @raw), "]\n";

my %tally;
my $distinct = bump_counts(\%tally, qw(a b a c a b));
print "distinct=$distinct ", join(' ', map { "$_=$tally{$_}" } sort keys %tally), "\n";

my @queue = (1, 2);
my $len = push_all(\@queue, 3, 4, 5);
print "queue len=$len contents=@queue\n";

my ($x, $y, $z) = (10, 20, 30);
my $touched = increment_args($x, $y, $z);
print "after increment_args: x=$x y=$y z=$z touched=$touched\n";

my ($left, $right) = ('port', 'starboard');
swap($left, $right);
print "after swap: left=$left right=$right\n";

# Nested structure mutated through a chain of references.
my %config = (
    limits => { cpu => 1, mem => 512 },
    hosts  => [ 'web1', 'web2' ],
);

sub raise_limit {
    my ($cfg, $key, $factor) = @_;
    $cfg->{limits}{$key} *= $factor;
    return $cfg->{limits}{$key};
}

sub add_host {
    my ($cfg, $host) = @_;
    push @{ $cfg->{hosts} }, $host;
    return;
}

raise_limit(\%config, 'mem', 4);
raise_limit(\%config, 'cpu', 8);
add_host(\%config, 'web3');

print "limits: ", join(' ', map { "$_=$config{limits}{$_}" } sort keys %{ $config{limits} }), "\n";
print "hosts: @{ $config{hosts} }\n";

# A ref handed to a sub that replaces the *contents* but not the ref itself.
sub reset_to {
    my ($aref, @new) = @_;
    @$aref = @new;
    return;
}
reset_to(\@queue, 'x', 'y');
print "queue now=@queue\n";
