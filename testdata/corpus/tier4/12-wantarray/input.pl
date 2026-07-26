#!/usr/bin/perl
# TRAP: wantarray -- one sub returns DIFFERENT SHAPES depending on the
# caller's context, and context flows invisibly through wrappers.
use strict;
use warnings;

sub pieces {
    my @p = split /,/, $_[0];
    return wantarray ? @p : scalar(@p);    # a list OR a count
}
my @list  = pieces("a,b,c");
my $count = pieces("a,b,c");
print "list=@list count=$count\n";

sub ctx {
    return wantarray ? "LIST" : defined(wantarray) ? "SCALAR" : "VOID";
}
my @a = ctx();
my $s = ctx();
ctx();    # void context: wantarray is undef here
print "ctx: @a $s (void call ran too)\n";

# The killer: context FLOWS THROUGH a plain-looking wrapper.
sub wrapper { return pieces(@_) }    # inherits its caller's context
my $n = wrapper("x,y,z,w");
my @w = wrapper("x,y,z,w");
print "wrapped: n=$n w=@w\n";
