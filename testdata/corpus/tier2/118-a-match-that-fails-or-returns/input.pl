#!/usr/bin/perl
# `my (...) = EXPR or return ...` leaves the sub early when the match found
# nothing. The return is a statement in expression position, and it has to
# survive the trip: losing it would let the sub fall through to code that
# only makes sense after a successful match.
use strict;
use warnings;

sub day_number {
    my ($date) = @_;
    my ($y, $m, $d) = $date =~ /^(\d{4})-(\d{2})-(\d{2})$/ or return 0;
    my @cum = (0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334);
    return $y * 365 + $cum[$m - 1] + $d;
}

sub parse_pair {
    my ($text) = @_;
    my ($key, $value) = $text =~ /^(\w+)=(\w+)$/ or return;
    return "$key -> $value";
}

for my $date ('2026-08-15', 'not-a-date', '1999-12-31') {
    printf "%-12s %d\n", $date, day_number($date);
}

for my $pair ('mode=fast', 'broken line', 'depth=3') {
    my $parsed = parse_pair($pair);
    print defined $parsed ? "$parsed\n" : "skipped: $pair\n";
}
