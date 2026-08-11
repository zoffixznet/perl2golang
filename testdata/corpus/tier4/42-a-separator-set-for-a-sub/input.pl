#!/usr/bin/perl
# TRAP: no `local` anywhere. A plain assignment to $, changes what print does
# inside a sub that was written before the assignment and never mentions it.
# The sub is compiled once and its behaviour still changes twice.
use strict;
use warnings;

sub row {
    my @cells = @_;
    print @cells;
    print "\n";
    return scalar @cells;
}

print "default:\n";
row( 'id', 'name', 'size' );

$, = "\t";
print "tab separated:\n";
row( 'id', 'name', 'size' );

$, = ',';
print "comma separated:\n";
row( 'id', 'name', 'size' );

$, = '';
print "back to nothing:\n";
row( 'id', 'name', 'size' );
