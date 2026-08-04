#!/usr/bin/perl
use strict;
use warnings;

# The capture variables are global and outlive the match that filled them,
# which is what makes reading them after the if block work, and what makes a
# failed match leave the previous answer standing.

my @lines = ( 'user=ada id=7', 'nothing here', 'user=grace id=9' );

print "--- read after the block that matched ---\n";
if ( $lines[0] =~ /user=(\w+)/ ) {
    print "inside:  $1\n";
}
print "outside: $1\n";
print "whole:   $&\n";

print "--- a failed match leaves the last answer standing ---\n";
if ( $lines[1] =~ /user=(\w+)/ ) {
    print "never\n";
}
print "still:   $1\n";

print "--- and the next success replaces it ---\n";
for my $line (@lines) {
    next unless $line =~ /id=(\d+)/;
}
print "last id: $1\n";

print "--- prematch and postmatch ---\n";
if ( $lines[2] =~ /id=/ ) {
    printf "pre=[%s] match=[%s] post=[%s]\n", $`, $&, $';
}
