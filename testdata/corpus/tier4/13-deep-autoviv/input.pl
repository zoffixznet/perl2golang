#!/usr/bin/perl
# TRAP: autovivification -- merely READING a deep path creates every
# intermediate level as a side effect. Even passing one to a sub does.
use strict;
use warnings;

my %h;
if ( $h{a}{b}{c} ) { print "never\n" }    # this is only a READ...
print "after read: ", ( exists $h{a}      ? "h{a} EXISTS"    : "h{a} absent" ), "\n";
print "inner:      ", ( exists $h{a}{b}   ? "h{a}{b} EXISTS" : "absent" ),      "\n";
print "leaf:       ", ( exists $h{a}{b}{c} ? "exists"        : "leaf absent" ), "\n";

my %cfg;
$cfg{db}{primary}{port} = 5432;           # one write builds three levels
print "port=$cfg{db}{primary}{port}\n";

sub peek { return $_[0] }
my %seen;
peek( $seen{x}{y} );                      # sub args are lvalues: vivifies!
print "seen{x}: ", ( exists $seen{x} ? "EXISTS (viv'd by a sub call)" : "absent" ), "\n";
