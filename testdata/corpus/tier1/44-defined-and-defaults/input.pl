#!/usr/bin/perl
use strict;
use warnings;

# The two questions that look alike and are not: is this true, and does this
# have a value at all.

print "--- // keeps a false value ---\n";
my $zero = 0;
my $empty = '';
my $word = 'set';
printf "0     // 'dflt' = '%s'\n", ( $zero // 'dflt' );
printf "0     || 'dflt' = '%s'\n", ( $zero || 'dflt' );
printf "''    // 'dflt' = '%s'\n", ( $empty // 'dflt' );
printf "''    || 'dflt' = '%s'\n", ( $empty || 'dflt' );
printf "'set' // 'dflt' = '%s'\n", ( $word // 'dflt' );

print "--- a variable that always has one ---\n";
my $count = 3 * 7;
my $label = "count is $count";
printf "count defined: %s, label defined: %s\n",
    ( defined $count ? 'yes' : 'no' ), ( defined $label ? 'yes' : 'no' );

print "--- a hash key that is missing, present, or zero ---\n";
my %conf = ( host => 'localhost', port => 0, name => '' );
for my $key (qw(host port name timeout)) {
    printf "%-8s exists=%d // gives '%s' || gives '%s'\n", $key,
        ( exists $conf{$key} ? 1 : 0 ),
        ( $conf{$key} // 'DEFAULT' ),
        ( $conf{$key} || 'DEFAULT' );
}
