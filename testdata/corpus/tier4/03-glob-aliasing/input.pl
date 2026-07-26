#!/usr/bin/perl
# TRAP: typeglob manipulation -- aliasing entire symbol-table slots at
# runtime, so two names refer to the SAME variable or sub.
use strict;
use warnings;

our @queue = (1, 2, 3);
our $count = 40;
our (@jobs, $total);

*jobs  = \@queue;    # @jobs IS @queue now (same array, two names)
*total = \$count;    # $total IS $count

push @jobs, 4;
$total += 2;
print "queue=@queue total=$count\n";    # both changed through the aliases

sub real { print "real(@_)\n" }
*fake = \&real;                          # alias a sub
fake("x");

*greet = sub { print "anon installed as greet\n" };   # install sub at runtime
greet();
