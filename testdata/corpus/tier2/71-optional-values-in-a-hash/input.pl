#!/usr/bin/perl
use strict;
use warnings;

# A settings table where "not set" and "set to zero" are different answers,
# which is the case a Go map of plain numbers cannot hold.

my %limit = (
    retries => 0,
    timeout => 30,
    burst   => undef,
);

print "--- three states per key ---\n";
for my $key (qw(retries timeout burst window)) {
    printf "%-8s exists=%s defined=%s value=%s\n", $key,
        ( exists  $limit{$key} ? 'yes' : 'no' ),
        ( defined $limit{$key} ? 'yes' : 'no' ),
        ( defined $limit{$key} ? $limit{$key} : '-' );
}

print "--- defaults only fill what is missing ---\n";
$limit{retries} //= 3;
$limit{burst}   //= 5;
$limit{window}  //= 60;
printf "retries=%d burst=%d window=%d\n",
    $limit{retries}, $limit{burst}, $limit{window};

print "--- counting through a slot that was never set ---\n";
my %seen = ( alpha => 2 );
$seen{beta} = undef;
$seen{$_}++ for qw(alpha beta gamma);
for my $name (sort keys %seen) {
    printf "%-6s %d\n", $name, $seen{$name};
}

print "--- undef read as a number and as text ---\n";
my %row = ( label => 'core', count => undef );
my $count = $row{count};
{
    no warnings 'uninitialized';
    printf "count as number: %d\n", $count + 0;
    printf "count as text:   [%s]\n", $count;
}
printf "label is %s\n", $row{label};

print "--- delete hands back what was there ---\n";
my $gone = delete $limit{timeout};
printf "removed timeout=%s, left with %d keys\n", $gone, scalar keys %limit;
