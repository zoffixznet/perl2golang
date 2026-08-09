#!/usr/bin/perl
use strict;
use warnings;

# One slot holding text here and numbers there, in the shapes scripts
# actually use: a config hash with mixed values, an accumulator first seen
# as text, a list of labels and counts, and a field carved out of a
# fixed-width record and then done arithmetic on.

my %conf = ( host => 'localhost', port => 8080, retries => 3, name => 'svc' );
print "endpoint: $conf{host}:$conf{port}\n";
print "give up after: ", $conf{retries} * 2, " tries\n";
$conf{port} = $conf{port} + 1;
print "bumped port: $conf{port}\n";

# The accumulator arrives as text, from a line that looks like a file's.
my $carried = "1200";
$carried += 34;
$carried += 6;
print "carried: $carried\n";

# Labels and counts share one list.
my @report = ( 'widgets', 12, 'gadgets', 3 );
print "report: ", join( '|', @report ), "\n";

# A fixed-width record: every field text, one of them then treated as a
# number.
my ( $tag, $qty, $item ) = unpack 'a3 A5 A12', 'ORD  042widget-blue ';
print "tag=$tag item=$item\n";
print "double qty: ", $qty * 2, "\n";
