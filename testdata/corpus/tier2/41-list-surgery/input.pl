#!/usr/bin/perl
# A small work queue, written the way a real script writes one: splice for
# every list edit, hash slices for bulk assignment, and each for the walk.
use strict;
use warnings;

my @queue = map { "job$_" } 1 .. 9;

# ---- take a batch off the front ----------------------------------------
my @batch = splice @queue, 0, 3;
print "batch: @batch\n";
print "left:  @queue\n";

# ---- put a priority job at the front -----------------------------------
splice @queue, 0, 0, 'urgent';
print "after unshift-by-splice: @queue\n";

# ---- replace a run with a different number of jobs ---------------------
splice @queue, 2, 2, 'merged-a', 'merged-b', 'merged-c';
print "after replace:           @queue\n";

# ---- trim to a maximum length, the common one-liner --------------------
my $cap = 5;
splice @queue, $cap if @queue > $cap;
print "capped at $cap:          @queue\n";

# ---- negative offset and negative length -------------------------------
my @tail = splice @queue, -2;
print "tail: @tail / left @queue\n";
my @middle = splice @queue, 1, -1;
print "middle: @middle / left @queue\n";

# ---- splice through a reference held in a hash -------------------------
my %lanes = ( fast => [ 1 .. 5 ], slow => [ 6 .. 8 ] );
my @moved = splice @{ $lanes{fast} }, 1, 2;
push @{ $lanes{slow} }, @moved;
print "fast: @{$lanes{fast}}\n";
print "slow: @{$lanes{slow}}\n";

# ---- a hash slice assigned in one statement ----------------------------
my %conf;
@conf{qw(host port user)} = ( 'localhost', 8080, 'admin' );
@conf{ 'retries', 'timeout' } = ( 3, 30 );
print join( ' ', map { "$_=$conf{$_}" } sort keys %conf ), "\n";

# ---- each, collected and then sorted so the order is ours --------------
my @pairs;
while ( my ( $k, $v ) = each %conf ) {
    push @pairs, "$k/$v";
}
print "pairs: ", join( ' ', sort @pairs ), "\n";

# ---- substr on the left of an assignment -------------------------------
my $line = 'ERROR 2024-01-01 disk full';
substr( $line, 0, 5 ) = 'FATAL';
print "$line\n";
substr( $line, -4 ) = 'gone';
print "$line\n";

# ---- a conditional as an assignment target -----------------------------
my ( $wins, $losses ) = ( 0, 0 );
for my $score ( 3, -1, 7, -4, 0 ) {
    ( $score >= 0 ? $wins : $losses ) += 1;
}
print "wins=$wins losses=$losses\n";
