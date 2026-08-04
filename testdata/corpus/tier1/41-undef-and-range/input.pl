#!/usr/bin/perl
# undef where a typed container cannot keep it, and reading past the end.
# These are the places where Perl's one scalar type and Go's many part
# company for good, and none of them converts yet.
use strict;
use warnings;

print "--- undef stored in a hash ---\n";
my %age = ( alice => 30, bob => 25 );
$age{frank} = undef;
for my $k (qw(alice frank zed)) {
    printf "%-6s exists=%d defined=%d true=%d\n",
        $k, ( exists $age{$k} ? 1 : 0 ), ( defined $age{$k} ? 1 : 0 ),
        ( $age{$k} ? 1 : 0 );
}

print "--- undef as a value that is not zero ---\n";
my $unset;
printf "unset is defined: %s\n", ( defined $unset ? 'yes' : 'no' );
my $zero = 0;
printf "zero is defined:  %s\n", ( defined $zero ? 'yes' : 'no' );
printf "defined-or on undef: %s\n", ( $unset // 'fallback' );
printf "defined-or on zero:  %s\n", ( $zero  // 'fallback' );
printf "or on zero:          %s\n", ( $zero  || 'fallback' );

print "--- draining a queue that contains a zero ---\n";
my @queue = ( 3, 0, 7 );
while ( defined( my $item = shift @queue ) ) {
    print "item $item\n";
}

print "--- the same drain without the defined test ---\n";
my @q2 = ( 3, 0, 7 );
while ( my $item = shift @q2 ) {
    print "stopped-early item $item\n";
}
printf "left behind: %d\n", scalar @q2;

print "--- reading past the end ---\n";
my @three = ( 'a', 'b', 'c' );
my $past = $three[9];
printf "past the end is defined: %s\n", ( defined $past ? 'yes' : 'no' );
printf "the array is still %d long\n", scalar @three;
$three[5] = 'f';
printf "after writing index 5 it is %d long\n", scalar @three;
printf "the gap at 4 is defined: %s\n", ( defined $three[4] ? 'yes' : 'no' );
