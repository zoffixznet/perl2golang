#!/usr/bin/perl
# TRAP: a BEGIN block decides AT COMPILE TIME (based on the environment!)
# whether tag() gets a ($) prototype -- which changes how the call on the
# last line PARSES. The same source text has two different parse trees.
use strict;
use warnings;

BEGIN {
    if ( $ENV{TAG_GREEDY} ) {
        eval 'sub tag { return "[" . join(",", @_) . "]" }';
    }
    else {
        eval 'sub tag ($) { return "[" . $_[0] . "]" }';
    }
    die $@ if $@;
}

# With the ($) prototype this is:    (tag("a"), "b")   -> "[a]|b"
# Without the prototype it is:       (tag("a", "b"))   -> "[a,b]"
my @out = ( tag "a", "b" );
print join( "|", @out ), "\n";
