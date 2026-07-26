#!/usr/bin/perl
# TRAP: local() on punctuation variables ($", $,, $\) changes what
# builtins do inside OTHER subs, for the dynamic extent of the block.
use strict;
use warnings;

sub render { return "@_" }     # list interpolation uses $" internally
sub show   { print @_ }        # print uses $, between args and $\ after

print "plain: ", render( 1, 2, 3 ), "\n";
{
    local $" = "|";
    print "local: ", render( 1, 2, 3 ), "\n";    # render() output changes
}
print "reset: ", render( 1, 2, 3 ), "\n";

show( "a", "b", "c" );
print "\n";
{
    local $, = "-";
    local $\ = "<END>\n";
    show( "a", "b", "c" );                       # show() output changes
}
show( "a", "b", "c" );
print "\n";
