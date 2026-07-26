#!/usr/bin/perl
use strict;
use warnings;

# Inventory report. Reads pipe-delimited records into an array of hashes,
# sorts on several keys, and prints a fixed-width table plus a roll-up.

my $file = 'files/servers.txt';
open my $fh, '<', $file or die "cannot open $file: $!\n";

my @servers;
while (my $line = <$fh>) {
    chomp $line;
    next if $line =~ /^\s*$/;
    my ($name, $role, $region, $cpu, $mem, $state) = split /\|/, $line;
    push @servers, {
        name   => $name,
        role   => $role,
        region => $region,
        cpu    => $cpu,
        mem_mb => $mem,
        state  => $state,
    };
}
close $fh or die "close failed: $!\n";

printf "%-8s %-9s %-8s %4s %8s %-5s\n",
    'NAME', 'ROLE', 'REGION', 'CPU', 'MEM(MB)', 'STATE';
printf "%s\n", '-' x 46;

for my $s (sort { $a->{region} cmp $b->{region}
                    || $b->{cpu} <=> $a->{cpu}
                    || $a->{name} cmp $b->{name} } @servers) {
    printf "%-8s %-9s %-8s %4d %8d %-5s\n",
        $s->{name}, $s->{role}, $s->{region}, $s->{cpu}, $s->{mem_mb}, $s->{state};
}

my $total_cpu = 0;
my $total_mem = 0;
my $down      = 0;
for my $s (@servers) {
    $total_cpu += $s->{cpu};
    $total_mem += $s->{mem_mb};
    $down++ if $s->{state} eq 'down';
}

printf "%s\n", '-' x 46;
printf "%d hosts, %d vCPU, %.1f GiB, %d down\n",
    scalar @servers, $total_cpu, $total_mem / 1024, $down;

my @big = grep { $_->{cpu} >= 8 } @servers;
print "large hosts: ", join(', ', map { $_->{name} } sort { $a->{name} cmp $b->{name} } @big), "\n";

my ($first_down) = grep { $_->{state} eq 'down' } @servers;
print "first down record: $first_down->{name} in $first_down->{region}\n";
