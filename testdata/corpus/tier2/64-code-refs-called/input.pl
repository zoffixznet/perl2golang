#!/usr/bin/perl
use strict;
use warnings;

# Calling through a code reference, in every shape a script does it.

print "-- a closure factory --\n";
sub make_counter {
    my ($start) = @_;
    my $bump = sub { $start += $_[0]; return $start };
    my $peek = sub { return $start };
    return ( $bump, $peek );
}
my ( $bump, $peek ) = make_counter(100);
print "bump 5  -> ", $bump->(5), "\n";
print "bump 20 -> ", $bump->(20), "\n";
print "peek    -> ", $peek->(), "\n";

print "-- an array flattened into the call --\n";
my $join_all = sub { return join( '|', @_ ) };
my @parts = qw(alpha beta gamma);
print "list only:   ", $join_all->(@parts), "\n";
print "one in front: ", $join_all->( 'first', @parts ), "\n";
print "two lists:    ", $join_all->( @parts, @parts ), "\n";
print "no arguments: [", $join_all->(), "]\n";

print "-- a table with a fallback --\n";
my %ops = (
    add => sub { $_[0] + $_[1] },
    mul => sub { $_[0] * $_[1] },
);
sub op_for { my ($name) = @_; return $ops{$name} || sub { 'n/a' } }
for my $name (qw(add mul nope)) {
    print "$name(6,7) = ", op_for($name)->( 6, 7 ), "\n";
}
