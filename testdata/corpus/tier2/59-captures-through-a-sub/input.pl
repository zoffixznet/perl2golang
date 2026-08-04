#!/usr/bin/perl
use strict;
use warnings;

# The neighbouring case: a match whose captures leave the sub that made them.
# Here the parentheses that say "list" are at the call site, not at the match,
# so the sub itself has to decide what shape it returns.

my @records = (
    'host=web1 port=8080',
    'host=db1 port=5432',
    'malformed line',
);

sub split_pair {
    my ($text) = @_;
    return $text =~ /host=(\w+)\s+port=(\d+)/;
}

sub first_word {
    my ($text) = @_;
    return $text =~ /^(\w+)/;
}

print "--- captures returned from a sub ---\n";
for my $rec (@records) {
    my ($host, $port) = split_pair($rec);
    if (defined $host) {
        printf "%-5s %s\n", $host, $port;
    } else {
        print "unparsed: $rec\n";
    }
}

print "--- the same sub asked for a count ---\n";
for my $rec (@records) {
    my @got = split_pair($rec);
    print scalar(@got), " value(s)\n";
}

print "--- one capture, read as a truth value ---\n";
for my $rec (@records) {
    my ($word) = first_word($rec);
    print "word=$word\n";
    print "  (that sub is also used as a test)\n" if first_word($rec);
}
