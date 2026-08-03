#!/usr/bin/perl
# Nested data, which is where a converter either works out what a program
# holds or gives up and calls everything "anything". Hash of arrays, hash of
# hashes, arrays of records, and the copy-then-edit idiom.
use strict;
use warnings;

my @rows = (
    'ann disk 40',
    'bob net 91',
    'ann cpu 12',
    'bob disk 77',
);

# ---- hash of arrays, built by autovivification -------------------------
my %by_owner;
my %usage;
my %totals;
for my $row (@rows) {
    my ( $owner, $kind, $pct ) = split ' ', $row;
    push @{ $by_owner{$owner} }, $kind;
    $usage{$owner}{$kind} = $pct;
    $totals{$owner} += $pct;
}

for my $owner ( sort keys %by_owner ) {
    printf "%s: %s (total %d)\n", $owner,
        join( ',', @{ $by_owner{$owner} } ), $totals{$owner};
}

# ---- hash of hashes, read back two levels down -------------------------
for my $owner ( sort keys %usage ) {
    for my $kind ( sort keys %{ $usage{$owner} } ) {
        printf "  %s/%s = %d\n", $owner, $kind, $usage{$owner}{$kind};
    }
}

# ---- an array of records built by a sub --------------------------------
sub parse_rows {
    my (@lines) = @_;
    my @out;
    for my $line (@lines) {
        my ( $owner, $kind, $pct ) = split ' ', $line;
        push @out, { owner => $owner, kind => $kind };
    }
    return @out;
}

my @records = parse_rows(@rows);
printf "records: %d, first owner %s\n", scalar @records, $records[0]{owner};

# ---- a comparator held in a variable -----------------------------------
my %ORDER = (
    kind  => sub { $a->{kind} cmp $b->{kind} or $a->{owner} cmp $b->{owner} },
    owner => sub { $a->{owner} cmp $b->{owner} or $a->{kind} cmp $b->{kind} },
);
for my $which (qw(kind owner)) {
    my $cmp    = $ORDER{$which};
    my @sorted = sort $cmp @records;
    print "by $which: ", join( ' ', map { "$_->{owner}/$_->{kind}" } @sorted ), "\n";
}

# ---- copy then edit, without touching the original ---------------------
my $raw = '  91%  ';
( my $pct = $raw ) =~ s/^\s+|\s+$//g;
$pct =~ s/%$//;
print "raw=[$raw] pct=[$pct]\n";

# ---- a list behind a reference, asked how long it is -------------------
my $lists = { small => [ 1, 2 ], large => [ 1 .. 5 ] };
for my $name ( sort keys %$lists ) {
    printf "%s has %d\n", $name, scalar @{ $lists->{$name} };
}
