#!/usr/bin/perl
# A log-line splitter whose assignments do several jobs at once: targets
# that declare mid-list, targets that are fields of a record, a slice of
# the named captures, and arrayrefs walked through $_->[i].
use strict;
use warnings;

my %event = ( kind => '?', detail => '?' );

my $line = 'auth failure from 10.0.0.7 port 2222';
( my $what, $event{kind}, my $rest ) = split ' ', $line, 3;
print "what=$what kind=$event{kind} rest=$rest\n";

( $rest, my $port ) = split / port /, $rest, 2;
print "addr-part=$rest port=$port\n";

if ( $line =~ /^(?<service>\w+) (?<outcome>\w+) from (?<addr>[\d.]+)/ ) {
    my ( $addr, $service, $outcome ) = @+{qw(addr service outcome)};
    print "svc=$service outcome=$outcome addr=$addr\n";
}

# Pairs held as arrayrefs, formatted through the topic.
my @pairs = ( [ 'cpu', 91 ], [ 'mem', 74 ], [ 'disk', 55 ] );
my @high = map { "$_->[0]=$_->[1]%" } grep { $_->[1] > 60 } @pairs;
print "high: @high\n";

# The same shapes through a sub, so $_[0] keeps meaning the first argument.
sub label { return "[$_[0]]" }
print "labelled: ", join( ' ', map { label($_->[0]) } @pairs ), "\n";
