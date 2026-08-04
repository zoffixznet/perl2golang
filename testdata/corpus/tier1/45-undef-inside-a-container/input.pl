#!/usr/bin/perl
use strict;
use warnings;

# undef stored in a container, and the chained default, neither of which
# survives the conversion.

print "--- undef as a value in a hash ---\n";
my %age = ( alice => 34, bob => 41 );
$age{frank} = undef;
for my $who (qw(alice frank zed)) {
    printf "%-6s exists=%d defined=%d true=%d\n", $who,
        ( exists $age{$who} ? 1 : 0 ),
        ( defined $age{$who} ? 1 : 0 ),
        ( $age{$who} ? 1 : 0 );
}

print "--- undef in an array ---\n";
my @slots = ( 1, undef, 3 );
printf "length %d, slot 1 defined: %s\n", scalar @slots,
    ( defined $slots[1] ? 'yes' : 'no' );
my $filled = grep { defined } @slots;
print "filled slots: $filled\n";

print "--- a chain of defaults ---\n";
my %opt = ( retries => 0 );
my $retries = $opt{retries} // $opt{tries} // 5;
my $tries   = $opt{tries} // $opt{retries} // 5;
print "retries=$retries tries=$tries\n";

print "--- undef through a variable ---\n";
my $maybe = $age{frank};
printf "copied out: %s\n", ( defined $maybe ? 'defined' : 'undef' );
$maybe = 0;
printf "after = 0:  %s\n", ( defined $maybe ? 'defined' : 'undef' );
