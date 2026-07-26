#!/usr/bin/perl
use strict;
use warnings;

# Word frequency counter reading STDIN: normalise, count, drop stopwords,
# sort by count then alphabetically, and print a histogram. Ties are
# broken deterministically so the output never depends on hash order.

my %stop = map { $_ => 1 } qw(the a an and or of to in is it that for on with as);

my %freq;
my %first_line;
my $words = 0;
my $lines = 0;

while (my $line = <STDIN>) {
    $lines++;
    chomp $line;
    $line = lc $line;
    for my $w ($line =~ /([a-z][a-z'\-]*)/g) {
        $w =~ s/^-+|-+$//g;
        next unless length $w;
        $words++;
        $freq{$w}++;
        $first_line{$w} = $lines unless exists $first_line{$w};
    }
}

my @ranked = sort { $freq{$b} <=> $freq{$a} || $a cmp $b } keys %freq;
my @content = grep { !$stop{$_} } @ranked;

printf "%d lines, %d words, %d distinct, %d after stopwords\n",
    $lines, $words, scalar @ranked, scalar @content;

my $max = @content ? $freq{ $content[0] } : 0;
print "-- top 10 content words --\n";
my $rank = 0;
for my $w (@content[0 .. ($#content < 9 ? $#content : 9)]) {
    $rank++;
    printf "%2d. %-12s %3d %s\n", $rank, $w, $freq{$w}, '#' x $freq{$w};
}

print "-- singletons --\n";
my @once = grep { $freq{$_} == 1 } @content;
printf "%d word(s) seen once: %s\n", scalar @once, join(' ', @once[0 .. ($#once < 7 ? $#once : 7)]);

print "-- stopwords found --\n";
my @stops_present = grep { $stop{$_} } @ranked;
printf "%-6s %3d (first seen line %d)\n", $_, $freq{$_}, $first_line{$_} for @stops_present;

print "-- by length --\n";
my %by_len;
push @{ $by_len{ length $_ } }, $_ for @content;
for my $len (sort { $a <=> $b } keys %by_len) {
    my @ws = sort @{ $by_len{$len} };
    printf "%2d: %d word(s) %s\n", $len, scalar @ws,
        join(',', @ws[0 .. ($#ws < 4 ? $#ws : 4)]);
}

# Deduplicated list of the words in first-appearance order.
my %seen;
my @in_order = grep { !$seen{$_}++ } sort { $first_line{$a} <=> $first_line{$b} || $a cmp $b } keys %freq;
print "-- first eight distinct, in order of appearance --\n";
print join(' ', @in_order[0 .. 7]), "\n";
print "cumulative coverage of top 5: ";
my $covered = 0;
$covered += $freq{$_} for @ranked[0 .. 4];
printf "%.1f%%\n", 100 * $covered / $words;
