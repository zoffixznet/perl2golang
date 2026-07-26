#!/usr/bin/perl
use strict;
use warnings;

# Closures: counters that keep private state, generators, and the classic
# "close over the loop variable" behaviour that surprises people coming
# from languages with a single shared loop slot.

sub make_counter {
    my ($start, $step) = @_;
    $start = 0 unless defined $start;
    $step  = 1 unless defined $step;
    my $n = $start;
    return sub { my $cur = $n; $n += $step; return $cur };
}

my $tick = make_counter();
my $odd  = make_counter(1, 2);
print "tick: ", join(' ', map { $tick->() } 1 .. 5), "\n";
print "odd:  ", join(' ', map { $odd->() } 1 .. 5), "\n";
print "tick continues: ", $tick->(), "\n";

# Two closures sharing one private variable.
sub make_account {
    my ($balance) = @_;
    my $deposit  = sub { $balance += $_[0]; return $balance };
    my $withdraw = sub {
        return undef if $_[0] > $balance;
        $balance -= $_[0];
        return $balance;
    };
    my $peek = sub { return $balance };
    return ($deposit, $withdraw, $peek);
}

my ($dep, $wd, $peek) = make_account(100);
print "deposit 50 -> ", $dep->(50), "\n";
print "withdraw 30 -> ", $wd->(30), "\n";
my $bad = $wd->(1000);
print "withdraw 1000 -> ", (defined $bad ? $bad : 'refused'), "\n";
print "balance -> ", $peek->(), "\n";

# Each iteration of a foreach with a `my` variable gets its own binding, so
# each closure captures a distinct value.
my @printers;
for my $name (qw(alpha beta gamma)) {
    push @printers, sub { return "printer for $name" };
}
print $_->(), "\n" for @printers;

# A C-style loop shares one variable, so all closures see the final value.
my @shared;
for (my $i = 0; $i < 3; $i++) {
    push @shared, sub { return $i };
}
print "shared: ", join(' ', map { $_->() } @shared), "\n";

# Generator over a list, returning undef when exhausted.
sub iterator {
    my @items = @_;
    my $idx   = 0;
    return sub {
        return if $idx >= @items;
        return $items[ $idx++ ];
    };
}

my $it = iterator(qw(red green blue));
while (defined(my $colour = $it->())) {
    print "colour: $colour\n";
}

# Memoising wrapper built from a closure around a code ref.
sub memoize {
    my ($fn) = @_;
    my %cache;
    my $calls = 0;
    return (sub {
        my ($arg) = @_;
        $cache{$arg} = $fn->($arg) unless exists $cache{$arg};
        return $cache{$arg};
    }, sub { return $calls });
}

my $raw_calls = 0;
my ($slow, $counter) = memoize(sub { $raw_calls++; return $_[0] ** 3 });
print "cubes: ", join(' ', map { $slow->($_) } (2, 3, 2, 4, 3, 2)), "\n";
print "underlying calls: $raw_calls\n";
