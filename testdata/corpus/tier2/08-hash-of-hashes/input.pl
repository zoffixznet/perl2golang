#!/usr/bin/perl
use strict;
use warnings;

# Metric matrix as a hash of hashes: host -> metric -> value. Walks the
# structure two ways and produces both a table and per-metric summaries.

my %metrics = (
    web1   => { cpu => 42, mem => 61, disk => 30 },
    web2   => { cpu => 91, mem => 74, disk => 55 },
    db1    => { cpu => 25, mem => 88, disk => 71 },
    cache1 => { cpu => 12, mem => 33, disk =>  9 },
);

my %seen_metric;
for my $host (keys %metrics) {
    $seen_metric{$_} = 1 for keys %{ $metrics{$host} };
}
my @cols  = sort keys %seen_metric;
my @hosts = sort keys %metrics;

printf "%-8s", 'HOST';
printf "%6s", uc $_ for @cols;
print "\n";

for my $host (@hosts) {
    printf "%-8s", $host;
    printf "%6d", $metrics{$host}{$_} for @cols;
    print "\n";
}

for my $col (@cols) {
    my @vals = map { $metrics{$_}{$col} } @hosts;
    my $sum  = 0;
    $sum += $_ for @vals;
    my ($hi_host) = sort { $metrics{$b}{$col} <=> $metrics{$a}{$col} || $a cmp $b } @hosts;
    printf "%-5s avg=%5.1f max=%3d (%s)\n",
        $col, $sum / @vals, $metrics{$hi_host}{$col}, $hi_host;
}

# Add a derived third level: host -> metric -> { value, status }.
my %detail;
for my $host (@hosts) {
    for my $col (@cols) {
        my $v = $metrics{$host}{$col};
        $detail{$host}{$col} = {
            value  => $v,
            status => $v >= 85 ? 'critical' : $v >= 60 ? 'warning' : 'ok',
        };
    }
}

for my $host (@hosts) {
    my @bad = grep { $detail{$host}{$_}{status} ne 'ok' } @cols;
    next unless @bad;
    printf "%s: %s\n", $host,
        join(' ', map { "$_=$detail{$host}{$_}{status}" } @bad);
}

# exists / delete on nested keys.
print "web1 has swap? ", (exists $metrics{web1}{swap} ? 'yes' : 'no'), "\n";
delete $metrics{cache1}{disk};
print "cache1 keys now: ", join(',', sort keys %{ $metrics{cache1} }), "\n";
print "host count: ", scalar(keys %metrics), "\n";
