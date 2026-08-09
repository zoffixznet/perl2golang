#!/usr/bin/perl
use strict;
use warnings;

# A slot that genuinely holds different shapes of thing: a count here, a
# hashref there. Text has no honest form for that, so this is the mix that
# must stay dynamic, and the file around it should still convert and run.

my %stats = (
    total   => 42,
    breakdown => { pass => 30, fail => 12 },
    label   => 'nightly',
);

print "label: $stats{label}\n";
print "total: $stats{total}\n";

my $b = $stats{breakdown};
print "pass:  $b->{pass}\n";
print "fail:  $b->{fail}\n";

# The same mix in a list: a value and the structure it came from.
my @trail = ( 'first', { step => 1 }, 'second', { step => 2 } );
for my $t (@trail) {
    if ( ref $t eq 'HASH' ) {
        print "step $t->{step}\n";
    }
    else {
        print "mark $t\n";
    }
}
