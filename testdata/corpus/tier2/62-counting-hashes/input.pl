#!/usr/bin/perl
use strict;
use warnings;

# The counting hash, at every depth a report actually uses. Nothing here is
# declared with its shape: the ++ and the += build it on the way down.

my @events = (
    'web1 GET /index.html 200 812',
    'web1 GET /style.css 200 1204',
    'web1 POST /login 302 0',
    'db1  GET /health 200 0',
    'db1  GET /health 500 88',
    'web2 GET /index.html 404 155',
    'web2 GET /api/v1/status 200 42',
    'web1 GET /index.html 200 812',
);

my %hits;        # one level:    status -> count
my %by_host;     # two levels:   host -> method -> count
my %matrix;      # three levels: host -> path -> status -> count
my %bytes;       # two levels, accumulating rather than counting
my %seen_paths;  # a list under each key

for my $line (@events) {
    my ( $host, $method, $path, $status, $size ) = split ' ', $line;
    $hits{$status}++;
    $by_host{$host}{$method}++;
    $matrix{$host}{$path}{$status}++;
    $bytes{$host}{$status} += $size;
    push @{ $seen_paths{$host} }, $path;
}

print "-- hits by status --\n";
printf "%s %d\n", $_, $hits{$_} for sort keys %hits;

print "-- methods per host --\n";
for my $host ( sort keys %by_host ) {
    my @parts = map { "$_=$by_host{$host}{$_}" } sort keys %{ $by_host{$host} };
    printf "%-5s %s\n", $host, join( ' ', @parts );
}

print "-- bytes per host and status --\n";
for my $host ( sort keys %bytes ) {
    my $total = 0;
    $total += $bytes{$host}{$_} for keys %{ $bytes{$host} };
    printf "%-5s total=%d statuses=%d\n", $host, $total, scalar keys %{ $bytes{$host} };
}

print "-- three levels deep --\n";
for my $host ( sort keys %matrix ) {
    for my $path ( sort keys %{ $matrix{$host} } ) {
        my @codes = sort keys %{ $matrix{$host}{$path} };
        printf "%-5s %-16s %s\n", $host, $path,
            join( ',', map { "$_:$matrix{$host}{$path}{$_}" } @codes );
    }
}

print "-- lists under keys --\n";
for my $host ( sort keys %seen_paths ) {
    my %uniq;
    $uniq{$_}++ for @{ $seen_paths{$host} };
    printf "%-5s %d request(s), %d distinct\n", $host,
        scalar @{ $seen_paths{$host} }, scalar keys %uniq;
}
