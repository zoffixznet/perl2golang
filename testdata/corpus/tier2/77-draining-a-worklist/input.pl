#!/usr/bin/perl
use strict;
use warnings;

# Emptying a list one element at a time. The test is whether there was an
# element, not whether the element was true, and the difference shows up the
# moment a 0 or an empty string is in the list.

print "--- a queue with a zero in it ---\n";
my @queue = ( 3, 0, 7 );
while ( defined( my $job = shift @queue ) ) {
    printf "took %d, %d left\n", $job, scalar @queue;
}

print "--- the same list, tested for truth instead ---\n";
my @again = ( 3, 0, 7 );
while ( my $job = shift @again ) {
    printf "took %d, %d left\n", $job, scalar @again;
}
printf "stopped with %d still queued\n", scalar @again;

print "--- a stack of names, one of them empty ---\n";
my @stack = ( 'alpha', '', 'omega' );
while ( defined( my $name = pop @stack ) ) {
    printf "popped [%s]\n", $name;
}

print "--- a worklist that grows while it drains ---\n";
my @work = ( 'a' );
my %seen;
my @order;
my %children = ( a => [ 'b', 'c' ], b => ['d'], c => [], d => [] );
while ( defined( my $node = shift @work ) ) {
    next if $seen{$node}++;
    push @order, $node;
    push @work, @{ $children{$node} };
}
printf "visited: %s\n", join ',', @order;
