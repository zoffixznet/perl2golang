#!/usr/bin/perl
# TRAP: DESTROY fires at a DETERMINISTIC instant -- the moment the
# refcount hits zero. Guard-object code depends on that exact timing.
# Go's GC provides no such guarantee.
use strict;
use warnings;

package Guard;
sub new     { my ( $c, $n ) = @_; print "acquire $n\n"; return bless { n => $n }, $c }
sub DESTROY { my $s = shift; print "release $s->{n}\n" }

package main;

print "start\n";
{
    my $g = Guard->new("lock-A");
    print "working under lock-A\n";
}    # DESTROY fires HERE, precisely at the closing brace
print "between\n";

my $g2 = Guard->new("lock-B");
undef $g2;    # DESTROY fires HERE, precisely at the undef
print "end\n";
