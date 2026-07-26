#!/usr/bin/perl
use strict;
use warnings;

# Batch-size helper used by a nightly job. Exercises the plain positional
# calling conventions Perl subs are usually written with.

sub greet {
    my ($name, $greeting) = @_;
    $greeting = 'Hello' unless defined $greeting;
    return "$greeting, $name!";
}

sub total {
    my $sum = 0;
    $sum += $_ for @_;
    return $sum;
}

sub head_and_rest {
    my $first = shift;
    my @rest  = @_;
    return ($first, scalar @rest);
}

sub classify {
    my ($n) = @_;
    return 'empty'  if $n == 0;
    return 'single' if $n == 1;
    return 'few'    if $n < 5;
    return 'many';
}

sub stats {
    my @nums = @_;
    return unless @nums;            # early return: empty list
    my @sorted = sort { $a <=> $b } @nums;
    return ($sorted[0], $sorted[-1], total(@nums) / @nums);
}

my @counts = (3, 9, 4, 1, 7);

print greet('Ada'), "\n";
print greet('Bob', 'Howdy'), "\n";
print "total: ", total(@counts), "\n";

my ($first, $n_rest) = head_and_rest(@counts);
print "first=$first rest=$n_rest\n";

for my $n (0, 1, 3, 12) {
    printf "%2d => %s\n", $n, classify($n);
}

my ($min, $max, $avg) = stats(@counts);
printf "min=%d max=%d avg=%.2f\n", $min, $max, $avg;

my @none = stats();
printf "stats() on empty input returned %d values\n", scalar @none;

# The same sub harvested in list context and then counted.
my @pair  = head_and_rest(@counts);
my $howmany = () = head_and_rest(@counts);
print "pair=@pair howmany=$howmany\n";

# A sub used as the last expression of another sub (implicit return).
sub double_total { total(@_) * 2 }
print "double: ", double_total(@counts), "\n";
