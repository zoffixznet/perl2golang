#!/usr/bin/perl
# A list slice read for one value, inside the operators that hand a value
# on: defined-or, logical or, the ternary, and plain concatenation. Each
# read wants the element, never the one-element list the slice builds.
use strict;
use warnings;

my %seen = (alpha => "2024-01-05", beta => "2024-03-02", gamma => "2024-02-11");
my %empty;

# The newest date, or a default when there is nothing to pick from.
my $latest = (sort map { $seen{$_} } keys %seen)[-1] // 'none';
my $none   = (sort map { $empty{$_} } keys %empty)[-1] // 'none';
print "latest: $latest\n";
print "none:   $none\n";

# The same read under || and under the ternary.
my @parts = split /,/, "low,,high";
my $mid  = (split /,/, "low,,high")[1] || 'filled';
my $tail = @parts > 2 ? (sort @parts)[-1] : 'short';
print "mid:    $mid\n";
print "tail:   $tail\n";

# A short list read past its end: Perl answers undef, never an error.
my ($host, $port, $proto) = (split /:/, "db.example.net:5432")[0, 1, 2];
$proto = defined $proto ? $proto : 'tcp';
print "host:   $host\n";
print "port:   $port\n";
print "proto:  $proto\n";

# The element spliced straight into text.
print "first sorted part: " . (sort @parts)[0] . "\n";
