#!/usr/bin/perl
# TRAP: runtime monkey-patching of ANOTHER package's symbol table.
# Existing objects pick up the replaced method instantly.
use strict;
use warnings;

package Greeter;
sub new   { return bless {}, shift }
sub hello { return "hello" }

package main;

my $g = Greeter->new;
print "before: ", $g->hello, "\n";

# Replace Greeter::hello at runtime, wrapping the original.
{
    no warnings 'redefine';
    my $orig = \&Greeter::hello;
    *Greeter::hello = sub { return uc( $orig->(@_) ) . "!" };
}

# The same object, the same call site, different behaviour:
print "after:  ", $g->hello, "\n";
