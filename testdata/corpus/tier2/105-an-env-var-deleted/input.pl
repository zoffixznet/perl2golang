#!/usr/bin/perl
use strict;
use warnings;

# %ENV is an editable view of the process environment: an assignment sets a
# real variable, and delete removes it outright, answering with the value it
# removed. A removed variable is absent, not empty.
$ENV{P2G_DEMO_FLAVOR} = 'plum';
print "set: $ENV{P2G_DEMO_FLAVOR}\n";

my $removed = delete $ENV{P2G_DEMO_FLAVOR};
print "delete returned: $removed\n";
print "read after delete: ", ($ENV{P2G_DEMO_FLAVOR} // '(gone)'), "\n";

# Deleting a variable that is not there is quiet, exactly like a hash key.
delete $ENV{P2G_DEMO_NEVER_SET};
print "double delete is quiet\n";
