#!/usr/bin/perl
use strict;
use warnings;

# Access-log roll-up. Every counter bucket below springs into existence on
# first touch: %tally, %by_client and %matrix are never pre-declared with
# their inner structures, they autovivify as the ++ walks down.

my %tally;
my %by_client;
my %matrix;
my %paths_seen;

open my $fh, '<', 'files/access.log' or die "open: $!\n";
while (my $line = <$fh>) {
    chomp $line;
    my ($ip, $method, $path, $status) = split ' ', $line;

    $tally{$status}++;                       # hash autoviv
    $by_client{$ip}{$method}++;              # two levels deep
    $matrix{$ip}{$path}{$status}++;          # three levels deep
    push @{ $paths_seen{$path} }, $ip;       # array autoviv inside a hash
}
close $fh or die "close: $!\n";

print "-- status counts --\n";
printf "%s %d\n", $_, $tally{$_} for sort keys %tally;

print "-- per client methods --\n";
for my $ip (sort keys %by_client) {
    my $inner = $by_client{$ip};
    printf "%-10s %s\n", $ip,
        join(' ', map { "$_=$inner->{$_}" } sort keys %$inner);
}

print "-- three level matrix --\n";
for my $ip (sort keys %matrix) {
    for my $path (sort keys %{ $matrix{$ip} }) {
        for my $status (sort keys %{ $matrix{$ip}{$path} }) {
            printf "%s %s %s %d\n", $ip, $path, $status, $matrix{$ip}{$path}{$status};
        }
    }
}

print "-- paths --\n";
for my $path (sort keys %paths_seen) {
    my %uniq;
    my @clients = grep { !$uniq{$_}++ } @{ $paths_seen{$path} };
    printf "%-12s hits=%d clients=%s\n",
        $path, scalar @{ $paths_seen{$path} }, join(',', sort @clients);
}

# Reading a missing nested key with exists does NOT create it; reading it
# through an intermediate level does. Both behaviours are shown.
print "exists 10.0.0.9? ", (exists $matrix{'10.0.0.9'} ? 'yes' : 'no'), "\n";
my $probe = $matrix{'10.0.0.9'}{'/nope'};
print "after probe, 10.0.0.9 exists? ", (exists $matrix{'10.0.0.9'} ? 'yes' : 'no'), "\n";
print "and /nope under it? ", (exists $matrix{'10.0.0.9'}{'/nope'} ? 'yes' : 'no'), "\n";
print "client keys: ", join(',', sort keys %matrix), "\n";
