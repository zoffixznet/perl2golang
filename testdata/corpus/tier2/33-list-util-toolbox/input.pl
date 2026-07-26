#!/usr/bin/perl
use strict;
use warnings;
use List::Util qw(sum sum0 max min maxstr minstr first reduce any all none uniq uniqnum pairs);

# The List::Util functions a data-wrangling script leans on, applied to a
# small load-average table.

my @hosts;
my %samples;

open my $fh, '<', 'files/samples.txt' or die "open: $!\n";
while (my $line = <$fh>) {
    chomp $line;
    next unless $line =~ /\S/;
    my ($host, @vals) = split ' ', $line;
    push @hosts, $host;
    $samples{$host} = \@vals;
}
close $fh or die "close: $!\n";

printf "%-8s %6s %6s %6s %6s\n", 'HOST', 'SUM', 'MIN', 'MAX', 'MEAN';
for my $host (@hosts) {
    my @v = @{ $samples{$host} };
    printf "%-8s %6.2f %6.2f %6.2f %6.2f\n",
        $host, sum(@v), min(@v), max(@v), sum(@v) / @v;
}

my @all_values = map { @{ $samples{$_} } } @hosts;
printf "overall: n=%d sum=%.2f min=%.2f max=%.2f\n",
    scalar @all_values, sum(@all_values), min(@all_values), max(@all_values);

# sum returns undef on an empty list; sum0 returns 0.
my @empty;
printf "sum(empty)=%s sum0(empty)=%d\n",
    (defined sum(@empty) ? sum(@empty) : 'undef'), sum0(@empty);

# String comparison variants.
printf "maxstr=%s minstr=%s\n", maxstr(@hosts), minstr(@hosts);

# first: stop at the first match, returning undef if there is none.
my $busy = first { max(@{ $samples{$_} }) > 0.85 } @hosts;
my $idle = first { max(@{ $samples{$_} }) < 0.05 } @hosts;
printf "first busy host: %s\n", (defined $busy ? $busy : 'none');
printf "first idle host: %s\n", (defined $idle ? $idle : 'none');

# reduce: fold a list with an accumulator in $a and the item in $b.
my $product = reduce { $a * $b } 1, 2, 3, 4, 5;
my $longest = reduce { length($a) >= length($b) ? $a : $b } @hosts;
my $joined  = reduce { "$a>$b" } @hosts;
printf "product=%d longest=%s chain=%s\n", $product, $longest, $joined;

# reduce building a structure instead of a scalar.
my $totals = reduce {
    $a->{ $b } = sum(@{ $samples{$b} });
    $a;
} {}, @hosts;
printf "totals: %s\n", join(' ', map { sprintf('%s=%.2f', $_, $totals->{$_}) } sort keys %$totals);

# any / all / none as readable predicates.
printf "any over 0.9?  %s\n", ((any { $_ > 0.9 } @all_values) ? 'yes' : 'no');
printf "all positive?  %s\n", ((all { $_ > 0 } @all_values) ? 'yes' : 'no');
printf "none over 1.0? %s\n", ((none { $_ > 1.0 } @all_values) ? 'yes' : 'no');
printf "all hosts named? %s\n", ((all { /^\w+\d$/ } @hosts) ? 'yes' : 'no');

# uniq preserves order; uniqnum compares numerically.
my @tags = qw(prod prod web web db prod edge db);
printf "uniq: %s (%d of %d)\n", join(',', uniq @tags), scalar(uniq @tags), scalar @tags;
my @numish = (1, '1.0', 1.0, 2, '2', 3);
printf "uniq on numish:    %s\n", join(',', uniq @numish);
printf "uniqnum on numish: %s\n", join(',', uniqnum @numish);

# pairs turns a flat list into arrayrefs, handy after a //g match.
my @flat = (alpha => 1, beta => 2, gamma => 3);
printf "pairs: %s\n", join(' ', map { "$_->[0]:$_->[1]" } pairs @flat);

# Composing them: rank hosts by peak, keep the top three.
my @ranked = sort { max(@{ $samples{$b} }) <=> max(@{ $samples{$a} }) || $a cmp $b } @hosts;
printf "top 3 by peak: %s\n", join(', ',
    map { sprintf('%s(%.2f)', $_, max(@{ $samples{$_} })) } @ranked[0 .. 2]);
