#!/usr/bin/perl
use strict;
use warnings;

# Reads a hosts file line by line with a lexical filehandle, skipping
# comments and blanks, and reports on what it found.

my $path = 'files/hosts.txt';

open my $fh, '<', $path or die "$0: cannot open $path: $!\n";

my $lineno    = 0;
my $comments  = 0;
my $blanks    = 0;
my @entries;

while (my $line = <$fh>) {
    $lineno++;
    chomp $line;

    if ($line =~ /^\s*#/) { $comments++; next }
    if ($line =~ /^\s*$/) { $blanks++;   next }

    my ($ip, @names) = split ' ', $line;
    push @entries, { line => $lineno, ip => $ip, names => \@names };
}

close $fh or die "cannot close $path: $!\n";

printf "read %d lines: %d entries, %d comments, %d blank\n",
    $lineno, scalar @entries, $comments, $blanks;

for my $e (@entries) {
    printf "%3d  %-12s %s\n", $e->{line}, $e->{ip}, join(' ', @{ $e->{names} });
}

# Second pass: read into an array all at once, then index by name.
open my $fh2, '<', $path or die "reopen failed: $!\n";
my @all = <$fh2>;
close $fh2 or die "close: $!\n";
chomp @all;
printf "slurped %d lines, longest is %d chars\n",
    scalar @all, (sort { $b <=> $a } map { length } @all)[0];

my %by_name;
for my $e (@entries) {
    $by_name{$_} = $e->{ip} for @{ $e->{names} };
}
for my $name (sort keys %by_name) {
    printf "%-18s -> %s\n", $name, $by_name{$name};
}

# $. holds the current input line number for the active handle.
open my $fh3, '<', $path or die "reopen failed: $!\n";
my $last_data_line = 0;
while (<$fh3>) {
    $last_data_line = $. if /^\d/;
}
print "last IPv4 entry was on line $last_data_line\n";
close $fh3 or die "close: $!\n";

# chomp returns the number of characters removed.
my $sample = "trailing newline\n";
my $removed = chomp $sample;
print "chomp removed $removed char(s), left ", length($sample), "\n";
