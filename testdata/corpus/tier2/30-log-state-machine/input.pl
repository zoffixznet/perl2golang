#!/usr/bin/perl
use strict;
use warnings;

# A line-oriented state machine over a build log. It tracks whether it is
# inside a stage, accumulates per-stage detail, and reports transitions --
# the shape you end up with whenever a log has begin/end markers.

my $state = 'OUTSIDE';
my $current;
my @stages;
my @transitions;
my %counts;
my $lineno = 0;

open my $fh, '<', 'files/build.log' or die "open: $!\n";
while (my $line = <$fh>) {
    $lineno++;
    chomp $line;
    next unless $line =~ /\S/;

    if ($state eq 'OUTSIDE') {
        if ($line =~ /^BEGIN STAGE (\S+)$/) {
            $current = { name => $1, start => $lineno, lines => [], warnings => 0, failures => [] };
            push @transitions, "$lineno OUTSIDE->INSIDE($1)";
            $state = 'INSIDE';
        }
        else {
            $counts{preamble}++;
        }
        next;
    }

    if ($state eq 'INSIDE') {
        if ($line =~ /^END STAGE (\S+) (\S+)$/) {
            my ($name, $result) = ($1, $2);
            die "stage mismatch at line $lineno: $name vs $current->{name}\n"
                if $name ne $current->{name};
            $current->{end}    = $lineno;
            $current->{result} = $result;
            push @stages, $current;
            push @transitions, "$lineno INSIDE($name)->OUTSIDE [$result]";
            $counts{$result}++;
            undef $current;
            $state = 'OUTSIDE';
        }
        elsif ($line =~ /^WARNING (.+)$/) {
            $current->{warnings}++;
            push @{ $current->{lines} }, "warn: $1";
        }
        elsif ($line =~ /^FAILURE test (\S+)$/) {
            push @{ $current->{failures} }, $1;
            push @{ $current->{lines} }, "fail: $1";
        }
        elsif ($line =~ /^\[(\d\d):(\d\d)\]\s+(.*)$/) {
            push @{ $current->{lines} }, sprintf('%3ds %s', $1 * 60 + $2, $3);
        }
        else {
            push @{ $current->{lines} }, "other: $line";
        }
        next;
    }
}
close $fh or die "close: $!\n";

die "log ended while still inside stage $current->{name}\n" if $state ne 'OUTSIDE';

printf "%d line(s), %d stage(s), final state %s\n", $lineno, scalar @stages, $state;
print "-- transitions --\n";
print "$_\n" for @transitions;

print "-- stages --\n";
printf "%-10s %5s %5s %-8s %5s %5s\n", 'STAGE', 'FROM', 'TO', 'RESULT', 'WARN', 'FAIL';
for my $s (@stages) {
    printf "%-10s %5d %5d %-8s %5d %5d\n",
        $s->{name}, $s->{start}, $s->{end}, $s->{result},
        $s->{warnings}, scalar @{ $s->{failures} };
}

print "-- detail --\n";
for my $s (@stages) {
    next unless $s->{warnings} || @{ $s->{failures} };
    print "$s->{name}:\n";
    print "  $_\n" for grep { /^(warn|fail):/ } @{ $s->{lines} };
}

print "-- summary --\n";
printf "%-9s %d\n", $_, $counts{$_} for sort keys %counts;
my @failed = map { $_->{name} } grep { $_->{result} eq 'failed' } @stages;
printf "failed stages: %s\n", (@failed ? join(',', @failed) : 'none');
my @all_failures = map { @{ $_->{failures} } } @stages;
printf "failing tests: %s\n", join(' ', sort @all_failures);
