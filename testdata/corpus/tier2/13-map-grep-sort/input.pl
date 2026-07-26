#!/usr/bin/perl
use strict;
use warnings;

# List pipelines: map/grep with multi-statement blocks, sort with custom
# comparators, and a Schwartzian transform.

my @lines = (
    'ada lovelace,1815,analyst',
    'grace hopper,1906,admiral',
    'alan turing,1912,logician',
    'edsger dijkstra,1930,professor',
    'barbara liskov,1939,professor',
);

# map producing more than one element per input.
my @flat = map { my @f = split /,/; ($f[0], $f[1]) } @lines;
print "flat count: ", scalar @flat, "\n";

# map producing hashrefs, with a multi-statement block.
my @people = map {
    my ($name, $year, $role) = split /,/;
    my ($first, $last) = split ' ', $name;
    {
        name    => $name,
        last    => $last,
        year    => $year,
        role    => $role,
        century => int($year / 100) + 1,
    };
} @lines;

printf "%-18s %-10s %4s c%d\n", $_->{name}, $_->{role}, $_->{year}, $_->{century}
    for sort { $a->{last} cmp $b->{last} } @people;

# grep with a block that does real work.
my @modern = grep {
    my $p = $_;
    $p->{year} >= 1900 && $p->{role} ne 'admiral';
} @people;
print "modern non-admirals: ", join(', ', map { $_->{last} } @modern), "\n";

# grep in scalar context counts matches.
my $professors = grep { $_->{role} eq 'professor' } @people;
print "professors: $professors\n";

# Sort by two keys, descending then ascending.
for my $p (sort { $b->{year} <=> $a->{year} || $a->{last} cmp $b->{last} } @people) {
    printf "%4d %s\n", $p->{year}, $p->{last};
}

# Schwartzian transform: decorate, sort, undecorate.
my @by_namelen =
    map  { $_->[1] }
    sort { $a->[0] <=> $b->[0] || $a->[1] cmp $b->[1] }
    map  { [ length($_->{name}), $_->{name} ] }
    @people;
print "by length: ", join(' | ', @by_namelen), "\n";

# A named comparator used as a sort SUBNAME.
sub by_role_then_year {
    return $a->{role} cmp $b->{role} || $a->{year} <=> $b->{year};
}
print join(',', map { "$_->{role}:$_->{year}" } sort by_role_then_year @people), "\n";

# Nested map over a hash of arrays.
my %roles;
push @{ $roles{ $_->{role} } }, $_->{last} for @people;
print join('; ', map { "$_ => " . join(',', sort @{ $roles{$_} }) } sort keys %roles), "\n";

# map/grep composed into one pipeline with a numeric reduction.
my $total_years = 0;
$total_years += $_ for map { $_->{year} } grep { $_->{century} == 20 } @people;
print "sum of 20th century years: $total_years\n";

# sort on a computed key with a reverse.
my @rev = reverse sort { lc($a) cmp lc($b) } map { uc $_->{last} } @people;
print "reverse alpha: @rev\n";
