#!/usr/bin/perl
use strict;
use warnings;

# Recursion, mutual recursion, memoisation via a file-scoped hash, and
# context-sensitive returns. Note that main runs before the subs are
# textually defined.

print "fib(20) = ", fib(20), "\n";
print "fact(10) = ", fact(10), "\n";

my %memo;

sub fib {
    my ($n) = @_;
    return $n if $n < 2;
    return $memo{$n} if exists $memo{$n};
    return $memo{$n} = fib($n - 1) + fib($n - 2);
}

sub fact {
    my ($n) = @_;
    return 1 if $n <= 1;
    return $n * fact($n - 1);
}

sub is_even { my ($n) = @_; return 1 if $n == 0; return is_odd($n - 1) }
sub is_odd  { my ($n) = @_; return 0 if $n == 0; return is_even($n - 1) }

sub context_report {
    if (wantarray()) {
        return ('list', 'context');
    }
    elsif (defined wantarray()) {
        return 'scalar context';
    }
    else {
        print "void context\n";
        return;
    }
}

sub depth_first {
    my ($node, $depth) = @_;
    $depth ||= 0;
    my @out = (('  ' x $depth) . $node->{name});
    for my $kid (@{ $node->{kids} || [] }) {
        push @out, depth_first($kid, $depth + 1);
    }
    return @out;
}

for my $n (0, 1, 6, 7) {
    printf "%d: even=%d odd=%d\n", $n, is_even($n), is_odd($n);
}

my @list = context_report();
my $scalar = context_report();
print "list   -> @list\n";
print "scalar -> $scalar\n";
context_report();

my $tree = {
    name => 'root',
    kids => [
        { name => 'etc', kids => [ { name => 'passwd' }, { name => 'hosts' } ] },
        { name => 'var', kids => [ { name => 'log', kids => [ { name => 'syslog' } ] } ] },
    ],
};
print "$_\n" for depth_first($tree);

# Recursive sum over an arbitrarily nested array structure.
sub deep_sum {
    my ($thing) = @_;
    return $thing unless ref $thing;
    my $sum = 0;
    $sum += deep_sum($_) for @$thing;
    return $sum;
}
print "deep_sum = ", deep_sum([1, [2, 3], [4, [5, [6, 7]]], 8]), "\n";
