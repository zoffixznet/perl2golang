#!/usr/bin/perl
use strict;
use warnings;

# exists on %ENV asks whether the variable is set at all, which is a
# different question from whether its value is true. A variable holding
# "0" or the empty string exists; only an unset one does not.
$ENV{P2G_DEMO_LEVEL} = '0';
print "holds 0, exists: ",  (exists $ENV{P2G_DEMO_LEVEL} ? 'yes' : 'no'), "\n";

$ENV{P2G_DEMO_EMPTY} = '';
print "holds '', exists: ", (exists $ENV{P2G_DEMO_EMPTY} ? 'yes' : 'no'), "\n";

print "never set, exists: ", (exists $ENV{P2G_DEMO_UNSET} ? 'yes' : 'no'), "\n";

delete $ENV{P2G_DEMO_LEVEL};
print "deleted, exists: ",  (exists $ENV{P2G_DEMO_LEVEL} ? 'yes' : 'no'), "\n";
