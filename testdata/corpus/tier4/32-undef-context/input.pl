#!/usr/bin/perl
# TRAP: undef is 0 in numeric context, "" in string context, false in
# boolean context -- and "00", "0.0", "0E0" are TRUE while "0" is false.
# Go has no undef and no truthiness at all.
use strict;
use warnings;
no warnings 'uninitialized';

my %opts  = ( verbose => 1 );
my $level = $opts{level};                 # missing key: undef, not a panic
print "level+1: ", $level + 1, "\n";      # 1
print "level str: <$level>\n";            # <>
print "defined: ", ( defined $level ? "yes" : "no" ), "\n";
print "bool: ",    ( $level         ? "true" : "false" ), "\n";

for my $v ( undef, 0, "0", "", "00", "0.0", "0E0" ) {
    my $show = defined $v ? "'$v'" : "undef";
    printf "%-6s -> %s\n", $show, ( $v ? "true" : "false" );
}
